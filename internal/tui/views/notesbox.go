package views

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// notesBox is a discussion: the comments on a merge request or an issue, as a list of
// who said what, and the whole thread behind Enter.
type notesBox struct {
	notes  []gitlab.Note
	cursor int // indexes the shown notes, not all of them
	scroll int

	// showSystem includes GitLab's own bookkeeping — "changed the description", "added 3
	// commits".
	showSystem bool

	// threadOpen is the whole thread, rendered as prose and scrolled as a body — a
	// conversation is read end to end, not row by row.
	threadOpen   bool
	threadScroll int
	threadLines  []string // rendered thread, kept per width like a diff is
	threadWidth  int
	scrollable   bool
}

// setNotes takes a fetched discussion, keeping the cursor in range.
func (b *notesBox) setNotes(notes []gitlab.Note) {
	b.notes = notes
	b.cursor = clampCursor(b.cursor, len(b.shown()))
	b.threadLines = nil
}

// shown is the notes on screen: everything, or only what people wrote.
func (b *notesBox) shown() []gitlab.Note {
	if b.showSystem || b.human() == 0 {
		return b.notes
	}
	out := make([]gitlab.Note, 0, b.human())
	for _, n := range b.notes {
		if !n.System {
			out = append(out, n)
		}
	}
	return out
}

// toggleSystem shows or hides the bookkeeping, and says which.
func (b *notesBox) toggleSystem() tea.Cmd {
	if b.human() == 0 {
		return statusCmd("This discussion is only GitLab's own record", false)
	}
	b.showSystem = !b.showSystem
	b.cursor = clampCursor(b.cursor, len(b.shown()))
	b.threadLines = nil
	if b.showSystem {
		return statusCmd("Showing GitLab's own record too", false)
	}
	return statusCmd("Showing what people wrote", false)
}

// resetNotes forgets the discussion: another merge request or issue is opening.
func (b *notesBox) resetNotes() {
	b.notes = nil
	b.cursor, b.scroll = 0, 0
	b.threadOpen, b.threadScroll = false, 0
	b.threadLines = nil
}

// human counts the comments a person actually wrote, which is what the heading
// promises: a thread of thirty system notes is not a discussion.
func (b *notesBox) human() int {
	n := 0
	for _, note := range b.notes {
		if !note.System {
			n++
		}
	}
	return n
}

// notesTitle names the box: how many are shown, and of how many there are when some are
// being left out.
func (b *notesBox) notesTitle() string {
	if len(b.notes) == 0 {
		return "Discussion"
	}
	if shown := len(b.shown()); shown != len(b.notes) {
		return fmt.Sprintf("Discussion (%d of %d)", shown, len(b.notes))
	}
	return fmt.Sprintf("Discussion (%d)", len(b.notes))
}

// noteRow renders one row: when, who, and the first line of what they said.
func (b *notesBox) noteRow(i int) string {
	n := b.shown()[i]
	who := components.MutedStyle.Render(components.PadRight(components.Truncate(n.Author, 14), 14))
	first := firstLine(n.Body)
	if n.System {
		// GitLab's own bookkeeping: part of the record, a weight quieter.
		return fmt.Sprintf("%s %s %s",
			components.MutedStyle.Render(util.TimeAgoShort(n.CreatedAt)), who,
			components.MutedStyle.Render(first))
	}
	mark := ""
	switch {
	case n.Resolved:
		mark = components.StatusIcon("success") + " "
	case n.Resolvable:
		mark = components.StatusIcon("pending") + " "
	}
	return fmt.Sprintf("%s %s %s%s",
		components.MutedStyle.Render(util.TimeAgoShort(n.CreatedAt)), who, mark,
		components.BodyStyle.Render(first))
}

// notesPanel renders the discussion as a page column.
func (b *notesBox) notesPanel(width, height int, focused, loading bool) string {
	if len(b.notes) == 0 {
		msg := "No comments yet"
		if loading {
			msg = "Loading…"
		}
		return components.RenderPanel(b.notesTitle(),
			[]string{components.HelpDescStyle.Render(msg)}, width, height, focused)
	}
	return renderRowsBox(width, height, b.notesTitle(), len(b.shown()), b.noteRow,
		cursorWhen(focused, b.cursor), &b.scroll)
}

// notesKey drives the list and reports whether it took the key.
func (b *notesBox) notesKey(key string, height int) bool {
	if key == keyEnter {
		if len(b.notes) > 0 {
			b.threadOpen = true
			b.threadScroll = 0
		}
		return true
	}
	if act := components.NavFor(key); act != components.NavNone {
		b.cursor = components.ApplyNav(act, b.cursor, len(b.shown()), listRows(height))
		return true
	}
	return false
}

// threadKey scrolls the open thread.
func (b *notesBox) threadKey(key string, height int) bool {
	if act := components.NavFor(key); act != components.NavNone {
		b.threadScroll = scrollBy(act, b.threadScroll, listRows(height))
		return true
	}
	return false
}

// closeThread goes back from the thread to the list.
func (b *notesBox) closeThread() {
	b.threadOpen = false
	b.threadScroll = 0
}

// threadTitle names the open thread.
func (b *notesBox) threadTitle() string {
	shown := len(b.shown())
	if shown != len(b.notes) {
		return fmt.Sprintf("Discussion (%d of %d comments)", shown, len(b.notes))
	}
	return fmt.Sprintf("Discussion (%d comments)", shown)
}

// threadView renders the whole conversation, scrolled to threadScroll.
func (b *notesBox) threadView(width, height int) string {
	lines := b.thread(width - 2)

	rows := height - 1
	if rows < 1 {
		rows = 1
	}
	b.scrollable = len(lines) > rows
	if maxScroll := len(lines) - rows; b.threadScroll > maxScroll {
		b.threadScroll = max(0, maxScroll)
	}
	if b.threadScroll < 0 {
		b.threadScroll = 0
	}
	end := min(b.threadScroll+rows, len(lines))
	return strings.Join(lines[b.threadScroll:end], "\n")
}

// thread renders every comment as prose: who and when as a heading, the body wrapped
// beneath it.
func (b *notesBox) thread(width int) []string {
	if b.threadLines != nil && b.threadWidth == width {
		return b.threadLines
	}
	if width < 20 {
		width = 20
	}

	var out []string
	for i, n := range b.shown() {
		if i > 0 {
			out = append(out, "")
		}
		when := util.TimeAgo(n.CreatedAt)
		if n.System {
			// One line, quiet: it is the record, not the conversation.
			line := fmt.Sprintf("%s %s · %s", n.Author, collapse(n.Body), when)
			out = append(out, components.MutedStyle.Render(components.Truncate(line, width)))
			continue
		}

		head := components.TitleStyle.Render(n.Author) + components.MutedStyle.Render("  "+when)
		if n.OnPath != "" {
			head += components.MutedStyle.Render(fmt.Sprintf("  on %s:%d", n.OnPath, n.OnLine))
		}
		if n.Resolved {
			head += "  " + components.StatusIcon("success") + components.MutedStyle.Render(" resolved")
		}
		out = append(out, head)
		for _, line := range strings.Split(strings.TrimRight(n.Body, "\n"), "\n") {
			out = append(out, components.WrapLine(line, width)...)
		}
	}
	if len(out) == 0 {
		out = []string{components.HelpDescStyle.Render("No comments yet")}
	}

	b.threadLines, b.threadWidth = out, width
	return out
}

// threadHints are the footer hints while the thread has the screen.
func (b *notesBox) threadHints() []KeyHint {
	hints := []KeyHint{}
	if b.scrollable {
		hints = append(hints, KeyHint{"j/k", "Scroll"})
	}
	hints = append(hints, KeyHint{"c", "Comment"})
	hints = append(hints, b.systemToggleHint()...)
	return append(hints, KeyHint{"Esc", "Back"})
}

// boxHints are the footer hints while the discussion box has the focus.
func (b *notesBox) boxHints() []KeyHint {
	hints := []KeyHint{{"Enter", "Read the thread"}, {"c", "Comment"}}
	hints = append(hints, b.systemToggleHint()...)
	return append(hints, KeyHint{"j/k", "Move"})
}

// systemToggleHint offers s only where it would change what is on screen: there is no
// point offering to hide the record when the record is all there is.
func (b *notesBox) systemToggleHint() []KeyHint {
	if human := b.human(); human == 0 || human == len(b.notes) {
		return nil
	}
	return []KeyHint{{"s", b.systemHint()}}
}

// systemHint says what s would do, in the terms of what is on screen.
func (b *notesBox) systemHint() string {
	if b.showSystem {
		return "Hide the record"
	}
	return "Show the record"
}

// firstLine is a note's opening line, for a list row.
func firstLine(body string) string {
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return strings.TrimSpace(body)
}

// collapse squeezes a system note onto one line and drops the HTML GitLab writes into
// some of them ("added 22 commits <ul><li><code>…").
func collapse(body string) string {
	return strings.Join(strings.Fields(stripHTML(body)), " ")
}

// stripHTML removes tags, leaving their text.
func stripHTML(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
			b.WriteByte(' ')
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// commentWrittenMsg carries what the editor left behind: the text to post, or an empty
// body when the comment was abandoned.
type commentWrittenMsg struct {
	body string
	err  error
}

// composeComment opens the user's editor on a scratch file and returns what they wrote.
func composeComment(subject string) tea.Cmd {
	path, err := commentFile(subject)
	if err != nil {
		return func() tea.Msg { return commentWrittenMsg{err: err} }
	}

	editor, args := editorCommand()
	cmd := exec.Command(editor, append(args, path)...) //nolint:gosec // the editor is the user's own
	return tea.ExecProcess(cmd, func(runErr error) tea.Msg {
		defer func() { _ = os.Remove(path) }()
		if runErr != nil {
			return commentWrittenMsg{err: fmt.Errorf("%s: %w", editor, runErr)}
		}
		raw, readErr := os.ReadFile(path) //nolint:gosec // a file we just wrote
		if readErr != nil {
			return commentWrittenMsg{err: readErr}
		}
		return commentWrittenMsg{body: stripCommentLines(string(raw))}
	})
}

// commentFile writes the scratch file the editor opens.
func commentFile(subject string) (string, error) {
	f, err := os.CreateTemp("", "lazyglab-comment-*.md")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	template := fmt.Sprintf(
		"\n# Write your comment above, in Markdown.\n"+
			"# Lines starting with '#' are ignored, and an empty comment is not posted.\n"+
			"#\n# On: %s\n", subject)
	if _, err := f.WriteString(template); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// editorCommand is the editor to use, in git's order of preference.
func editorCommand() (string, []string) {
	for _, env := range []string{"GIT_EDITOR", "VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			// An editor variable may carry flags ("code -w", "nvim -u NONE").
			parts := strings.Fields(v)
			return parts[0], parts[1:]
		}
	}
	return "vi", nil
}

// stripCommentLines removes the template's '#' lines and trims the rest.
func stripCommentLines(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
