package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// ViewID identifies a cockpit view.
type ViewID int

const (
	ViewOverview ViewID = iota
	ViewPipelines
	ViewMRs
	ViewIssues
	ViewCommits
)

// Context is the shared session state handed to every view. Views read it; the
// shell owns and mutates it.
type Context struct {
	Client  *gitlab.Client
	Project *gitlab.Project // nil until selected
	Branch  *gitlab.Branch  // nil = default/all
}

// KeyHint is a footer hint.
type KeyHint struct{ Key, Desc string }

// View is one full-screen cockpit view.
type View interface {
	Focus() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	Body(width, height int) string
	Title() string
	KeyHints() []KeyHint
}

func viewIDFromName(name string) (ViewID, bool) {
	switch name {
	case "overview":
		return ViewOverview, true
	case "pipelines":
		return ViewPipelines, true
	case "mrs":
		return ViewMRs, true
	case "issues":
		return ViewIssues, true
	case "commits":
		return ViewCommits, true
	}
	return 0, false
}

// defaultViews are the tabs shown when settings.views is absent.
//
// Commits is not among them: Overview already lists recent commits, and Enter
// opens the full commit page in place, so a separate tab would only offer a
// taller list of the same thing. It remains available via settings.views for
// anyone who wants it.
func defaultViews() []ViewID {
	return []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues}
}

// ParseViews converts config names into an ordered, deduplicated ViewID list.
// Empty -> all in default order. Unknown/duplicate dropped with warnings.
func ParseViews(names []string) ([]ViewID, []string) {
	if len(names) == 0 {
		return defaultViews(), nil
	}
	var out []ViewID
	var warnings []string
	seen := make(map[ViewID]bool)
	for _, n := range names {
		id, ok := viewIDFromName(n)
		if !ok {
			warnings = append(warnings, "unknown view \""+n+"\" ignored")
			continue
		}
		if seen[id] {
			warnings = append(warnings, "duplicate view \""+n+"\" ignored")
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return defaultViews(), warnings
	}
	return out, warnings
}

// DefaultViewIndex returns the index of the configured default view within the
// enabled list, or 0 when unset/absent.
func DefaultViewIndex(views []ViewID, name string) int {
	id, ok := viewIDFromName(name)
	if !ok {
		return 0
	}
	for i, v := range views {
		if v == id {
			return i
		}
	}
	return 0
}

// authorWidth is the fixed width of the author column in commit lists, so the
// titles beside it line up.
const authorWidth = 16

// splitLines splits a rendered detail string into lines for RenderBox.
func splitLines(s string) []string { return strings.Split(s, "\n") }

// joinH joins two rendered blocks side by side, top-aligned.
func joinH(a, b string) string { return lipgloss.JoinHorizontal(lipgloss.Top, a, b) }

// renderListBox renders a bordered, scrollable, single-selection list. The
// cockpit shows one view at a time, so the rendered list is always the focused
// one; pass a negative cursor to render without a selection.
// Header lines (prefixed with "\x00") are rendered but never highlighted.
//
// scroll points at the caller's stored scroll offset and is updated in place.
// The offset has to persist between frames: derived fresh from the cursor it
// would pin the cursor to an edge, and moving back would scroll the viewport
// instead of walking the cursor through it.
func renderListBox(width, height int, title string, items []string, cursor int, scroll *int) string {
	innerWidth := width - 4   // border + padding on each side
	innerHeight := height - 2 // top + bottom border
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 0 {
		innerHeight = 0
	}

	// Keep the cursor visible with a margin of context beyond it.
	scrollOffset := 0
	if scroll != nil {
		*scroll = components.ScrollOffset(*scroll, cursor, len(items), innerHeight)
		scrollOffset = *scroll
	}

	var contentLines []string
	for i := scrollOffset; i < len(items) && len(contentLines) < innerHeight; i++ {
		item := items[i]
		isHeader := len(item) > 0 && item[0] == '\x00'
		if isHeader {
			item = item[1:]
		}
		displayItem := components.Truncate(item, innerWidth)
		if i == cursor && !isHeader {
			plain := ansi.Strip(displayItem)
			visW := lipgloss.Width(plain)
			if visW < innerWidth {
				plain += strings.Repeat(" ", innerWidth-visW)
			}
			displayItem = components.SelectedItemStyle.Render(plain)
		}
		contentLines = append(contentLines, displayItem)
	}

	return components.RenderBox(title, contentLines, width, height, components.ColorPrimary, components.ColorPrimary)
}
