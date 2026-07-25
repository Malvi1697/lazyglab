package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// CommitsView is the self-contained cockpit view for commits.
type CommitsView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	commits []gitlab.Commit
	cursor  int
	scroll  int // first visible row, kept across frames

	// Commit detail drill-down: the commit's full message and the pipelines run
	// for it. GitLab builds refs, not commits, so there may be none.
	viewingCommit   bool
	detailCommit    *gitlab.Commit
	detailPipelines []gitlab.Pipeline
	detailRefs      []gitlab.CommitRef
	detailMRs       []gitlab.MergeRequest
	detailSHA       string // the request in flight, to ignore stale replies
	detailLoading   bool
	pendingSHA      string // commit to focus once the list arrives (from Overview)

	status string
}

// NewCommitsView creates a CommitsView bound to the shared session context.
func NewCommitsView(ctx *Context) *CommitsView { return &CommitsView{ctx: ctx} }

// Title implements View.
func (v *CommitsView) Title() string { return "Commits" }

// Focus implements View: loads commits for the active project/branch.
func (v *CommitsView) Focus() tea.Cmd { return v.load() }

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *CommitsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case CommitDetailLoadedMsg:
		if msg.SHA != v.detailSHA {
			return nil // a stale reply for a commit we have moved off
		}
		v.detailLoading = false
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading commit: %v", msg.Err)
			return nil
		}
		if msg.Commit != nil {
			v.detailCommit = msg.Commit
		}
		v.detailPipelines = msg.Pipelines
		v.detailRefs = msg.Refs
		v.detailMRs = msg.MRs
		return nil

	case ShowCommitMsg:
		// Drilling in from Overview: focus the commit, then open its detail.
		v.pendingSHA = msg.ShortSHA
		if v.focusPending() {
			return v.openDetail()
		}
		return nil

	case CommitsLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading commits: %v", msg.Err)
			return nil
		}
		v.commits = msg.Commits
		v.clampCursor()
		v.status = fmt.Sprintf("Loaded %d commits", len(msg.Commits))
		return nil

	case StatusMsg:
		v.status = msg.Text
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

func (v *CommitsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.viewingCommit {
		return v.handleDetailKey(key)
	}

	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.commits), listRows(v.height))
		return nil
	}

	if key == keyOpenBrowse {
		return v.openCommitInBrowser()
	}
	if key == keyEnter {
		return v.openDetail()
	}
	if key == keyCopy {
		return v.copyHash()
	}
	return nil
}

// openDetail drills into the selected commit: its full message plus the pipelines
// GitLab ran for it. Unlike jumping straight to the Pipelines view, this stays
// put and is meaningful even when no pipeline ever ran for the commit.
func (v *CommitsView) openDetail() tea.Cmd {
	c := v.selected()
	if c == nil {
		return nil
	}
	v.viewingCommit = true
	v.detailCommit = c
	v.detailPipelines = nil
	v.detailRefs = nil
	v.detailMRs = nil
	return v.loadDetail(c)
}

// handleDetailKey drives the commit detail. Esc goes back to the list.
func (v *CommitsView) handleDetailKey(key string) tea.Cmd {
	switch key {
	case keyEscape:
		v.viewingCommit = false
		v.detailPipelines = nil
		v.detailRefs = nil
		v.detailMRs = nil
		v.detailSHA = ""
		return nil
	case keyCopy:
		return v.copyHash()
	case keyOpenBrowse:
		return v.openCommitInBrowser()
	case keyEnter:
		// Drill on into the Pipelines view, where jobs and logs live.
		return v.showPipeline()
	case keyRetry:
		return v.retryCommitPipeline()
	case keyRun:
		return v.runPipelineOnRef()
	}
	return nil
}

// loadDetail fetches the commit's full message and its pipelines.
func (v *CommitsView) loadDetail(c *gitlab.Commit) tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	v.detailSHA = sha
	v.detailLoading = true

	return func() tea.Msg {
		commit, err := client.GetCommit(projectID, sha)
		if err != nil {
			return CommitDetailLoadedMsg{SHA: sha, Err: err}
		}
		// The client resolves "passed with warnings" for these, which the list
		// endpoint alone cannot report.
		pipelines, err := client.ListPipelinesBySHA(projectID, sha)
		if err != nil {
			return CommitDetailLoadedMsg{SHA: sha, Commit: commit, Err: err}
		}

		// Branches, tags and merge requests are what GitLab's commit page shows
		// beside the message; a failure here must not lose the rest.
		refs, _ := client.GetCommitRefs(projectID, sha)
		mrs, _ := client.ListCommitMergeRequests(projectID, sha)

		return CommitDetailLoadedMsg{
			SHA: sha, Commit: commit, Pipelines: pipelines, Refs: refs, MRs: mrs,
		}
	}
}

// commitPipeline returns the most recent pipeline for the shown commit, or nil.
func (v *CommitsView) commitPipeline() *gitlab.Pipeline {
	if len(v.detailPipelines) == 0 {
		return nil
	}
	return &v.detailPipelines[0]
}

// retryCommitPipeline retries the commit's pipeline, if it has one.
func (v *CommitsView) retryCommitPipeline() tea.Cmd {
	p := v.commitPipeline()
	if p == nil {
		return func() tea.Msg {
			return StatusMsg{Text: "No pipeline to retry for this commit", IsErr: true}
		}
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := p.ID
	return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", pipelineID), func() tea.Msg {
		if err := client.RetryPipeline(projectID, pipelineID); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Retried pipeline #%d", pipelineID)}
	})
}

// runPipelineOnRef runs a new pipeline on the active ref.
//
// GitLab creates pipelines for a ref, never for an arbitrary commit, so this
// builds the ref's current head — which is only this commit if it happens to be
// the tip. The confirmation says so instead of implying otherwise.
func (v *CommitsView) runPipelineOnRef() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	ref := v.ref()
	if ref == "" {
		ref = v.ctx.Project.DefaultBranch
	}
	if ref == "" {
		return func() tea.Msg { return StatusMsg{Text: "No branch to run a pipeline on", IsErr: true} }
	}

	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	prompt := fmt.Sprintf("Run new pipeline on %s? (builds the branch head, not this commit)", ref)
	return confirmCmd(prompt, func() tea.Msg {
		p, err := client.RunPipeline(projectID, ref)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Run failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Started pipeline #%d on %s", p.ID, ref)}
	})
}

// focusPending moves the cursor to pendingSHA and reports whether it was found.
func (v *CommitsView) focusPending() bool {
	if v.pendingSHA == "" {
		return false
	}
	for i, c := range v.commits {
		if strings.HasPrefix(c.ID, v.pendingSHA) || c.ShortID == v.pendingSHA {
			v.cursor = i
			v.pendingSHA = ""
			return true
		}
	}
	return false
}

// selected returns the highlighted commit, or nil.
func (v *CommitsView) selected() *gitlab.Commit {
	if v.cursor < 0 || v.cursor >= len(v.commits) {
		return nil
	}
	return &v.commits[v.cursor]
}

// copyHash copies the selected commit's full SHA to the clipboard. The list
// shows the author rather than the hash, so this is how the hash is obtained.
func (v *CommitsView) copyHash() tea.Cmd {
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

// showPipeline asks the shell to open the selected commit's pipeline.
func (v *CommitsView) showPipeline() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	sha := v.commits[v.cursor].ShortID
	return func() tea.Msg { return ShowCommitPipelineMsg{ShortSHA: sha} }
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail panel on the right.
func (v *CommitsView) Body(width, height int) string {
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

	left := renderListBox(leftWidth, height, "Commits", v.commitItems(), v.cursor, &v.scroll)

	detail := v.commitDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	detailTitle := "Commit"
	if v.viewingCommit {
		detailTitle = "Commit detail"
	}
	right := components.RenderBox(detailTitle, splitLines(detail), rightWidth, height, components.ColorSecondary, components.ColorPrimary)

	return joinH(left, right)
}

// commitItems renders the commit list rows.
func (v *CommitsView) commitItems() []string {
	items := make([]string, len(v.commits))
	for i, c := range v.commits {
		icon := "  "
		if c.Status != "" {
			icon = components.StatusIconPadded(c.Status)
		}
		items[i] = fmt.Sprintf("%s %s %s  %s",
			util.TimeAgoShort(c.CreatedAt),
			icon,
			components.PadRight(components.Truncate(c.AuthorName, authorWidth), authorWidth),
			c.Title,
		)
	}
	return items
}

func (v *CommitsView) commitDetail() string {
	if len(v.commits) == 0 {
		return "No commits"
	}
	c := v.selected()
	if c == nil {
		return ""
	}
	if v.viewingCommit {
		return v.commitDetailFull()
	}

	return fmt.Sprintf("%s\n\n%s\n\nAuthor: %s\n%s\n\n%s\n\n%s",
		components.TitleStyle.Render(c.ShortID),
		c.Title,
		c.AuthorName,
		util.TimeAgo(c.CreatedAt),
		components.HelpDescStyle.Render(c.WebURL),
		components.HelpDescStyle.Render("Enter: commit detail & pipelines"),
	)
}

// commitDetailFull renders the drilled-in commit the way GitLab's commit page
// does: the message, then what the commit belongs to (parent, branches, merge
// requests) and the pipelines it triggered.
func (v *CommitsView) commitDetailFull() string {
	c := v.detailCommit
	if c == nil {
		c = v.selected()
	}
	if c == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(components.TitleStyle.Render(c.Title) + "\n\n")

	message := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Message), c.Title))
	if message != "" {
		b.WriteString(message + "\n\n")
	}

	b.WriteString(components.HelpDescStyle.Render("commit ") + c.ShortID +
		components.HelpDescStyle.Render("   authored by ") + c.AuthorName +
		components.HelpDescStyle.Render("   "+util.TimeAgo(c.CreatedAt)) + "\n")

	if len(c.ParentIDs) > 0 {
		parents := make([]string, 0, len(c.ParentIDs))
		for _, id := range c.ParentIDs {
			parents = append(parents, shortSHA(id))
		}
		b.WriteString(components.HelpDescStyle.Render("parent ") + strings.Join(parents, ", ") + "\n")
	}

	b.WriteString(v.detailRefsLine())
	b.WriteString(v.detailMRLines())
	b.WriteString("\n" + v.detailPipelineLines())
	b.WriteString("\n" + components.HelpDescStyle.Render(c.WebURL))
	return b.String()
}

// detailRefsLine lists the branches and tags containing the commit.
func (v *CommitsView) detailRefsLine() string {
	if v.detailLoading && len(v.detailRefs) == 0 {
		return ""
	}
	if len(v.detailRefs) == 0 {
		return components.HelpDescStyle.Render("no branches or tags contain it") + "\n"
	}

	var branches, tags []string
	for _, r := range v.detailRefs {
		if r.Type == "tag" {
			tags = append(tags, r.Name)
		} else {
			branches = append(branches, r.Name)
		}
	}
	out := ""
	if len(branches) > 0 {
		out += components.HelpDescStyle.Render("branches ") + strings.Join(branches, ", ") + "\n"
	}
	if len(tags) > 0 {
		out += components.HelpDescStyle.Render("tags ") + strings.Join(tags, ", ") + "\n"
	}
	return out
}

// detailMRLines lists the merge requests the commit belongs to.
func (v *CommitsView) detailMRLines() string {
	if len(v.detailMRs) == 0 {
		return ""
	}
	out := ""
	for _, mr := range v.detailMRs {
		out += components.HelpDescStyle.Render("merge request ") +
			fmt.Sprintf("!%d %s", mr.IID, components.Truncate(mr.Title, 48)) + "\n"
	}
	return out
}

// detailPipelineLines renders the pipelines run for the commit, distinguishing a
// success with warnings from a plain success — that is the whole reason the
// detail asks GitLab for each pipeline individually.
func (v *CommitsView) detailPipelineLines() string {
	out := components.TitleStyle.Render("Pipelines") + "\n"
	switch {
	case v.detailLoading:
		return out + components.HelpDescStyle.Render("Loading…") + "\n"
	case len(v.detailPipelines) == 0:
		// Nothing ran for this commit — say so plainly, and note that a pipeline
		// can only be started for a branch, never for a past commit.
		return out +
			components.HelpDescStyle.Render("No pipeline ran for this commit.") + "\n" +
			components.HelpDescStyle.Render("p runs one on the branch head instead.") + "\n"
	}

	for _, p := range v.detailPipelines {
		status := p.Status
		if p.HasWarnings {
			status = components.StatusWarning
		}
		label := p.StatusLabel
		if label == "" {
			label = p.Status
		}
		out += fmt.Sprintf("%s #%d  %s  %s  %s\n",
			components.StatusIconPadded(status), p.ID, label, p.Ref,
			components.HelpDescStyle.Render(util.TimeAgo(p.CreatedAt)))
	}
	return out
}

// shortSHA abbreviates a full SHA the way GitLab displays it.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *CommitsView) KeyHints() []KeyHint {
	if v.viewingCommit {
		return []KeyHint{
			{"Enter", "Pipelines view"},
			{"R", "Retry"},
			{"p", "Run on branch"},
			{"y", "Copy SHA"},
			{"Esc", "Back"},
		}
	}
	return []KeyHint{
		{"Enter", "Commit detail"},
		{"y", "Copy SHA"},
		{"o", "Open"},
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

// ref is the active branch, empty when the project default is in use.
func (v *CommitsView) ref() string {
	if v.ctx == nil || v.ctx.Branch == nil {
		return ""
	}
	return v.ctx.Branch.Name
}

func (v *CommitsView) load() tea.Cmd {
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

func (v *CommitsView) openCommitInBrowser() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	cmd := openBrowserCmd(v.commits[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *CommitsView) clampCursor() {
	n := len(v.commits)
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}
