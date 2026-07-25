package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// commitDetail is the full-screen commit page: the message, what the commit
// belongs to, and the pipelines it triggered with their jobs — the same things
// GitLab's own commit page shows.
//
// It is a drill-down opened in place by whichever view holds a commit list, so
// pressing Enter never moves you to another tab.
type commitDetail struct {
	ctx *Context

	active bool
	commit *gitlab.Commit

	pipelines []gitlab.Pipeline
	refs      []gitlab.CommitRef
	mrs       []gitlab.MergeRequest
	jobs      []gitlab.Job

	sha     string // request in flight, to ignore stale replies
	loading bool
	scroll  int
}

// open drills into a commit and starts loading everything about it.
func (d *commitDetail) open(c *gitlab.Commit) tea.Cmd {
	if c == nil {
		return nil
	}
	d.active = true
	d.commit = c
	d.pipelines, d.refs, d.mrs, d.jobs = nil, nil, nil, nil
	d.scroll = 0
	return d.load(c)
}

// close returns to the list.
func (d *commitDetail) close() {
	d.active = false
	d.pipelines, d.refs, d.mrs, d.jobs = nil, nil, nil, nil
	d.sha = ""
	d.scroll = 0
}

// load fetches the commit, its pipelines (with their jobs), the refs containing
// it and the merge requests it belongs to, in one command.
func (d *commitDetail) load(c *gitlab.Commit) tea.Cmd {
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client := d.ctx.Client
	projectID := d.ctx.Project.ID
	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	d.sha = sha
	d.loading = true

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

		// The newest pipeline's jobs are what GitLab shows as status circles.
		var jobs []gitlab.Job
		if len(pipelines) > 0 {
			jobs, _ = client.ListPipelineJobs(projectID, pipelines[0].ID)
		}

		// Branches, tags and merge requests round out the page; a failure in any
		// of them must not lose the rest.
		refs, _ := client.GetCommitRefs(projectID, sha)
		mrs, _ := client.ListCommitMergeRequests(projectID, sha)

		return CommitDetailLoadedMsg{
			SHA: sha, Commit: commit, Pipelines: pipelines, Refs: refs, MRs: mrs, Jobs: jobs,
		}
	}
}

// update absorbs the loaded detail, ignoring replies for a commit left behind.
func (d *commitDetail) update(msg CommitDetailLoadedMsg) string {
	if msg.SHA != d.sha {
		return ""
	}
	d.loading = false
	if msg.Err != nil {
		return fmt.Sprintf("Error loading commit: %v", msg.Err)
	}
	if msg.Commit != nil {
		d.commit = msg.Commit
	}
	d.pipelines, d.refs, d.mrs, d.jobs = msg.Pipelines, msg.Refs, msg.MRs, msg.Jobs
	return ""
}

// handleKey drives the detail. Esc goes back to the list it was opened from.
func (d *commitDetail) handleKey(key string, height int) tea.Cmd {
	if act := components.NavFor(key); act != components.NavNone {
		// The page scrolls; there is nothing to select on it.
		d.scroll = components.ApplyNav(act, d.scroll, len(d.lines(0)), listRows(height))
		return nil
	}

	switch key {
	case keyEscape:
		d.close()
		return nil
	case keyCopy:
		return d.copySHA()
	case keyOpenBrowse:
		if d.commit == nil {
			return nil
		}
		if cmd := openBrowserCmd(d.commit.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
		return nil
	case keyEnter:
		// Jobs and logs live in the Pipelines view; hand off to it.
		return d.showPipeline()
	case keyRetry:
		return d.retryPipeline()
	case keyRun:
		return d.runOnRef()
	}
	return nil
}

// copySHA copies the commit's full hash.
func (d *commitDetail) copySHA() tea.Cmd {
	if d.commit == nil {
		return nil
	}
	sha := d.commit.ID
	if sha == "" {
		sha = d.commit.ShortID
	}
	short := d.commit.ShortID
	return tea.Batch(
		copyToClipboard(sha),
		func() tea.Msg { return StatusMsg{Text: "Copied " + short + " to the clipboard"} },
	)
}

// showPipeline asks the shell for the Pipelines view, positioned on this commit.
func (d *commitDetail) showPipeline() tea.Cmd {
	if d.commit == nil {
		return nil
	}
	if len(d.pipelines) == 0 {
		return func() tea.Msg {
			return StatusMsg{Text: "No pipeline ran for this commit", IsErr: true}
		}
	}
	sha := d.commit.ShortID
	return func() tea.Msg { return ShowCommitPipelineMsg{ShortSHA: sha} }
}

// pipeline returns the newest pipeline for the commit, or nil.
func (d *commitDetail) pipeline() *gitlab.Pipeline {
	if len(d.pipelines) == 0 {
		return nil
	}
	return &d.pipelines[0]
}

// retryPipeline retries the commit's pipeline, if it has one.
func (d *commitDetail) retryPipeline() tea.Cmd {
	p := d.pipeline()
	if p == nil {
		return func() tea.Msg {
			return StatusMsg{Text: "No pipeline to retry for this commit", IsErr: true}
		}
	}
	client := d.ctx.Client
	projectID := d.ctx.Project.ID
	id := p.ID
	return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", id), func() tea.Msg {
		if err := client.RetryPipeline(projectID, id); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Retried pipeline #%d", id)}
	})
}

// runOnRef starts a pipeline on the active branch.
//
// GitLab creates pipelines for a ref, never for an arbitrary commit, so this
// builds the branch's current head — only this commit if it happens to be the
// tip. The confirmation says so rather than implying otherwise.
func (d *commitDetail) runOnRef() tea.Cmd {
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	ref := ""
	if d.ctx.Branch != nil {
		ref = d.ctx.Branch.Name
	}
	if ref == "" {
		ref = d.ctx.Project.DefaultBranch
	}
	if ref == "" {
		return func() tea.Msg { return StatusMsg{Text: "No branch to run a pipeline on", IsErr: true} }
	}

	client := d.ctx.Client
	projectID := d.ctx.Project.ID
	prompt := fmt.Sprintf("Run new pipeline on %s? (builds the branch head, not this commit)", ref)
	return confirmCmd(prompt, func() tea.Msg {
		p, err := client.RunPipeline(projectID, ref)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Run failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Started pipeline #%d on %s", p.ID, ref)}
	})
}

// keyHints are the detail's footer hints.
func (d *commitDetail) keyHints() []KeyHint {
	return []KeyHint{
		{"Esc", "Back"},
		{"Enter", "Pipelines view"},
		{"R", "Retry"},
		{"p", "Run on branch"},
		{"y", "Copy SHA"},
		{"o", "Open"},
	}
}

// ============================================================================
// Rendering
// ============================================================================

// body renders the detail as the view's whole body.
func (d *commitDetail) body(width, height int) string {
	lines := d.lines(width - 4)
	rows := height - 2
	if rows < 1 {
		rows = 1
	}

	// The page is scrolled directly rather than followed around a cursor, so the
	// offset only needs clamping — ScrollOffset would drag it back to keep a
	// non-existent cursor in view.
	if d.scroll > len(lines)-rows {
		d.scroll = max(0, len(lines)-rows)
	}
	if d.scroll < 0 {
		d.scroll = 0
	}

	end := min(d.scroll+rows, len(lines))
	visible := lines[d.scroll:end]

	title := "Commit"
	if d.commit != nil {
		title = "Commit " + d.commit.ShortID
	}
	if len(lines) > rows {
		title = fmt.Sprintf("%s  (%d-%d of %d)", title, d.scroll+1, end, len(lines))
	}
	return components.RenderBox(title, visible, width, height,
		components.ColorPrimary, components.ColorPrimary)
}

// lines builds the page. width is the available text width; 0 means "don't wrap".
func (d *commitDetail) lines(width int) []string {
	c := d.commit
	if c == nil {
		return []string{"No commit"}
	}

	var out []string
	add := func(s string) { out = append(out, s) }
	wrap := func(s string) {
		if width <= 0 {
			add(s)
			return
		}
		out = append(out, components.WrapLine(s, width)...)
	}

	wrap(components.TitleStyle.Render(c.Title))
	add("")

	// The body of the message, without repeating the subject.
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Message), c.Title))
	if body != "" {
		for _, line := range strings.Split(body, "\n") {
			wrap(line)
		}
		add("")
	}

	add(components.HelpDescStyle.Render("commit  ") + c.ShortID +
		components.HelpDescStyle.Render("   authored by ") + c.AuthorName)
	if len(c.ParentIDs) > 0 {
		parents := make([]string, 0, len(c.ParentIDs))
		for _, id := range c.ParentIDs {
			parents = append(parents, shortSHA(id))
		}
		add(components.HelpDescStyle.Render("parent  ") + strings.Join(parents, ", "))
	}
	out = append(out, d.refLines()...)
	out = append(out, d.mrLines()...)
	add("")
	out = append(out, d.pipelineLines()...)
	add("")
	wrap(components.HelpDescStyle.Render(c.WebURL))
	return out
}

// refLines lists the branches and tags containing the commit.
func (d *commitDetail) refLines() []string {
	if d.loading && len(d.refs) == 0 {
		return nil
	}
	if len(d.refs) == 0 {
		return []string{components.HelpDescStyle.Render("refs    ") +
			components.HelpDescStyle.Render("no branches or tags contain it")}
	}

	var branches, tags []string
	for _, r := range d.refs {
		if r.Type == "tag" {
			tags = append(tags, r.Name)
		} else {
			branches = append(branches, r.Name)
		}
	}
	var out []string
	if len(branches) > 0 {
		out = append(out, components.HelpDescStyle.Render("branch  ")+strings.Join(branches, ", "))
	}
	if len(tags) > 0 {
		out = append(out, components.HelpDescStyle.Render("tags    ")+strings.Join(tags, ", "))
	}
	return out
}

// mrLines lists the merge requests the commit belongs to.
func (d *commitDetail) mrLines() []string {
	var out []string
	for _, mr := range d.mrs {
		out = append(out, components.HelpDescStyle.Render("mr      ")+
			fmt.Sprintf("!%d %s", mr.IID, mr.Title))
	}
	return out
}

// pipelineLines renders the commit's pipelines and the newest one's jobs grouped
// by stage — GitLab's row of status circles, spelled out.
func (d *commitDetail) pipelineLines() []string {
	out := []string{components.TitleStyle.Render("Pipelines")}

	switch {
	case d.loading && len(d.pipelines) == 0:
		return append(out, components.HelpDescStyle.Render("Loading…"))
	case len(d.pipelines) == 0:
		// Nothing ran for this commit; a pipeline can only be started for a
		// branch, so say what p would actually do.
		return append(out,
			components.HelpDescStyle.Render("No pipeline ran for this commit."),
			components.HelpDescStyle.Render("p runs one on the branch head instead."))
	}

	for _, p := range d.pipelines {
		status := p.Status
		if p.HasWarnings {
			status = components.StatusWarning
		}
		label := p.StatusLabel
		if label == "" {
			label = p.Status
		}
		out = append(out, fmt.Sprintf("%s #%d  %s  %s",
			components.StatusIconPadded(status), p.ID, label,
			components.HelpDescStyle.Render(p.Ref)))
	}

	if len(d.jobs) == 0 {
		return out
	}

	out = append(out, "", components.TitleStyle.Render("Jobs"))
	stage := ""
	for _, j := range d.jobs {
		if j.Stage != stage {
			stage = j.Stage
			out = append(out, components.HelpDescStyle.Render("  "+stage))
		}
		out = append(out, fmt.Sprintf("    %s %s  %s",
			components.StatusIconPadded(j.Status), j.Name,
			components.HelpDescStyle.Render(jobDuration(j))))
	}
	return out
}

// jobDuration renders a job's runtime, or its status when it has not run.
func jobDuration(j gitlab.Job) string {
	if j.Duration <= 0 {
		return j.Status
	}
	secs := int(j.Duration)
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm%ds", secs/60, secs%60)
}
