package views

import (
	"fmt"
	"strings"
	"time"

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

// A commit row reads left to right as: how CI went, what kind of change it is, what
// it says, who wrote it, and when.
//
// The when is at the far right, past the author, rather than opening the row: it is
// the thing you look up rather than scan, and nineteen columns of metadata before
// the message was that much of every row spent on something the eye skips. The kind
// keeps a column of its own — "fix:" and "refactor:" are different lengths, and
// inline they left every subject starting somewhere else.
const (
	authorWidth  = 14 // the author column
	kindMax      = 10 // "refactor:" is the longest that matters
	subjectMin   = 20 // below this the row is too narrow to bother aligning
	updatedWidth = 14 // "30.12.25 08:00", the widest whole timestamp
)

// commitStamp is a commit's whole timestamp, date and time together, for the column
// at the right. Empty for a commit GitLab gave no date.
func commitStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	if t.Year() == time.Now().Year() {
		return fmt.Sprintf("%d.%d. %02d:%02d", t.Day(), int(t.Month()), t.Hour(), t.Minute())
	}
	return fmt.Sprintf("%d.%d.%02d %02d:%02d",
		t.Day(), int(t.Month()), t.Year()%100, t.Hour(), t.Minute())
}

// columnWidth is the width a column needs for these values: the widest of them,
// clamped.
//
// Sizing to content is what keeps the columns together. Padding one out to the full
// width instead leaves a canyon of blank between it and the next — the author
// stranded against the right edge, reading as a separate list rather than the last
// column of this one.
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

// commitLayout is the measured width of a commit list's columns.
type commitLayout struct{ kind, subject, updated int }

// commitColumns measures them over the whole list, so every row lines up.
func commitColumns(titles, stamps []string, width int) (cols commitLayout) {
	kinds := make([]string, len(titles))
	subjects := make([]string, len(titles))
	for i, title := range titles {
		kinds[i], subjects[i] = splitConventional(title)
	}

	cols.kind = columnWidth(kinds, 0, kindMax)
	cols.updated = columnWidth(stamps, 0, updatedWidth)

	// icon(2, with its space) + kind + space + subject + space + author + space + when
	room := width - 2 - cols.kind - 1 - 1 - authorWidth - 1 - cols.updated

	// The subject takes what is left, so the two right-hand columns land against the
	// right edge rather than trailing whatever the longest subject happened to be.
	cols.subject = max(room, subjectMin)
	return cols
}

// splitConventional splits "feat(scope): subject" into the kind of change and what
// it says. A title that is not conventional is all subject.
//
// The scope is dropped: aligning "refactor(#105934):" into a column cost thirteen
// characters of every row before the message even started, and a scope is almost
// always either repeated by the subject or an issue number that lives in the body.
// What the change is — feat, fix, docs — is the part worth a column.
func splitConventional(title string) (kind, subject string) {
	i := strings.Index(title, ": ")
	if i <= 0 || i >= 24 || strings.Contains(title[:i], " ") {
		return "", title
	}
	kind = title[:i]
	if open := strings.IndexByte(kind, '('); open > 0 {
		kind = kind[:open]
	}
	return kind + ":", title[i+2:]
}

// refAndTitle renders a row that names something by number — "!42", "#7" — and
// then says what it is. The number is how you refer to it, not what it is about,
// so it is metadata to scan past, exactly like a commit's "feat(scope):" prefix.
func refAndTitle(ref, title string) string {
	if ref == "" {
		return styleCommitTitle(title)
	}
	return components.MutedStyle.Render(ref) + " " + styleCommitTitle(title)
}

// styleCommitTitle dims a leading "type(scope):" so the subject stands out. Used
// where a title is one column rather than two — the merge-request and issue lists.
func styleCommitTitle(title string) string {
	if i := strings.Index(title, ": "); i > 0 && i < 24 && !strings.Contains(title[:i], " ") {
		return components.MutedStyle.Render(title[:i+1]) + components.BodyStyle.Render(title[i+1:])
	}
	return components.BodyStyle.Render(title)
}

// padLeft right-aligns s in w columns, which is how the kind column is set: the
// colons line up and the subject starts right after them, so the gap the eye has
// to cross sits before the dim text rather than between it and the message.
func padLeft(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

// commitRow renders one row with the widths commitColumns measured for its list.
func commitRow(icon, author, title, when string, cols commitLayout, width int) string {
	kind, subject := splitConventional(title)

	row := fmt.Sprintf("%s%s %s",
		icon, // already carries its own trailing space
		components.MutedStyle.Render(padLeft(components.Truncate(kind, cols.kind), cols.kind)),
		components.BodyStyle.Render(components.PadRight(components.Truncate(subject, cols.subject), cols.subject)),
	)

	// The two right-hand columns come along only if they fit beside the rest, never
	// instead of it: on a narrow terminal the message is what matters.
	if lipgloss.Width(row)+1+authorWidth <= width {
		row += " " + components.MutedStyle.Render(
			components.PadRight(components.Truncate(author, authorWidth), authorWidth))
	}
	if cols.updated > 0 && lipgloss.Width(row)+1+cols.updated <= width {
		row += " " + components.MutedStyle.Render(padLeft(when, cols.updated))
	}
	return row
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
