package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// Local key constant specific to the Issues view (see pipelines.go for the
// shared subset).
const keyClose = "c"

// IssuesView is the self-contained cockpit view for issues.
type IssuesView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	issues []gitlab.Issue
	cursor int // indexes the visible (searched) list, not issues
	scroll int // first visible row, kept across frames

	search listSearch

	// detail is the in-place issue page, opened with Enter: the issue and its
	// discussion, which is the one thing the list row does not carry.
	detail issueDetail
}

// NewIssuesView creates an IssuesView bound to the shared session context.
func NewIssuesView(ctx *Context) *IssuesView {
	return &IssuesView{ctx: ctx, detail: newIssueDetail(ctx)}
}

// Title implements View.
func (v *IssuesView) Title() string { return "Issues" }

// Focus implements View: loads issues for the active project.
func (v *IssuesView) Focus() tea.Cmd { return v.load() }

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *IssuesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case IssuesLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading issues: %v", msg.Err), true)
		}
		v.issues = msg.Issues
		v.clampCursor()
		return nil

	case tea.PasteMsg:
		if !v.detail.active {
			v.search.paste(msg.Content, &v.cursor)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	// Everything else may belong to the issue page.
	return v.detail.update(msg)
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *IssuesView) CapturingText() bool { return !v.detail.active && v.search.capturing() }

// stepIssue moves to the neighbouring issue, keeping the page open.
func (v *IssuesView) stepIssue(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selected(), v.cursor, len(visible))
}

// visible is the issues matching the search; the cursor indexes it.
func (v *IssuesView) visible() []gitlab.Issue {
	return filtered(v.issues, v.search.filter, func(i gitlab.Issue) string {
		return fmt.Sprintf("#%d %s %s %s", i.IID, i.Title, i.Author, strings.Join(i.Labels, " "))
	})
}

// selected returns the highlighted issue, or nil.
func (v *IssuesView) selected() *gitlab.Issue {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *IssuesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring issue belongs to the list's owner — but not
		// while the thread is open, where the arrows belong to what you are reading.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepIssue(step)
		}
		return v.detail.handleKey(key, v.height)
	}

	if v.search.handleKey(msg, &v.cursor) {
		return nil
	}

	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.visible()), listRows(v.height))
		return nil
	}

	issue := v.selected()
	if issue == nil {
		return nil
	}
	switch key {
	case keyEnter:
		return v.detail.openAt(issue, v.cursor, len(v.visible()))
	case keyClose:
		action := "Close"
		if issue.State != "opened" {
			action = "Reopen"
		}
		return confirmCmd(fmt.Sprintf("%s #%d %s?", action, issue.IID, issue.Title), v.toggleIssue())
	case keyCopy:
		return copyRef(fmt.Sprintf("#%d", issue.IID))
	case keyCopyLink:
		return copyLink(fmt.Sprintf("#%d", issue.IID), issue.WebURL)
	case keyOpenBrowse:
		if cmd := openBrowserCmd(issue.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail panel on the right.
func (v *IssuesView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into an issue: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	leftWidth := width * 45 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	visible := v.visible()
	left := renderRowsBox(leftWidth, height,
		v.search.title("Issues", len(visible), len(v.issues)),
		len(visible), func(i int) string { return issueRow(visible[i]) },
		v.cursor, &v.scroll)

	detail := v.issueDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderPanel(v.detailTitle(), strings.Split(detail, "\n"), rightWidth-4, height, false)

	return joinPanels(left, right, height)
}

func (v *IssuesView) detailTitle() string {
	if issue := v.selected(); issue != nil {
		return fmt.Sprintf("Issue (#%d)", issue.IID)
	}
	return "Issue"
}

// issueRow renders one issue list row.
func issueRow(issue gitlab.Issue) string {
	return refAndTitle(fmt.Sprintf("#%d", issue.IID), issue.Title)
}

func (v *IssuesView) issueDetail() string {
	if len(v.issues) == 0 {
		return "No issues"
	}
	issue := v.selected()
	if issue == nil {
		if v.search.on() {
			return components.HelpDescStyle.Render("No issue matches " + v.search.filter.Query)
		}
		return ""
	}

	labels := "none"
	if len(issue.Labels) > 0 {
		labels = strings.Join(issue.Labels, ", ")
	}
	assignees := "unassigned"
	if len(issue.Assignees) > 0 {
		assignees = strings.Join(issue.Assignees, ", ")
	}

	return fmt.Sprintf("%s\n\nAuthor: %s\nAssignees: %s\nLabels: %s\n\n%s\n\n%s\n\n%s",
		components.TitleStyle.Render(fmt.Sprintf("#%d %s", issue.IID, issue.Title)),
		issue.Author,
		assignees,
		labels,
		issue.Description,
		components.HelpDescStyle.Render(issue.WebURL),
		components.HelpDescStyle.Render("Enter: the issue page and its discussion"),
	)
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *IssuesView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	return []KeyHint{
		{"Enter", "Issue page"},
		{"c", "Close/reopen"},
		{"o", "Open"},
		{"y/Y", "Copy #/link"},
		v.search.hint(),
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *IssuesView) load() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	return func() tea.Msg {
		issues, err := client.ListIssues(projectID)
		return IssuesLoadedMsg{Issues: issues, Err: err}
	}
}

func (v *IssuesView) toggleIssue() tea.Cmd {
	selected := v.selected()
	if selected == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	issue := *selected
	return func() tea.Msg {
		var err error
		if issue.State == "opened" {
			err = client.CloseIssue(projectID, issue.IID)
		} else {
			err = client.ReopenIssue(projectID, issue.IID)
		}
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Toggle issue failed: %v", err), IsErr: true}
		}
		action := "Closed"
		if issue.State != "opened" {
			action = "Reopened"
		}
		return StatusMsg{Text: fmt.Sprintf("%s #%d", action, issue.IID)}
	}
}

// ============================================================================
// Helpers
// ============================================================================

func (v *IssuesView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}
