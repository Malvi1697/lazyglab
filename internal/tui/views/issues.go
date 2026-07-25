package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	cursor int

	status string
}

// NewIssuesView creates an IssuesView bound to the shared session context.
func NewIssuesView(ctx *Context) *IssuesView { return &IssuesView{ctx: ctx} }

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
			v.status = fmt.Sprintf("Error loading issues: %v", msg.Err)
			return nil
		}
		v.issues = msg.Issues
		v.clampCursor()
		v.status = fmt.Sprintf("Loaded %d issues", len(msg.Issues))
		return nil

	case StatusMsg:
		v.status = msg.Text
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

func (v *IssuesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if isNavUp(msg) {
		v.moveCursor(-1)
		return nil
	}
	if isNavDown(msg) {
		v.moveCursor(1)
		return nil
	}
	if key == keyTop {
		v.cursor = 0
		return nil
	}
	if key == keyBottom {
		v.cursor = len(v.issues) - 1
		if v.cursor < 0 {
			v.cursor = 0
		}
		return nil
	}
	if key == keyHalfDown {
		v.moveCursor(halfPage(v.height))
		return nil
	}
	if key == keyHalfUp {
		v.moveCursor(-halfPage(v.height))
		return nil
	}

	switch key {
	case keyClose:
		if v.cursor < len(v.issues) {
			issue := v.issues[v.cursor]
			action := "Close"
			if issue.State != "opened" {
				action = "Reopen"
			}
			return confirmCmd(fmt.Sprintf("%s #%d %s?", action, issue.IID, issue.Title), v.toggleIssue())
		}
	case keyOpenBrowse:
		return v.openIssueInBrowser()
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

	leftWidth := width * 45 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	left := renderListBox(leftWidth, height, "Issues", v.issueItems(), v.cursor)

	detail := v.issueDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderBox(v.detailTitle(), strings.Split(detail, "\n"), rightWidth, height, components.ColorSecondary, components.ColorPrimary)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *IssuesView) detailTitle() string {
	if v.cursor < len(v.issues) {
		return fmt.Sprintf("Issue (#%d)", v.issues[v.cursor].IID)
	}
	return "Issue"
}

// issueItems renders the issue list rows.
func (v *IssuesView) issueItems() []string {
	items := make([]string, len(v.issues))
	for i, issue := range v.issues {
		items[i] = fmt.Sprintf("#%d %s", issue.IID, issue.Title)
	}
	return items
}

func (v *IssuesView) issueDetail() string {
	if len(v.issues) == 0 {
		return "No issues"
	}
	if v.cursor >= len(v.issues) {
		return ""
	}
	issue := v.issues[v.cursor]

	labels := "none"
	if len(issue.Labels) > 0 {
		labels = strings.Join(issue.Labels, ", ")
	}
	assignees := "unassigned"
	if len(issue.Assignees) > 0 {
		assignees = strings.Join(issue.Assignees, ", ")
	}

	return fmt.Sprintf("%s\n\nAuthor: %s\nAssignees: %s\nLabels: %s\n\n%s\n\n%s",
		components.TitleStyle.Render(fmt.Sprintf("#%d %s", issue.IID, issue.Title)),
		issue.Author,
		assignees,
		labels,
		issue.Description,
		components.HelpDescStyle.Render(issue.WebURL),
	)
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *IssuesView) KeyHints() []KeyHint {
	return []KeyHint{
		{"c", "Close/reopen"},
		{"o", "Open"},
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
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.issues) == 0 {
		return nil
	}
	if v.cursor >= len(v.issues) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	issue := v.issues[v.cursor]
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

func (v *IssuesView) openIssueInBrowser() tea.Cmd {
	if v.cursor >= len(v.issues) {
		return nil
	}
	cmd := openBrowserCmd(v.issues[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *IssuesView) moveCursor(delta int) {
	n := len(v.issues)
	if n == 0 {
		return
	}
	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= n {
		v.cursor = n - 1
	}
}

func (v *IssuesView) clampCursor() {
	n := len(v.issues)
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}
