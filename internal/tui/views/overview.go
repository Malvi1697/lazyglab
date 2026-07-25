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

// OverviewView is a dashboard summarizing recent activity across commits,
// pipelines, merge requests, and issues. The recent-commits list is navigable;
// the three summary boxes below it are read-only.
type OverviewView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	commits   []gitlab.Commit
	pipelines []gitlab.Pipeline
	mrs       []gitlab.MergeRequest
	issues    []gitlab.Issue

	cursor int // into commits
	scroll int // first visible row, kept across frames

	// detail is the in-place commit page, opened with Enter — the same one the
	// Commits view uses, so drilling in never moves you to another tab.
	detail commitDetail
}

// NewOverviewView creates an OverviewView bound to the shared session context.
func NewOverviewView(ctx *Context) *OverviewView {
	return &OverviewView{ctx: ctx, detail: newCommitDetail(ctx)}
}

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

// Update implements View: navigates the recent-commits list and opens the
// selected commit in a browser. View switching stays with the shell.
func (v *OverviewView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)

	case CommitsLoadedMsg:
		if msg.Err == nil {
			v.commits = msg.Commits
			v.clampCursor()
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

	// Everything else may belong to the commit page or its jobs panel.
	cmd, _ := v.detail.update(msg)
	return cmd
}

// handleKey navigates the recent-commits list. The same keys work here as in
// every other view, so j/k are never dead — this is the default view.
func (v *OverviewView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring commit belongs to the list's owner, since
		// the page itself does not know what comes next — but not while a diff is
		// open, where the arrows step between that commit's files.
		if step, ok := commitStep(key); ok && !v.detail.readingDiff() {
			return v.stepCommit(step)
		}
		return v.detail.handleKey(key, v.height)
	}

	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.commits), listRows(v.height))
		return nil
	}
	if key == keyOpenBrowse {
		return v.openCommitInBrowser()
	}
	if key == keyEnter {
		return v.detail.openAt(v.selectedCommit(), v.cursor, len(v.commits))
	}
	if key == keyCopy {
		return v.copyHash()
	}
	return nil
}

// copyHash copies the selected commit's full SHA to the clipboard, since the
// list shows the author rather than the hash.
func (v *OverviewView) copyHash() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	c := v.commits[v.cursor]
	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	return tea.Batch(
		copyToClipboard(sha),
		func() tea.Msg { return StatusMsg{Text: "Copied " + c.ShortID + " to the clipboard"} },
	)
}

// stepCommit moves to the neighbouring commit, keeping the page open.
func (v *OverviewView) stepCommit(step int) tea.Cmd {
	next := v.cursor + step
	if next < 0 || next >= len(v.commits) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.openAt(v.selectedCommit(), v.cursor, len(v.commits))
}

// selectedCommit returns the highlighted commit, or nil.
func (v *OverviewView) selectedCommit() *gitlab.Commit {
	if v.cursor < 0 || v.cursor >= len(v.commits) {
		return nil
	}
	return &v.commits[v.cursor]
}

func (v *OverviewView) clampCursor() {
	if v.cursor >= len(v.commits) {
		v.cursor = len(v.commits) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

// openCommitInBrowser opens the selected commit's page on the GitLab host.
func (v *OverviewView) openCommitInBrowser() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	cmd := openBrowserCmd(v.commits[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		if err := cmd.Start(); err != nil {
			return StatusMsg{Text: "Could not open browser: " + err.Error(), IsErr: true}
		}
		return nil
	}
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: top half is a recent-commits list, bottom half is
// three side-by-side summary boxes for pipelines, merge requests, and issues.
func (v *OverviewView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a commit: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	// A blank row between the commit list and the summaries below it. Without a
	// frame to do the separating, the two halves otherwise run into each other.
	const gap = 1

	// The summaries take the height their content needs — they hold at most
	// summaryRows entries — and the commit list gets everything left over. Split
	// evenly, three short lists were stretched down half a tall terminal while
	// the list you actually read was cut off.
	bottomHeight := 1 + maxInt(
		len(v.pipelineLines()),
		len(v.mrLines()),
		len(v.issueLines()),
	)
	if bottomHeight < 4 {
		bottomHeight = 4 // a heading and room to say "nothing here"
	}
	if maxBottom := (height - gap) / 2; bottomHeight > maxBottom {
		bottomHeight = maxBottom
	}

	topHeight := height - gap - bottomHeight
	if topHeight < 1 {
		topHeight = 1
	}
	if bottomHeight < 1 {
		bottomHeight = 1
	}

	top := renderListBox(width, topHeight, v.commitsTitle(), v.commitItems(), v.cursor, &v.scroll)

	colWidth := width / 3
	lastColWidth := width - colWidth*2

	pipelines := components.RenderPanel(v.pipelinesTitle(), v.pipelineLines(), colWidth-3, bottomHeight, false)
	mrs := components.RenderPanel(v.mrsTitle(), v.mrLines(), colWidth-3, bottomHeight, false)
	issues := components.RenderPanel(v.issuesTitle(), v.issueLines(), lastColWidth-3, bottomHeight, false)

	rule := components.VRule(bottomHeight)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, pipelines, " ", rule, " ", mrs, " ", rule, " ", issues)

	return lipgloss.JoinVertical(lipgloss.Left, top, "", bottom)
}

// maxInt returns the largest of its arguments.
func maxInt(ns ...int) int {
	out := 0
	for _, n := range ns {
		if n > out {
			out = n
		}
	}
	return out
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
		items[i] = commitRow(util.CommitTime(c.CreatedAt),
			commitStatusIcon(commitStatus(c.ShortID, v.pipelines)),
			c.AuthorName, c.Title)
	}
	return items
}

// pipelineLines renders a short list of the most recent pipelines.
func (v *OverviewView) pipelineLines() []string {
	const maxRows = summaryRows
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
	const maxRows = summaryRows
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
	const maxRows = summaryRows
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

// summaryRows caps each of Overview's three summary lists, which also decides how
// tall the bottom row of the dashboard is.
const summaryRows = 8

// commitStatusIcon renders a commit's CI state, or a faint dot when no pipeline
// ran for it — a blank would break the column that the eye follows down.
func commitStatusIcon(status string) string {
	if status == "" {
		return components.FaintStyle.Render("·") + " "
	}
	return components.StatusIconPadded(status)
}

// commitStatus maps a commit to its CI status by matching a pipeline SHA that
// starts with the commit's ShortID. A success with warnings reports the warning
// pseudo-status, so a failed allowed-to-fail job is not hidden behind a green
// tick — GitLab flags those in its own commit list. Returns "" if none.
func commitStatus(shortID string, pipelines []gitlab.Pipeline) string {
	for _, p := range pipelines {
		if shortID != "" && strings.HasPrefix(p.SHA, shortID) {
			if p.HasWarnings {
				return components.StatusWarning
			}
			return p.Status
		}
	}
	return ""
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *OverviewView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	return []KeyHint{
		{Key: "Enter", Desc: "Pipeline"},
		{Key: "y", Desc: "Copy SHA"},
		{Key: "o", Desc: "Open commit"},
	}
}

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
