package views

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// readmeBox is a project's README, rendered as terminal text and scrolled like a
// diff or a log.
//
// The markdown is styled rather than converted: headings take the accent, list
// bullets and quotes their marker, fenced code goes grey. That is deliberately
// less than a markdown renderer would do — a full one means another dependency
// with its own palette to fight, and what a dashboard needs is for the README to
// be readable and to look like the rest of the app.
type readmeBox struct {
	file   string // the repository path, so a different project refetches
	source string
	loaded bool

	offset     int      // first visible line
	lines      []string // rendered, wrapped
	width      int      // the width they were wrapped to
	scrollable bool
}

// setReadme takes a fetched README. An empty body from a project that has none is
// still "loaded": there is nothing more to ask for.
func (b *readmeBox) setReadme(file, source string) {
	b.file, b.source, b.loaded = file, source, true
	b.offset, b.lines = 0, nil
}

// resetReadme forgets it, for when the project or branch changes.
func (b *readmeBox) resetReadme() {
	b.file, b.source, b.loaded = "", "", false
	b.offset, b.lines = 0, nil
}

// wants reports whether this README still needs fetching: the project has one and
// we do not hold it yet.
func (b *readmeBox) wants(file string) bool {
	return file != "" && (!b.loaded || b.file != file)
}

// readmeTitle names the box after the file it is showing.
func (b *readmeBox) readmeTitle() string {
	if b.file == "" {
		return "Readme"
	}
	return b.file
}

// readmeKey scrolls the README and reports whether it took the key.
func (b *readmeBox) readmeKey(key string, height int) bool {
	if act := components.NavFor(key); act != components.NavNone {
		b.offset = scrollBy(act, b.offset, listRows(height))
		return true
	}
	return false
}

// readmePanel renders the box, scrolled to its own offset.
func (b *readmeBox) readmePanel(width, height int, focused bool) string {
	contentWidth := width - 2
	if contentWidth < 20 {
		contentWidth = 20
	}

	rows := height - 1
	if rows < 1 {
		rows = 1
	}

	var body []string
	switch {
	case !b.loaded:
		body = []string{components.HelpDescStyle.Render("Loading…")}
	case strings.TrimSpace(b.source) == "":
		body = []string{components.HelpDescStyle.Render("This project has no README.")}
	default:
		lines := b.render(contentWidth)
		b.scrollable = len(lines) > rows
		if maxScroll := len(lines) - rows; b.offset > maxScroll {
			b.offset = max(0, maxScroll)
		}
		if b.offset < 0 {
			b.offset = 0
		}
		body = lines[b.offset:min(b.offset+rows, len(lines))]
	}

	return components.RenderPanel(b.readmeTitle(), body, width, height, focused)
}

// render styles the README and wraps it to width, remembering the result: a
// keypress redraws the body, and re-wrapping a thousand lines each time is the
// same waste a job log used to be.
func (b *readmeBox) render(width int) []string {
	if b.lines != nil && b.width == width {
		return b.lines
	}

	var out []string
	inFence := false
	for _, raw := range strings.Split(strings.TrimRight(b.source, "\n"), "\n") {
		line := strings.TrimRight(raw, " \t")

		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			out = append(out, components.FaintStyle.Render(components.Truncate(line, width)))
			continue
		}
		if inFence {
			// Code is shown as written — wrapping it would break what it is.
			out = append(out, components.MutedStyle.Render(components.Truncate(line, width)))
			continue
		}

		out = append(out, styleMarkdownLine(line, width)...)
	}

	b.lines, b.width = out, width
	return out
}

// styleMarkdownLine renders one line of markdown, wrapped to width.
func styleMarkdownLine(line string, width int) []string {
	trimmed := strings.TrimSpace(line)

	switch {
	case trimmed == "":
		return []string{""}

	case strings.HasPrefix(trimmed, "#"):
		// A heading is the only structure worth the accent, as a diff's hunk header is.
		text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		style := components.TitleStyle
		if strings.HasPrefix(trimmed, "###") {
			style = components.MutedTitleStyle // deeper headings are quieter
		}
		return wrapStyled(text, width, style)

	case isRule(trimmed):
		return []string{components.FaintStyle.Render(strings.Repeat("─", width))}

	case strings.HasPrefix(trimmed, ">"):
		text := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		return wrapStyled("│ "+text, width, components.MutedStyle)
	}

	// A list keeps its indentation and gets a bullet that is one character wide in
	// every terminal.
	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	if marker, rest, ok := listItem(trimmed); ok {
		prefix := indent + components.MutedStyle.Render(marker) + " "
		wrapped := components.WrapLine(rest, max(width-lipgloss.Width(prefix), 8))
		out := make([]string, 0, len(wrapped))
		for i, frag := range wrapped {
			if i == 0 {
				out = append(out, prefix+frag)
				continue
			}
			out = append(out, indent+"  "+frag)
		}
		return out
	}

	return components.WrapLine(line, width)
}

// wrapStyled wraps text and applies one style to every line of it.
func wrapStyled(text string, width int, style lipgloss.Style) []string {
	wrapped := components.WrapLine(text, width)
	out := make([]string, 0, len(wrapped))
	for _, l := range wrapped {
		out = append(out, style.Render(l))
	}
	return out
}

// listItem splits "- text", "* text", "1. text" into a bullet and the rest.
func listItem(trimmed string) (marker, rest string, ok bool) {
	for _, m := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(trimmed, m) {
			return "•", strings.TrimPrefix(trimmed, m), true
		}
	}
	// An ordered item keeps its own number; it is part of the meaning.
	for i, r := range trimmed {
		if r >= '0' && r <= '9' {
			continue
		}
		if (r == '.' || r == ')') && i > 0 && i < 4 && len(trimmed) > i+1 && trimmed[i+1] == ' ' {
			return trimmed[:i+1], strings.TrimSpace(trimmed[i+1:]), true
		}
		break
	}
	return "", "", false
}

// isRule reports whether a line is a horizontal rule.
func isRule(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	for _, c := range []string{"-", "*", "_"} {
		if strings.Trim(trimmed, c) == "" {
			return true
		}
	}
	return false
}
