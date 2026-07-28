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

	cursor int // into the visible (searched) commits
	scroll int // first visible row, kept across frames

	search listSearch

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
	// With a commit page open, the page is what is on screen — refreshing the lists
	// behind it left its pipeline frozen at whatever it said when you opened it.
	if v.detail.active {
		if v.detail.readingBody() {
			return nil
		}
		return v.detail.reload()
	}
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

	case tea.PasteMsg:
		if !v.detail.active {
			v.search.paste(msg.Content, &v.cursor)
		}
		return nil
	}

	// Everything else may belong to the commit page or its jobs panel.
	return v.detail.update(msg)
}

// handleKey navigates the recent-commits list. The same keys work here as in
// every other view, so j/k are never dead — this is the default view.
func (v *OverviewView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring commit belongs to the list's owner, since
		// the page itself does not know what comes next — but not while a diff or a
		// job log is open, where the arrows belong to what you are reading.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepCommit(step)
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
	if key == keyOpenBrowse {
		return v.openCommitInBrowser()
	}
	if key == keyEnter {
		return v.detail.openAt(v.selectedCommit(), v.cursor, len(v.visible()))
	}
	if key == keyCopy {
		return v.copyHash()
	}
	if key == keyCopyLink {
		if c := v.selectedCommit(); c != nil {
			return copyLink(c.ShortID, c.WebURL)
		}
		return nil
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *OverviewView) CapturingText() bool { return !v.detail.active && v.search.capturing() }

// visible is the commits matching the search; the cursor indexes it.
func (v *OverviewView) visible() []gitlab.Commit {
	return filtered(v.commits, v.search.filter, func(c gitlab.Commit) string {
		return c.Title + " " + c.AuthorName + " " + c.ShortID
	})
}

// copyHash copies the selected commit's full SHA to the clipboard, since the
// list shows the author rather than the hash.
func (v *OverviewView) copyHash() tea.Cmd {
	selected := v.selectedCommit()
	if selected == nil {
		return nil
	}
	c := *selected
	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	return tea.Batch(
		copyToClipboard(sha),
		func() tea.Msg { return StatusMsg{Text: "Copied " + c.ShortID + " to the clipboard"} },
	)
}

// stepCommit moves to the neighbouring commit, keeping the page open. It steps
// within the search results when one is applied: the page was opened from that
// list, so those are the commits you are working through.
func (v *OverviewView) stepCommit(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selectedCommit(), v.cursor, len(visible))
}

// selectedCommit returns the highlighted commit, or nil.
func (v *OverviewView) selectedCommit() *gitlab.Commit {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *OverviewView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}

// openCommitInBrowser opens the selected commit's page on the GitLab host.
func (v *OverviewView) openCommitInBrowser() tea.Cmd {
	c := v.selectedCommit()
	if c == nil {
		return nil
	}
	cmd := openBrowserCmd(c.WebURL)
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
	bottomHeight := 1 + max(
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

	visible := v.visible()
	top := renderRowsBox(width, topHeight,
		v.search.title("Recent Commits", len(visible), len(v.commits)),
		len(visible), func(i int) string { return v.commitRow(visible[i]) },
		v.cursor, &v.scroll)

	colWidth := width / 3
	lastColWidth := width - colWidth*2

	pipelines := components.RenderPanel(v.pipelinesTitle(), v.pipelineLines(), colWidth-3, bottomHeight, false)
	mrs := components.RenderPanel(v.mrsTitle(), v.mrLines(), colWidth-3, bottomHeight, false)
	issues := components.RenderPanel(v.issuesTitle(), v.issueLines(), lastColWidth-3, bottomHeight, false)

	rule := components.VRule(bottomHeight)
	bottom := lipgloss.JoinHorizontal(lipgloss.Top, pipelines, " ", rule, " ", mrs, " ", rule, " ", issues)

	return lipgloss.JoinVertical(lipgloss.Left, top, "", bottom)
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

// commitRow renders one recent-commits row, mapping the commit's CI status from
// the loaded pipelines by SHA.
func (v *OverviewView) commitRow(c gitlab.Commit) string {
	return commitRow(util.CommitTime(c.CreatedAt),
		commitStatusIcon(commitStatus(c.ShortID, v.pipelines)),
		c.AuthorName, c.Title)
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
		lines[i] = fmt.Sprintf("%s %s", components.StatusIconPadded(p.Status), styleCommitTitle(title))
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
		lines[i] = refAndTitle(fmt.Sprintf("!%d", mr.IID), mr.Title) + pipeIcon
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
		lines[i] = refAndTitle(fmt.Sprintf("#%d", issue.IID), issue.Title)
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
		{Key: "Enter", Desc: "Commit page"},
		{Key: "y/Y", Desc: "Copy SHA/link"},
		{Key: "o", Desc: "Open commit"},
		v.search.hint(),
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
