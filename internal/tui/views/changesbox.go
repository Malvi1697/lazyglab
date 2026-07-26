package views

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// changesBox is a set of changed files and the reader for one of them: the
// "Changes" list, plus the full-screen unified diff behind Enter.
//
// A commit and a merge request are different things with the same body of work
// attached, so this is embedded in both pages rather than written twice. The
// fields are promoted, so a page says d.diffs and d.reading as if they were its
// own — which, as far as the page is concerned, they are.
type changesBox struct {
	diffs      []gitlab.FileDiff
	fileCursor int
	fileScroll int
	reading    bool
	diffScroll int
	diffCache  diffRender

	// diffScrollable is learned while rendering, so the footer offers j/k only
	// where they would move something.
	diffScrollable bool
}

// diffRender is a file's diff as display lines, kept so scrolling does not
// re-tokenise it on every frame.
type diffRender struct {
	path  string
	width int
	lines []string
}

// setDiffs takes a fetched set of changed files.
func (b *changesBox) setDiffs(diffs []gitlab.FileDiff) {
	b.diffs = diffs
	if b.fileCursor >= len(b.diffs) {
		b.fileCursor = 0
	}
	b.diffCache = diffRender{}
}

// resetFiles forgets everything: a different commit or merge request is opening,
// and two of them can touch the same path, so the rendered diff has to go too.
func (b *changesBox) resetFiles() {
	b.diffs = nil
	b.fileCursor, b.fileScroll = 0, 0
	b.reading, b.diffScroll = false, 0
	b.diffCache = diffRender{}
}

// selectedFile returns the highlighted file, or nil.
func (b *changesBox) selectedFile() *gitlab.FileDiff {
	if b.fileCursor < 0 || b.fileCursor >= len(b.diffs) {
		return nil
	}
	return &b.diffs[b.fileCursor]
}

// stepFile moves to the neighbouring changed file, keeping its diff open.
func (b *changesBox) stepFile(step int) {
	next := b.fileCursor + step
	if next < 0 || next >= len(b.diffs) {
		return // already at an end
	}
	b.fileCursor = next
	b.diffScroll = 0
}

// filesKey drives the list of changed files and reports whether it took the key.
// Enter opens the highlighted file; Esc is left to the page, which has somewhere
// to go back to.
func (b *changesBox) filesKey(key string, height int) bool {
	if key == keyEnter {
		if b.selectedFile() != nil {
			b.reading = true
			b.diffScroll = 0
		}
		return true
	}
	if act := components.NavFor(key); act != components.NavNone {
		b.fileCursor = components.ApplyNav(act, b.fileCursor, len(b.diffs), listRows(height))
		return true
	}
	return false
}

// readerKey drives an open diff: the arrows step files, the rest scrolls.
func (b *changesBox) readerKey(key string, height int) bool {
	// The arrows step within what you are looking at. Stepping to another commit
	// from inside a diff would swap the file under you for one from elsewhere.
	if step, ok := stepKey(key); ok {
		b.stepFile(step)
		return true
	}
	if act := components.NavFor(key); act != components.NavNone {
		b.diffScroll = scrollBy(act, b.diffScroll, listRows(height))
		return true
	}
	return false
}

// closeReader goes back from a diff to the list of files.
func (b *changesBox) closeReader() {
	b.reading = false
	b.diffScroll = 0
}

// filesTitle names the list.
func (b *changesBox) filesTitle() string {
	return fmt.Sprintf("Changes (%d)", len(b.diffs))
}

// readerTitle names the open diff: which file, and which of how many, the same
// way a page says which commit.
func (b *changesBox) readerTitle() string {
	f := b.selectedFile()
	if f == nil {
		return "Diff"
	}
	return fmt.Sprintf("%s  %d/%d", f.Path(), b.fileCursor+1, len(b.diffs))
}

// fileRow renders one row of the list.
func (b *changesBox) fileRow(i int) string {
	f := b.diffs[i]
	return fmt.Sprintf("%s %s%s", fileMark(f), f.Path(), diffStat(f))
}

// emptyFilesRow is what the box says when there is nothing to list: still loading,
// or a change with no files reported.
func (b *changesBox) emptyFilesRow(loading bool) []string {
	if loading {
		return []string{components.HelpDescStyle.Render("Loading…")}
	}
	return []string{components.HelpDescStyle.Render("No changes reported")}
}

// filesBox renders the list of changed files as a page column.
func (b *changesBox) filesBox(width, height int, focused, loading bool) string {
	if len(b.diffs) == 0 {
		return components.RenderPanel(b.filesTitle(), b.emptyFilesRow(loading), width, height, focused)
	}
	return renderRowsBox(width, height, b.filesTitle(), len(b.diffs), b.fileRow,
		cursorWhen(focused, b.fileCursor), &b.fileScroll)
}

// diffView renders the selected file's unified diff, scrolled to diffScroll.
func (b *changesBox) diffView(width, height int) string {
	f := b.selectedFile()
	if f == nil {
		return ""
	}
	if f.Withheld {
		return components.MutedStyle.Render("GitLab did not send this diff: the change is too large.")
	}

	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	lines := b.diffLines(f, contentWidth)

	rows := height - 1
	if rows < 1 {
		rows = 1
	}
	b.diffScrollable = len(lines) > rows
	if maxScroll := len(lines) - rows; b.diffScroll > maxScroll {
		b.diffScroll = max(0, maxScroll)
	}
	if b.diffScroll < 0 {
		b.diffScroll = 0
	}
	end := min(b.diffScroll+rows, len(lines))
	return strings.Join(lines[b.diffScroll:end], "\n")
}

// diffLines renders a file's unified diff into display lines, syntax highlighted
// and wrapped to width. The result is cached per file and width: scrolling
// re-renders the whole diff each frame, and a thousand-line file would be
// tokenised again on every keypress.
func (b *changesBox) diffLines(f *gitlab.FileDiff, width int) []string {
	path := f.Path()
	if b.diffCache.path == path && b.diffCache.width == width {
		return b.diffCache.lines
	}

	var lines []string
	for _, raw := range strings.Split(strings.TrimRight(f.Diff, "\n"), "\n") {
		lines = append(lines, styleDiffLine(path, raw, width)...)
	}

	b.diffCache = diffRender{path: path, width: width, lines: lines}
	return lines
}

// readerHints are the footer hints while a diff has the screen.
func (b *changesBox) readerHints(copyDesc string) []KeyHint {
	hints := []KeyHint{{"←/→ h/l", "Prev/next file"}}
	if b.diffScrollable {
		hints = append(hints, KeyHint{"j/k", "Scroll"})
	}
	return append(hints, KeyHint{"y/Y", copyDesc}, KeyHint{"Esc", "Back"})
}

// styleDiffLine renders one line of a unified diff, wrapped to width.
//
// The marker column carries the meaning — added, removed, context — so the code
// beside it is free to be syntax highlighted and read like the file it came from.
// A wrapped line keeps its marker column empty on the continuation rows, which
// keeps the code aligned and does not claim the marker twice.
func styleDiffLine(path, line string, width int) []string {
	switch {
	case strings.HasPrefix(line, "@@"):
		// The hunk header is the only structure in a diff, so it gets the accent.
		return []string{components.TitleStyle.Render(components.Truncate(line, width))}
	case strings.HasPrefix(line, "\\"):
		// "\ No newline at end of file" is a note about the diff, not part of it.
		return []string{components.MutedStyle.Render(components.Truncate(line, width))}
	}

	marker, body := " ", line
	var markerStyle lipgloss.Style
	switch {
	case strings.HasPrefix(line, "+"):
		marker, body = "+", line[1:]
		markerStyle = lipgloss.NewStyle().Foreground(components.ColorSuccess).Bold(true)
	case strings.HasPrefix(line, "-"):
		marker, body = "-", line[1:]
		markerStyle = lipgloss.NewStyle().Foreground(components.ColorError).Bold(true)
	case strings.HasPrefix(line, " "):
		body = line[1:]
	}

	codeWidth := width - 1
	if codeWidth < 1 {
		codeWidth = 1
	}

	wrapped := components.WrapLine(body, codeWidth)
	out := make([]string, 0, len(wrapped))
	for i, frag := range wrapped {
		gutter := " "
		if i == 0 && marker != " " {
			gutter = markerStyle.Render(marker)
		}
		out = append(out, gutter+components.Highlight(path, frag))
	}
	return out
}

// fileMark is the one-letter state of a changed file, coloured like a diff.
func fileMark(f gitlab.FileDiff) string {
	style := func(c color.Color, s string) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
	}
	switch {
	case f.New:
		return style(components.ColorSuccess, "A")
	case f.Deleted:
		return style(components.ColorError, "D")
	case f.Renamed:
		return style(components.ColorRunning, "R")
	default:
		return style(components.ColorWarning, "M")
	}
}

// diffStat renders "+12 -3" for a file, or says the diff was withheld.
func diffStat(f gitlab.FileDiff) string {
	if f.Withheld {
		return components.MutedStyle.Render("  (too large to show)")
	}
	out := ""
	if f.Added > 0 {
		out += lipgloss.NewStyle().Foreground(components.ColorSuccess).Render(fmt.Sprintf("  +%d", f.Added))
	}
	if f.Removed > 0 {
		out += lipgloss.NewStyle().Foreground(components.ColorError).Render(fmt.Sprintf("  -%d", f.Removed))
	}
	return out
}

// scrollBy moves a scroll offset by a navigation action, for content that is read
// rather than selected from.
func scrollBy(act components.NavAction, offset, rows int) int {
	switch act {
	case components.NavDown:
		return offset + 1
	case components.NavUp:
		return max(0, offset-1)
	case components.NavHalfDown:
		return offset + rows/2
	case components.NavHalfUp:
		return max(0, offset-rows/2)
	case components.NavPageDown:
		return offset + rows
	case components.NavPageUp:
		return max(0, offset-rows)
	case components.NavTop:
		return 0
	case components.NavBottom:
		return offset + 1<<20 // clamped while rendering, which knows the length
	}
	return offset
}

// cursorWhen returns the cursor only for a focused list, so an unfocused one has
// no highlighted row.
func cursorWhen(focused bool, cursor int) int {
	if focused {
		return cursor
	}
	return -1
}
