package views

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// mrDetail is the full-screen merge-request page: what it is, where it is going,
// what stands between it and being merged, its changed files and its jobs.
//
// It is the commit page's sibling — same boxes, same keys, same caching — because
// a merge request is the other thing you read a diff and a pipeline for. Both are
// built from changesBox, jobsPanel and pageFrame.
type mrDetail struct {
	ctx *Context

	active bool
	mr     *gitlab.MergeRequest

	approvals *gitlab.MRApprovals
	pipeline  *gitlab.Pipeline

	changesBox
	notesBox
	pageFrame

	// descScrollable is learned while rendering, so the footer offers j/k for the
	// description only where it would move something.
	descScrollable bool

	iid       int // the merge request on screen; replies for others are stale
	requested int // the one we have actually asked GitLab about
	loading   bool
	scroll    int

	jobs  jobsPanel
	focus pageFocus

	// pages remembers the last few merge requests fetched, so stepping back with h
	// is free. Each page is four requests.
	pages map[int]mrPage
	order []int
}

// mrPage is everything a merge request's page shows, as fetched.
type mrPage struct {
	mr        *gitlab.MergeRequest
	approvals *gitlab.MRApprovals
	pipeline  *gitlab.Pipeline
	jobs      []gitlab.Job
	diffs     []gitlab.FileDiff
	notes     []gitlab.Note
}

// mrPagesKept bounds the cache; a page holds its diffs, which can be large.
const mrPagesKept = 6

func newMRDetail(ctx *Context) mrDetail {
	return mrDetail{ctx: ctx, jobs: jobsPanel{ctx: ctx}, pages: map[int]mrPage{}}
}

// openAt drills into a merge request. Enter means "show me this one", so it
// fetches at once.
func (d *mrDetail) openAt(mr *gitlab.MergeRequest, index, total int) tea.Cmd {
	return d.showAt(mr, index, total, 0)
}

// stepAt is openAt for a step to a neighbour, which waits for the key to settle
// before asking GitLab anything — see stepSettleDelay.
func (d *mrDetail) stepAt(mr *gitlab.MergeRequest, index, total int) tea.Cmd {
	return d.showAt(mr, index, total, stepSettleDelay)
}

// mrFetchMsg asks the page to fetch a merge request, if it is still the one shown.
type mrFetchMsg struct{ iid int }

func (d *mrDetail) showAt(mr *gitlab.MergeRequest, index, total int, delay time.Duration) tea.Cmd {
	if mr == nil {
		return nil
	}
	d.active = true
	d.mr = mr
	d.placeIn(index, total)
	d.approvals, d.pipeline = nil, nil
	d.jobs.close()
	d.focus = focusPage
	d.resetFiles()
	d.resetNotes()
	d.scroll = 0
	d.iid, d.requested = mr.IID, 0

	if page, ok := d.pages[mr.IID]; ok {
		d.restore(page)
		return nil
	}

	d.loading = true
	if delay <= 0 {
		return d.load(mr.IID)
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return mrFetchMsg{iid: mr.IID} })
}

// close returns to the list.
func (d *mrDetail) close() {
	d.active = false
	d.jobs.close()
	d.mr, d.approvals, d.pipeline = nil, nil, nil
	d.focus = focusPage
	d.resetFiles()
	d.resetNotes()
	d.resetSearchState()
}

func (d *mrDetail) resetSearchState() {
	d.iid, d.requested = 0, 0
	d.scroll = 0
}

// restore puts a cached page back on screen.
func (d *mrDetail) restore(page mrPage) {
	d.loading = false
	if page.mr != nil {
		d.mr = page.mr
	}
	d.approvals, d.pipeline = page.approvals, page.pipeline
	d.setDiffs(page.diffs)
	d.setNotes(page.notes)
	if d.pipeline != nil {
		d.jobs.adopt(d.pipeline.ID, page.jobs)
	}
}

func (d *mrDetail) remember(iid int, page mrPage) {
	if iid == 0 {
		return
	}
	if d.pages == nil {
		d.pages = map[int]mrPage{}
	}
	if _, seen := d.pages[iid]; !seen {
		d.order = append(d.order, iid)
		for len(d.order) > mrPagesKept {
			delete(d.pages, d.order[0])
			d.order = d.order[1:]
		}
	}
	d.pages[iid] = page
}

func (d *mrDetail) forget(iid int) {
	delete(d.pages, iid)
	for i, v := range d.order {
		if v == iid {
			d.order = append(d.order[:i], d.order[i+1:]...)
			break
		}
	}
}

// reload refetches the merge request on screen — its pipeline, jobs, approvals,
// mergeability and discussion all move while you watch.
func (d *mrDetail) reload() tea.Cmd {
	if !d.active || d.iid == 0 {
		return nil
	}
	d.forget(d.iid)
	return d.load(d.iid)
}

// readingBody reports whether something long-form has the screen — a diff or a
// job's log — so the view hosting the page knows the arrows are not its to act on.
func (d *mrDetail) readingBody() bool {
	return d.reading || d.threadOpen || d.jobs.showingTrace()
}

// ============================================================================
// Loading
// ============================================================================

// load fetches the merge request, its approvals, its pipeline's jobs and its
// changed files in one command.
//
// The approvals and the pipeline are extras: an instance without approval rules,
// or a merge request nothing has built, must still show the page.
func (d *mrDetail) load(iid int) tea.Cmd {
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client := d.ctx.Client
	projectID := d.ctx.Project.ID
	d.requested = iid
	d.loading = true

	return func() tea.Msg {
		mr, err := client.GetMergeRequest(projectID, iid)
		if err != nil {
			return MRDetailLoadedMsg{IID: iid, Err: err}
		}

		approvals, _ := client.GetMergeRequestApprovals(projectID, iid)
		diffs, _ := client.GetMergeRequestDiff(projectID, iid)
		notes, _ := client.ListMergeRequestNotes(projectID, iid)

		var pipeline *gitlab.Pipeline
		var jobs []gitlab.Job
		if mr.Pipeline != nil {
			pipeline, _ = client.GetPipeline(projectID, mr.Pipeline.ID)
			jobs, _ = client.ListPipelineJobs(projectID, mr.Pipeline.ID)
		}

		return MRDetailLoadedMsg{
			IID: iid, MR: mr, Approvals: approvals,
			Pipeline: pipeline, Jobs: jobs, Diffs: diffs, Notes: notes,
		}
	}
}

// update absorbs the messages the page and its jobs panel care about.
func (d *mrDetail) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case mrFetchMsg:
		// Only the merge request still on screen, still unfetched, is worth asking
		// about: the steps that led here have been overtaken.
		if !d.active || m.iid != d.iid || d.requested == d.iid || d.mr == nil {
			return nil
		}
		if _, cached := d.pages[d.iid]; cached {
			return nil
		}
		return d.load(d.iid)

	case MRDetailLoadedMsg:
		if m.IID != d.iid {
			return nil // a stale reply for one we have moved off
		}
		d.loading = false
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading merge request: %v", m.Err), true)
		}
		if m.MR != nil {
			d.mr = m.MR
		}
		d.approvals, d.pipeline = m.Approvals, m.Pipeline
		d.setDiffs(m.Diffs)
		d.setNotes(m.Notes)
		if d.pipeline != nil {
			d.jobs.adopt(d.pipeline.ID, m.Jobs)
		}
		d.remember(m.IID, mrPage{
			mr: m.MR, approvals: m.Approvals, pipeline: m.Pipeline,
			jobs: m.Jobs, diffs: m.Diffs, notes: m.Notes,
		})
		return nil

	case JobsLoadedMsg:
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading jobs: %v", m.Err), true)
		}
		d.jobs.setJobs(m.Jobs)
		if page, ok := d.pages[d.iid]; ok {
			page.jobs = m.Jobs
			d.pages[d.iid] = page
		}
		return nil

	case JobTraceLoadedMsg:
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading log: %v", m.Err), true)
		}
		if strings.TrimSpace(m.Trace) == "" {
			return statusCmd("This job has not written a log yet", true)
		}
		d.jobs.setTrace(m.Trace)
		return nil

	case JobActionDoneMsg:
		if m.IsErr {
			return statusCmd(m.Text, true)
		}
		return tea.Batch(d.jobs.load(), statusCmd(m.Text, false))

	case commentWrittenMsg:
		if m.err != nil {
			return statusCmd(fmt.Sprintf("Could not open an editor: %v", m.err), true)
		}
		if m.body == "" {
			return statusCmd("Empty comment, nothing posted", false)
		}
		return d.postComment(m.body)

	case MRActionDoneMsg:
		if m.IsErr {
			return statusCmd(m.Text, true)
		}
		// Approving or merging changes what this page says, so its cached copy is
		// void and the page is refetched behind the message.
		d.forget(d.iid)
		return tea.Batch(d.load(d.iid), statusCmd(m.Text, false))
	}
	return nil
}

// ============================================================================
// Keys
// ============================================================================

// handleKey drives the page. Esc unwinds log -> box -> page -> list.
func (d *mrDetail) handleKey(key string, height int) tea.Cmd {
	// The thread has the screen: it scrolls, and c replies to what you are reading.
	if d.threadOpen {
		switch key {
		case keyEscape:
			d.closeThread()
			return nil
		case keyComment:
			return d.comment()
		case keySystem:
			return d.toggleSystem()
		}
		if d.threadKey(key, height) {
			return nil
		}
		return nil
	}

	if d.reading {
		if key == keyEscape {
			d.closeReader()
			return nil
		}
		if d.readerKey(key, height) {
			return nil
		}
		return d.copyKeys(key)
	}

	switch key {
	case keyTab:
		return d.cycleFocus(1)
	case keyShiftTab:
		return d.cycleFocus(-1)
	}

	if d.focus == focusFiles {
		if key == keyEscape {
			d.focus = focusPage
			return nil
		}
		if d.filesKey(key, height) {
			return nil
		}
		return d.copyKeys(key)
	}

	if d.jobs.showingTrace() {
		if key == keyEscape {
			d.jobs.closeTrace()
			return nil
		}
		if cmd, consumed := d.jobs.handleKey(key, height); consumed {
			return cmd
		}
		return nil
	}

	if d.focus == focusJobs {
		if key == keyEscape {
			d.focus = focusPage
			return nil
		}
		if cmd, consumed := d.jobs.handleKey(key, height); consumed {
			return cmd
		}
		return nil
	}

	if d.focus == focusNotes {
		switch key {
		case keyEscape:
			d.focus = focusPage
			return nil
		case keyComment:
			return d.comment()
		case keySystem:
			return d.toggleSystem()
		}
		if d.notesKey(key, height) {
			return nil
		}
		return d.copyKeys(key)
	}

	if act := components.NavFor(key); act != components.NavNone {
		// The description scrolls; there is nothing to select in it.
		d.scroll = components.ApplyNav(act, d.scroll, len(d.topLines(0)), listRows(height))
		return nil
	}

	switch key {
	case keyEscape:
		d.close()
		return nil
	case keyCopy, keyCopyLink:
		return d.copyKeys(key)
	case keyEnter:
		// Step into the boxes already on the page rather than replacing it.
		if len(d.diffs) > 0 {
			d.focus = focusFiles
			return nil
		}
		return d.focusJobs()
	case keyApprove:
		return d.approve()
	case keyMerge:
		return d.merge()
	case keyRetry:
		return d.retryPipeline()
	case keyComment:
		return d.comment()
	case keyOpenBrowse:
		if d.mr == nil {
			return nil
		}
		if cmd := openBrowserCmd(d.mr.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

func (d *mrDetail) cycleFocus(step int) tea.Cmd {
	// The discussion is always in the cycle, even when empty: c is how you start
	// one, so a merge request nobody has commented on still needs somewhere to
	// stand.
	d.focus = cycleFocus(d.focus, step, len(d.diffs) > 0, len(d.jobs.jobs) > 0, true)
	if d.focus == focusJobs {
		return d.focusJobs()
	}
	return nil
}

// comment opens the editor for a new comment on this merge request.
func (d *mrDetail) comment() tea.Cmd {
	if d.mr == nil {
		return nil
	}
	return composeComment(fmt.Sprintf("!%d %s", d.mr.IID, d.mr.Title))
}

// postComment sends what the editor produced, then reloads the discussion.
func (d *mrDetail) postComment(body string) tea.Cmd {
	if d.mr == nil || d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID, iid := d.ctx.Client, d.ctx.Project.ID, d.mr.IID
	return func() tea.Msg {
		if err := client.CreateMergeRequestNote(projectID, iid, body); err != nil {
			return MRActionDoneMsg{Text: fmt.Sprintf("Comment failed: %v", err), IsErr: true}
		}
		return MRActionDoneMsg{Text: fmt.Sprintf("Commented on !%d", iid)}
	}
}

// focusJobs hands the keys to the jobs listed on the page.
func (d *mrDetail) focusJobs() tea.Cmd {
	if len(d.jobs.jobs) == 0 {
		if d.pipeline == nil {
			return statusCmd("No pipeline ran for this merge request", true)
		}
		return d.jobs.load()
	}
	d.focus = focusJobs
	return nil
}

// copyKeys serves y and Y wherever the merge request itself has the focus: the
// reference you would type, the link you would send.
func (d *mrDetail) copyKeys(key string) tea.Cmd {
	if d.mr == nil {
		return nil
	}
	ref := fmt.Sprintf("!%d", d.mr.IID)
	switch key {
	case keyCopy:
		return copyRef(ref)
	case keyCopyLink:
		return copyLink(ref, d.mr.WebURL)
	}
	return nil
}

// ============================================================================
// Actions
// ============================================================================

func (d *mrDetail) approve() tea.Cmd {
	if d.mr == nil {
		return nil
	}
	// Why-not comes before the client check, so the explanation reaches the user
	// either way: a key that silently does nothing is a bug report waiting to happen.
	if d.approvals != nil && d.approvals.HasApproved {
		return statusCmd(fmt.Sprintf("You have already approved !%d", d.mr.IID), true)
	}
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID, iid := d.ctx.Client, d.ctx.Project.ID, d.mr.IID
	return confirmCmd(fmt.Sprintf("Approve !%d %s?", iid, d.mr.Title), func() tea.Msg {
		if err := client.ApproveMergeRequest(projectID, iid); err != nil {
			return MRActionDoneMsg{Text: fmt.Sprintf("Approve failed: %v", err), IsErr: true}
		}
		return MRActionDoneMsg{Text: fmt.Sprintf("Approved !%d", iid)}
	})
}

func (d *mrDetail) merge() tea.Cmd {
	if d.mr == nil {
		return nil
	}
	iid := d.mr.IID

	// Say what stands in the way rather than sending a request that cannot succeed —
	// and say it before the client check, or the reason never reaches the user.
	if d.mr.HasConflicts {
		return statusCmd(fmt.Sprintf("!%d has conflicts with %s", iid, d.mr.TargetBranch), true)
	}
	if d.mr.Draft {
		return statusCmd(fmt.Sprintf("!%d is a draft", iid), true)
	}
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID := d.ctx.Client, d.ctx.Project.ID

	prompt := fmt.Sprintf("Merge !%d into %s?", iid, d.mr.TargetBranch)
	if d.approvals != nil && d.approvals.Left > 0 {
		prompt = fmt.Sprintf("Merge !%d into %s? (%d approval(s) still missing)",
			iid, d.mr.TargetBranch, d.approvals.Left)
	}
	return confirmCmd(prompt, func() tea.Msg {
		if err := client.MergeMergeRequest(projectID, iid); err != nil {
			return MRActionDoneMsg{Text: fmt.Sprintf("Merge failed: %v", err), IsErr: true}
		}
		return MRActionDoneMsg{Text: fmt.Sprintf("Merged !%d", iid)}
	})
}

func (d *mrDetail) retryPipeline() tea.Cmd {
	if d.pipeline == nil {
		return statusCmd("No pipeline to retry for this merge request", true)
	}
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID, id := d.ctx.Client, d.ctx.Project.ID, d.pipeline.ID
	return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", id), func() tea.Msg {
		if err := client.RetryPipeline(projectID, id); err != nil {
			return MRActionDoneMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return MRActionDoneMsg{Text: fmt.Sprintf("Retried pipeline #%d", id)}
	})
}

// ============================================================================
// Rendering
// ============================================================================

// body renders the page as the view's whole body, between the arrow margins.
func (d *mrDetail) body(width, height int) string {
	if d.reading && d.selectedFile() != nil {
		d.pageWidth = width - 2*arrowGutter
		return d.withArrows(components.RenderPanel(d.readerTitle(),
			splitLines(d.diffView(d.pageWidth, height)), d.pageWidth, height, true),
			d.fileCursor > 0, d.fileCursor < len(d.diffs)-1)
	}

	// A conversation is read end to end, so it takes the body like a log does.
	if d.threadOpen {
		return components.RenderPanel(d.threadTitle(),
			splitLines(d.threadView(width, height)), width, height, true)
	}

	// A log needs the room, so it is the one thing that replaces the page.
	if d.jobs.showingTrace() {
		return components.RenderPanel(d.jobs.detailTitle(),
			splitLines(d.jobs.traceView(width, height)), width, height, true)
	}

	pageWidth := width - 2*arrowGutter
	if pageWidth < 20 {
		return d.page(width, height)
	}
	return d.withArrows(d.page(pageWidth, height), d.hasPrev(), d.hasNext())
}

// page renders the merge request itself: what it is on top, the two lists you act
// on side by side below.
func (d *mrDetail) page(width, height int) string {
	d.pageWidth = width

	top := d.topLines(width - 2)
	topHeight := len(top) + 1
	if maxTop := height * 3 / 5; topHeight > maxTop {
		topHeight = maxTop
	}
	if topHeight < 3 {
		topHeight = 3
	}

	const gap = 1
	bottomHeight := height - topHeight - gap
	if bottomHeight < 4 {
		d.descScrollable = len(top) > height-1
		return components.RenderPanel(d.title(len(top), height-1), d.window(top, height-1),
			width, height, true)
	}
	d.descScrollable = len(top) > topHeight-1

	return lipgloss.JoinVertical(lipgloss.Left,
		components.RenderPanel(d.title(len(top), topHeight-1), d.window(top, topHeight-1),
			width, topHeight, d.focus == focusPage),
		"",
		d.columns(width, bottomHeight),
	)
}

// title names the page, with its place in the list and, when the description does
// not fit, where in it you are.
func (d *mrDetail) title(lines, rows int) string {
	out := "Merge Request"
	if d.mr != nil {
		out = fmt.Sprintf("!%d", d.mr.IID)
	}
	out += d.counter()
	if lines > rows {
		end := min(d.scroll+rows, lines)
		out = fmt.Sprintf("%s  ·  %d-%d of %d", out, d.scroll+1, end, lines)
	}
	return out
}

// window is the visible slice of the top block, scrolled by the page's offset.
func (d *mrDetail) window(lines []string, rows int) []string {
	if rows < 1 {
		rows = 1
	}
	if d.scroll > len(lines)-rows {
		d.scroll = max(0, len(lines)-rows)
	}
	if d.scroll < 0 {
		d.scroll = 0
	}
	return lines[d.scroll:min(d.scroll+rows, len(lines))]
}

// columns renders the three lists you act on: the changed files, the jobs and the
// discussion. On a narrow terminal the discussion drops out of the row — Tab
// still reaches it, and it takes the body when read.
func (d *mrDetail) columns(width, height int) string {
	const rule = 3 // the rule and its spaces

	showNotes := width >= 130
	share := width / 2
	if showNotes {
		share = width / 3
	}
	if share < 20 {
		share = 20
	}

	files := d.filesBox(share, height, d.focus == focusFiles, d.loading)

	jobRows, jobToRow := d.jobs.items()
	jobCursor := -1
	if d.focus == focusJobs && d.jobs.cursor < len(jobToRow) {
		jobCursor = jobToRow[d.jobs.cursor]
	}

	jobsWidth := width - share - rule
	if showNotes {
		jobsWidth = share
	}
	jobs := renderListBox(jobsWidth, height,
		fmt.Sprintf("Jobs (%d)", len(d.jobs.jobs)), jobRows, jobCursor, &d.jobs.scroll)

	out := lipgloss.JoinHorizontal(lipgloss.Top, files, " ", components.VRule(height), " ", jobs)
	if !showNotes {
		return out
	}

	notesWidth := width - 2*share - 2*rule
	notes := d.notesPanel(notesWidth, height, d.focus == focusNotes, d.loading)
	return lipgloss.JoinHorizontal(lipgloss.Top, out, " ", components.VRule(height), " ", notes)
}

// topLines is the title, then what the merge request is and what stands between
// it and being merged, then its description.
func (d *mrDetail) topLines(width int) []string {
	mr := d.mr
	if mr == nil {
		return []string{"No merge request"}
	}

	var out []string
	add := func(s string) { out = append(out, s) }
	field := func(label, value string) {
		if value != "" {
			add(components.HelpDescStyle.Render(label) + value)
		}
	}
	wrap := func(s string) {
		if width <= 0 {
			add(s)
			return
		}
		out = append(out, components.WrapLine(s, width)...)
	}

	title := mr.Title
	if mr.Draft {
		title = "[Draft] " + title
	}
	wrap(components.TitleStyle.Render(title))
	add("")

	// The metadata comes before the description: the branch, the CI and whether it
	// can be merged are what you check, and a long description would push them
	// below the fold.
	field("branch  ", mr.SourceBranch+components.HelpDescStyle.Render(" → ")+mr.TargetBranch)
	field("author  ", mr.Author)
	if len(mr.Reviewers) > 0 {
		field("review  ", strings.Join(mr.Reviewers, ", "))
	}
	if len(mr.Assignees) > 0 {
		field("assign  ", strings.Join(mr.Assignees, ", "))
	}
	add(d.pipelineLine())
	add(d.mergeLine())
	if a := d.approvals; a != nil {
		add(d.approvalLine(a))
	}
	if len(mr.Labels) > 0 {
		field("labels  ", strings.Join(mr.Labels, ", "))
	}
	if !mr.UpdatedAt.IsZero() {
		field("updated ", util.TimeAgo(mr.UpdatedAt))
	}

	if body := strings.TrimSpace(mr.Description); body != "" {
		add("")
		for _, line := range strings.Split(body, "\n") {
			wrap(line)
		}
	}

	add("")
	wrap(components.HelpDescStyle.Render(mr.WebURL))
	return out
}

// pipelineLine is the merge request's pipeline as one line of metadata.
func (d *mrDetail) pipelineLine() string {
	switch {
	case d.loading && d.pipeline == nil:
		return components.HelpDescStyle.Render("ci      loading…")
	case d.pipeline == nil:
		return components.HelpDescStyle.Render("ci      no pipeline")
	}

	p := d.pipeline
	status := p.Status
	if p.HasWarnings {
		status = components.StatusWarning
	}
	label := p.StatusLabel
	if label == "" {
		label = p.Status
	}
	return components.HelpDescStyle.Render("ci      ") +
		components.StatusIconPadded(status) + fmt.Sprintf("#%d %s", p.ID, label)
}

// mergeLine says whether it can be merged, in GitLab's own terms, and colours the
// answer: this is the thing you opened the page to find out.
func (d *mrDetail) mergeLine() string {
	label := components.HelpDescStyle.Render("merge   ")
	if d.mr.HasConflicts {
		return label + lipgloss.NewStyle().Foreground(components.ColorError).
			Render("conflicts with "+d.mr.TargetBranch)
	}
	switch status := d.mr.MergeStatus; status {
	case "":
		if d.loading {
			return label + components.HelpDescStyle.Render("loading…")
		}
		return label + components.HelpDescStyle.Render("unknown")
	case "mergeable", "can_be_merged":
		return label + lipgloss.NewStyle().Foreground(components.ColorSuccess).Render("ready")
	default:
		// GitLab's own wording, underscores and all: "ci_still_running",
		// "not_approved", "draft_status", … Better its words than our guess.
		return label + lipgloss.NewStyle().Foreground(components.ColorWarning).
			Render(strings.ReplaceAll(status, "_", " "))
	}
}

// approvalLine says how many approvals are still needed and who has given theirs.
func (d *mrDetail) approvalLine(a *gitlab.MRApprovals) string {
	label := components.HelpDescStyle.Render("approve ")
	switch {
	case a.Approved && len(a.ApprovedBy) > 0:
		return label + lipgloss.NewStyle().Foreground(components.ColorSuccess).
			Render("approved") + components.MutedStyle.Render(" by "+strings.Join(a.ApprovedBy, ", "))
	case a.Left > 0:
		out := label + lipgloss.NewStyle().Foreground(components.ColorWarning).
			Render(fmt.Sprintf("%d still needed", a.Left))
		if len(a.ApprovedBy) > 0 {
			out += components.MutedStyle.Render("  (" + strings.Join(a.ApprovedBy, ", ") + ")")
		}
		return out
	case len(a.ApprovedBy) > 0:
		return label + components.MutedStyle.Render("by "+strings.Join(a.ApprovedBy, ", "))
	default:
		return label + components.HelpDescStyle.Render("none required")
	}
}

// ============================================================================
// KeyHints
// ============================================================================

func (d *mrDetail) keyHints() []KeyHint {
	if d.threadOpen {
		return d.threadHints()
	}
	if d.focus == focusNotes {
		return append(d.boxHints(),
			KeyHint{"←/→ h/l", "MR"},
			KeyHint{"Tab", "Changes"},
			KeyHint{"Esc", "Back"},
		)
	}
	if d.reading {
		return d.readerHints("Copy !/link")
	}
	if d.focus == focusFiles {
		return []KeyHint{
			{"Enter", "Read diff"},
			{"j/k", "File"},
			{"←/→ h/l", "MR"},
			{"Tab", "Jobs"},
			{"y/Y", "Copy !/link"},
			{"Esc", "Back"},
		}
	}
	if d.jobs.showingTrace() {
		return d.jobs.keyHints()
	}
	if d.focus == focusJobs {
		return append(d.jobs.keyHints(),
			KeyHint{"←/→ h/l", "MR"},
			KeyHint{"Tab", "Changes"},
		)
	}

	hints := []KeyHint{
		{"←/→ h/l", "Prev/next MR"},
		{"Enter", "Step in"},
		{"Tab", "Changes/Jobs"},
	}
	if d.descScrollable {
		hints = append(hints, KeyHint{"j/k", "Scroll"})
	}
	return append(hints,
		KeyHint{"a", "Approve"},
		KeyHint{"m", "Merge"},
		KeyHint{"c", "Comment"},
		KeyHint{"R", "Retry pipeline"},
		KeyHint{"y/Y", "Copy !/link"},
		KeyHint{"o", "Open"},
		KeyHint{"Esc", "Back"},
	)
}
