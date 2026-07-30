package views

import (
	"fmt"
	"image/color"
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

// Every list in the app is laid out the same way, because they are all the same
// kind of thing: something with a name, a state, a subject, and some metadata about
// who and when. Left to right —
//
//	 !42   feat: ● capacity-aware promotion        jiri.kucera   promo-cap   28.7. 20:28
//	        fix: ▲ count only free wild cards      Jan Všetíček              27.7. 15:35
//	#1234       · Login crashes on Safari          alice                      4.7. 09:02
//
// what it is called, what kind of change it is, how CI went, what it says, then who,
// where, and when.
//
// The when is at the far right rather than opening the row: it is the thing you look
// up rather than scan, and nineteen columns of metadata before the message was that
// much of every row spent on something the eye skips. The name and the kind keep
// columns of their own and are right-aligned, so the numbers and the colons line up
// and the subject starts in the same place on every row.
//
// Each list fills them in as far as it has them; an empty column costs nothing.
const (
	refWidth    = 5  // "!219", "#1234"
	kindMax     = 10 // "refactor:" is the longest that matters
	authorWidth = 14 // the author column
	extraMax    = 18 // a source branch, a project name
	marksMax    = 24 // twelve stage marks, which is more stages than anyone has
	stampWidth  = 14 // "30.12.25 08:00", the widest whole timestamp
	subjectMin  = 20 // below this the row is too narrow to bother aligning
)

// listRow is what one row says. Every field is optional: a list that has no CI to
// report, or no author, simply leaves it empty and no column is drawn for it.
type listRow struct {
	ref     string // how you refer to it: "!42", "#7"
	kind    string // "feat:", or why a to-do exists
	icon    string // the CI mark, two cells including its trailing space
	subject string
	marks   string // already-styled marks that follow the subject, one per pipeline stage
	author  string
	extra   string // a source branch, a project — whatever the list's third fact is
	stamp   string // when, from commitStamp

	// kindColor overrides the metadata grey for the kind, which the to-do list uses
	// to say that something is broken rather than merely waiting.
	kindColor color.Color
	// dimPrefix dims a conventional prefix inside the subject, for a list whose kind
	// column says something else and so cannot hold it.
	dimPrefix bool
}

// listColumns is the measured width of each column of a list.
type listColumns struct{ ref, kind, icon, subject, marks, author, extra, stamp int }

// measureColumns sizes the columns to the whole list — not to the visible window, so
// scrolling never shifts the text sideways — and gives the subject everything left
// over, which is what puts the right-hand columns against the right edge instead of
// trailing whatever the longest subject happened to be.
func measureColumns(rows []listRow, width int) listColumns {
	var cols listColumns
	measure := func(get func(listRow) string, maxWidth int) int {
		values := make([]string, len(rows))
		for i, r := range rows {
			values[i] = get(r)
		}
		return columnWidth(values, 0, maxWidth)
	}

	cols.ref = measure(func(r listRow) string { return r.ref }, refWidth)
	cols.kind = measure(func(r listRow) string { return r.kind }, kindMax)
	cols.author = measure(func(r listRow) string { return r.author }, authorWidth)
	cols.extra = measure(func(r listRow) string { return r.extra }, extraMax)
	cols.stamp = measure(func(r listRow) string { return r.stamp }, stampWidth)
	// The marks arrive already styled, so they are measured in display columns and
	// never truncated: half a row of stage marks would be a lie about how far the
	// pipeline got.
	cols.marks = measure(func(r listRow) string { return r.marks }, marksMax)
	for _, r := range rows {
		if r.icon != "" {
			cols.icon = lipgloss.Width(r.icon)
			break
		}
	}

	left := cols.icon // the icon carries its own trailing space
	for _, w := range []int{cols.ref, cols.kind} {
		if w > 0 {
			left += w + 1
		}
	}
	right := 0
	for _, w := range []int{cols.marks, cols.author, cols.extra, cols.stamp} {
		if w > 0 {
			right += w + 1
		}
	}
	cols.subject = max(width-left-right, subjectMin)
	return cols
}

// renderListRow renders one row to the widths measured for its list.
func renderListRow(r listRow, cols listColumns, width int) string {
	kindStyle := components.MutedStyle
	if r.kindColor != nil {
		kindStyle = lipgloss.NewStyle().Foreground(r.kindColor)
	}

	// The left-hand columns are right-aligned and the CI mark follows them, so all of
	// it sits against the message: the numbers line up, the colons line up, the marks
	// line up, and the only ragged edge is the blank at the very left. With the mark
	// out on the edge instead, it was stranded across a gap that changed width from
	// row to row.
	row := ""
	if cols.ref > 0 {
		row += components.MutedStyle.Render(padLeft(components.Truncate(r.ref, cols.ref), cols.ref)) + " "
	}
	if cols.kind > 0 {
		row += kindStyle.Render(padLeft(components.Truncate(r.kind, cols.kind), cols.kind)) + " "
	}
	row += r.icon

	subject := components.PadRight(components.Truncate(r.subject, cols.subject), cols.subject)
	if r.dimPrefix {
		row += styleCommitTitle(subject)
	} else {
		row += components.BodyStyle.Render(subject)
	}

	// The right-hand columns come along only if they fit beside the rest, never
	// instead of it: on a narrow terminal the message is what matters, and a branch
	// cut to eight characters says nothing.
	for _, col := range []struct {
		width  int
		text   string
		right  bool
		styled bool
	}{
		{cols.marks, r.marks, false, true},
		{cols.author, r.author, false, false},
		{cols.extra, r.extra, false, false},
		{cols.stamp, r.stamp, true, false},
	} {
		if col.width == 0 {
			continue
		}
		if lipgloss.Width(row)+1+col.width > width {
			break
		}
		if col.styled {
			// Already carries its own colours — the stage marks. Padding only.
			row += " " + col.text + strings.Repeat(" ", max(col.width-lipgloss.Width(col.text), 0))
			continue
		}
		text := components.Truncate(col.text, col.width)
		if col.right {
			text = padLeft(text, col.width)
		} else {
			text = components.PadRight(text, col.width)
		}
		row += " " + components.MutedStyle.Render(text)
	}
	return row
}

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
