package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// ViewID identifies a cockpit view.
type ViewID int

const (
	ViewDashboard ViewID = iota
	ViewPipelines
	ViewMRs
	ViewIssues
	ViewTodos
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
	// "overview" is what this tab was called before it became the dashboard; a
	// config that names it must keep working.
	case "dashboard", "overview":
		return ViewDashboard, true
	case "pipelines":
		return ViewPipelines, true
	case "mrs":
		return ViewMRs, true
	case "issues":
		return ViewIssues, true
	case "todos":
		return ViewTodos, true
	case "commits":
		return ViewCommits, true
	}
	return 0, false
}

// defaultViews are the tabs shown when settings.views is absent.
//
// Todos is last but is the only view that is not about the selected project: it
// is GitLab's own answer to "what is waiting on me". Anyone who starts their day
// there can make it the landing tab with settings.default_view.
//
// Commits is not among them: Overview already lists recent commits, and Enter
// opens the full commit page in place, so a separate tab would only offer a
// taller list of the same thing. It remains available via settings.views for
// anyone who wants it.
func defaultViews() []ViewID {
	return []ViewID{ViewDashboard, ViewPipelines, ViewMRs, ViewIssues, ViewTodos}
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

// A commit row is four columns: when it happened, its CI status, what kind of
// change it is, what it says, and who wrote it.
//
// The kind gets a column of its own because "fix:" and "refactor(api):" are
// different lengths: left inline, every subject started somewhere else and the eye
// had nothing to follow down the list. Three weights of text as everywhere else —
// the time, the kind and the author are metadata to scan past, the subject is what
// you read.
const (
	authorWidth      = 14 // the author column, after the subject
	kindMax          = 14 // a conventional prefix longer than this is truncated
	subjectMin       = 20 // below this the row is too narrow to bother aligning
	commitStampWidth = 12 // "24.12. 12:13", from util.CommitTime
)

// columnWidth is the width a column needs for these values: the widest of them,
// clamped.
//
// Sizing to content is what keeps the columns together. Padding one out to the full
// width instead leaves a canyon of blank between it and the next — the author
// stranded against the right edge, reading as a separate list rather than the last
// column of this one. Whatever is left over belongs at the end of the row.
//
// Measured over the whole list, not the visible window, so scrolling never shifts
// the text sideways.
func columnWidth(values []string, minWidth, maxWidth int) int {
	widest := 0
	for _, v := range values {
		if n := lipgloss.Width(v); n > widest {
			widest = n
		}
	}
	return min(max(widest, minWidth), maxWidth)
}

// commitColumns measures a commit list's two elastic columns: the conventional
// prefix and the subject beside it.
func commitColumns(titles []string, width int) (kindWidth, subjectWidth int) {
	kinds := make([]string, len(titles))
	subjects := make([]string, len(titles))
	for i, title := range titles {
		kinds[i], subjects[i] = splitConventional(title)
	}

	kindWidth = columnWidth(kinds, 0, kindMax)
	// when + space + icon(2) + space + kind + space + subject + space + author
	room := width - commitStampWidth - 1 - 2 - 1 - kindWidth - 1 - 1 - authorWidth
	return kindWidth, columnWidth(subjects, subjectMin, max(room, subjectMin))
}

// splitConventional splits "feat(scope): subject" into its prefix (with the colon)
// and the subject. A title that is not conventional is all subject.
func splitConventional(title string) (kind, subject string) {
	if i := strings.Index(title, ": "); i > 0 && i < 24 && !strings.Contains(title[:i], " ") {
		return title[:i+1], title[i+2:]
	}
	return "", title
}

// commitRow renders one row, with the column widths commitColumns measured for the
// list it belongs to.
func commitRow(when, icon, author, title string, kindWidth, subjectWidth, width int) string {
	kind, subject := splitConventional(title)

	row := fmt.Sprintf("%s %s %s %s",
		components.MutedStyle.Render(when),
		icon,
		components.MutedStyle.Render(components.PadRight(components.Truncate(kind, kindWidth), kindWidth)),
		components.BodyStyle.Render(components.PadRight(components.Truncate(subject, subjectWidth), subjectWidth)),
	)

	// The author comes along only if it fits beside the rest, not instead of it.
	if lipgloss.Width(row)+1+authorWidth <= width {
		row += " " + components.MutedStyle.Render(components.Truncate(author, authorWidth))
	}
	return row
}

// refAndTitle renders a row that names something by number — "!42", "#7" — and
// then says what it is. The number is how you refer to it, not what it is about,
// so it is metadata to scan past, exactly like a commit's "feat(scope):" prefix.
//
// Every list that carries a title goes through here, or the same commit reads one
// way in Recent Commits and another in Pipelines.
func refAndTitle(ref, title string) string {
	if ref == "" {
		return styleCommitTitle(title)
	}
	return components.MutedStyle.Render(ref) + " " + styleCommitTitle(title)
}

// styleCommitTitle dims a leading "type(scope):" so the subject stands out.
func styleCommitTitle(title string) string {
	if i := strings.Index(title, ": "); i > 0 && i < 24 && !strings.Contains(title[:i], " ") {
		return components.MutedStyle.Render(title[:i+1]) + components.BodyStyle.Render(title[i+1:])
	}
	return components.BodyStyle.Render(title)
}

// statusCmd puts a line in the shell's status bar.
func statusCmd(text string, isErr bool) tea.Cmd {
	return func() tea.Msg { return StatusMsg{Text: text, IsErr: isErr} }
}

// splitLines splits a rendered detail string into lines for RenderBox.
func splitLines(s string) []string { return strings.Split(s, "\n") }

// joinPanels puts two panels beside each other with a rule between them, which
// is what separates them now that neither has a frame.
func joinPanels(left, right string, height int) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", components.VRule(height), " ", right)
}

// renderListBox renders a scrollable, single-selection list as a body panel: a
// heading and a rule, then the rows. Header rows (prefixed with "\x00") are
// rendered but never selected.
//
// scroll points at the caller's stored scroll offset and is updated in place.
// The offset has to persist between frames: derived fresh from the cursor it
// would pin the cursor to an edge, and moving back would scroll the viewport
// instead of walking the cursor through it.
func renderListBox(width, height int, title string, items []string, cursor int, scroll *int) string {
	return renderRowsBox(width, height, title, len(items),
		func(i int) string { return items[i] }, cursor, scroll)
}

// renderRowsBox is renderListBox for a list whose rows are rendered on demand.
//
// A list of fifty commits shows about twenty-six of them, and styling a row is
// the most expensive thing a frame does — so the rows outside the window are
// never built at all. Every keypress redraws the body, so that is half the work
// of a frame saved on each one.
func renderRowsBox(width, height int, title string, total int, row func(int) string, cursor int, scroll *int) string {
	innerWidth := width       // panels have no side borders to pay for
	innerHeight := height - 1 // the heading takes one row
	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 0 {
		innerHeight = 0
	}

	// Keep the cursor visible with a margin of context beyond it.
	scrollOffset := 0
	if scroll != nil {
		*scroll = components.ScrollOffset(*scroll, cursor, total, innerHeight)
		scrollOffset = *scroll
	}

	contentLines := make([]string, 0, innerHeight)
	for i := scrollOffset; i < total && len(contentLines) < innerHeight; i++ {
		item := row(i)
		isHeader := len(item) > 0 && item[0] == '\x00'
		if isHeader {
			item = item[1:]
			// A stage heading is structure, not a row you can act on, so it keeps
			// the gutter empty and never highlights.
			contentLines = append(contentLines,
				strings.Repeat(" ", components.SelectionGutter)+components.Truncate(item, innerWidth-components.SelectionGutter))
			continue
		}
		contentLines = append(contentLines, components.SelectRow(item, innerWidth, i == cursor))
	}

	return components.RenderPanel(title, contentLines, width, height, true)
}
