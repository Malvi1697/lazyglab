package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// OverviewView is a read-only dashboard summarizing recent activity across
// commits, pipelines, merge requests, and issues.
type OverviewView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	commits   []gitlab.Commit
	pipelines []gitlab.Pipeline
	mrs       []gitlab.MergeRequest
	issues    []gitlab.Issue
}

// NewOverviewView creates an OverviewView bound to the shared session context.
func NewOverviewView(ctx *Context) *OverviewView { return &OverviewView{ctx: ctx} }

// Title implements View.
func (v *OverviewView) Title() string { return "Overview" }

// Focus implements View: loads commits, pipelines, merge requests, and issues
// concurrently for the active project/branch.
func (v *OverviewView) Focus() tea.Cmd {
	return tea.Batch(v.loadCommits(), v.loadPipelines(), v.loadMRs(), v.loadIssues())
}

// ============================================================================
// Update
// ============================================================================

// Update implements View. Overview is read-only: no key handling here,
// navigation and view switching are handled by the shell.
func (v *OverviewView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case CommitsLoadedMsg:
		if msg.Err == nil {
			v.commits = msg.Commits
		}
		return nil

	case PipelinesLoadedMsg:
		if msg.Err == nil {
			v.pipelines = msg.Pipelines
		}
		return nil

	case MRsLoadedMsg:
		if msg.Err == nil {
			v.mrs = msg.MRs
		}
		return nil

	case IssuesLoadedMsg:
		if msg.Err == nil {
			v.issues = msg.Issues
		}
		return nil
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: top half is a recent-commits list, bottom half is
// three side-by-side summary boxes for pipelines, merge requests, and issues.
func (v *OverviewView) Body(width, height int) string {
	v.width = width
	v.height = height

	topHeight := height / 2
	bottomHeight := height - topHeight
	if topHeight < 1 {
		topHeight = 1
	}
	if bottomHeight < 1 {
		bottomHeight = 1
	}

	top := renderListBox(width, topHeight, v.commitsTitle(), v.commitItems(), -1, false)

	colWidth := width / 3
	lastColWidth := width - colWidth*2

	pipelinesBox := components.RenderBox(v.pipelinesTitle(), v.pipelineLines(), colWidth, bottomHeight, components.ColorSecondary, components.ColorSecondary)
	mrsBox := components.RenderBox(v.mrsTitle(), v.mrLines(), colWidth, bottomHeight, components.ColorSecondary, components.ColorSecondary)
	issuesBox := components.RenderBox(v.issuesTitle(), v.issueLines(), lastColWidth, bottomHeight, components.ColorSecondary, components.ColorSecondary)

	bottom := lipgloss.JoinHorizontal(lipgloss.Top, pipelinesBox, mrsBox, issuesBox)

	return lipgloss.JoinVertical(lipgloss.Left, top, bottom)
}

func (v *OverviewView) commitsTitle() string {
	return fmt.Sprintf("Recent Commits (%d)", len(v.commits))
}

func (v *OverviewView) pipelinesTitle() string {
	return fmt.Sprintf("Pipelines (%d)", len(v.pipelines))
}

func (v *OverviewView) mrsTitle() string {
	return fmt.Sprintf("Merge Requests (%d)", len(v.mrs))
}

func (v *OverviewView) issuesTitle() string {
	return fmt.Sprintf("Issues (%d)", len(v.issues))
}

// commitItems renders the recent-commits rows, mapping each commit's CI
// status from the loaded pipelines by SHA.
func (v *OverviewView) commitItems() []string {
	items := make([]string, len(v.commits))
	for i, c := range v.commits {
		status := commitStatus(c.ShortID, v.pipelines)
		icon := "  "
		if status != "" {
			icon = components.StatusIconPadded(status)
		}
		items[i] = fmt.Sprintf("%s %s %s  %s",
			util.TimeAgoShort(c.CreatedAt),
			icon,
			components.PadRight(c.ShortID, 8),
			c.Title,
		)
	}
	return items
}

// pipelineLines renders a short list of the most recent pipelines.
func (v *OverviewView) pipelineLines() []string {
	const maxRows = 8
	n := len(v.pipelines)
	if n > maxRows {
		n = maxRows
	}
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		p := v.pipelines[i]
		title := p.CommitTitle
		if title == "" {
			title = p.Ref
		}
		lines[i] = fmt.Sprintf("%s %s", components.StatusIconPadded(p.Status), title)
	}
	return lines
}

// mrLines renders a short list of the most recent merge requests.
func (v *OverviewView) mrLines() []string {
	const maxRows = 8
	n := len(v.mrs)
	if n > maxRows {
		n = maxRows
	}
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		mr := v.mrs[i]
		pipeIcon := ""
		if mr.Pipeline != nil {
			pipeIcon = " " + components.StatusIcon(mr.Pipeline.Status)
		}
		lines[i] = fmt.Sprintf("!%d %s%s", mr.IID, mr.Title, pipeIcon)
	}
	return lines
}

// issueLines renders a short list of the most recent issues.
func (v *OverviewView) issueLines() []string {
	const maxRows = 8
	n := len(v.issues)
	if n > maxRows {
		n = maxRows
	}
	lines := make([]string, n)
	for i := 0; i < n; i++ {
		issue := v.issues[i]
		lines[i] = fmt.Sprintf("#%d %s", issue.IID, issue.Title)
	}
	return lines
}

// commitStatus maps a commit to its CI status by matching a pipeline SHA that
// starts with the commit's ShortID (or vice versa). Returns "" if none.
func commitStatus(shortID string, pipelines []gitlab.Pipeline) string {
	for _, p := range pipelines {
		if shortID != "" && strings.HasPrefix(p.SHA, shortID) {
			return p.Status
		}
	}
	return ""
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View. Overview has no view-specific key actions.
func (v *OverviewView) KeyHints() []KeyHint { return nil }

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *OverviewView) ref() string {
	if v.ctx == nil || v.ctx.Branch == nil {
		return ""
	}
	return v.ctx.Branch.Name
}

func (v *OverviewView) loadCommits() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	ref := v.ref()
	return func() tea.Msg {
		commits, err := client.ListCommits(projectID, ref)
		return CommitsLoadedMsg{Commits: commits, Err: err}
	}
}

func (v *OverviewView) loadPipelines() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	if v.ctx.Branch != nil {
		ref := v.ctx.Branch.Name
		return func() tea.Msg {
			pipelines, err := client.ListPipelinesByRef(projectID, ref)
			return PipelinesLoadedMsg{Pipelines: pipelines, Err: err}
		}
	}
	return func() tea.Msg {
		pipelines, err := client.ListPipelines(projectID)
		return PipelinesLoadedMsg{Pipelines: pipelines, Err: err}
	}
}

func (v *OverviewView) loadMRs() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	return func() tea.Msg {
		mrs, err := client.ListMergeRequests(projectID)
		return MRsLoadedMsg{MRs: mrs, Err: err}
	}
}

func (v *OverviewView) loadIssues() tea.Cmd {
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
