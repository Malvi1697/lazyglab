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

	// changesBox is the commit's changed files and the reader for one of them; the
	// merge-request page embeds the same thing. Its fields read as ours: d.diffs,
	// d.reading, d.fileCursor.
	changesBox

	// pageFrame is our place in the list we were opened from: the counter in the
	// heading and the ‹ › arrows in the margins.
	pageFrame

	// messageScrollable is learned while rendering, so the footer offers j/k for
	// the message only where it would move something.
	messageScrollable bool

	sha       string // the commit on screen; replies for anything else are stale
	requested string // the commit we have actually asked GitLab about
	loading   bool
	scroll    int

	// jobs is the same interactive panel the Pipelines view uses. Its rows are
	// rendered inline in the page, and Enter moves the focus into them rather than
	// swapping the screen for a panel — the jobs are already in front of you.
	jobs jobsPanel

	// focus says whether the keys drive the page, its files or its jobs.
	focus pageFocus

	// pages remembers the last few commits fetched, keyed by SHA, so stepping back
	// and forth with h/l is free after the first pass. Each page is six requests,
	// and walking a list of commits is exactly the thing you do twice.
	pages map[string]commitPage
	order []string // insertion order, for evicting the oldest
}

// commitPage is everything a commit's page shows, as fetched.
type commitPage struct {
	commit    *gitlab.Commit
	pipelines []gitlab.Pipeline
	refs      []gitlab.CommitRef
	mrs       []gitlab.MergeRequest
	jobs      []gitlab.Job
	diffs     []gitlab.FileDiff
}

// commitPagesKept bounds the cache. A page holds its diffs, which can be large,
// so this is a handful rather than a history.
const commitPagesKept = 6

// newCommitDetail builds the page, wiring the shared context into the nested
// jobs panel too — forgetting that leaves a panel that silently cannot load.
func newCommitDetail(ctx *Context) commitDetail {
	return commitDetail{ctx: ctx, jobs: jobsPanel{ctx: ctx}, pages: map[string]commitPage{}}
}

// openAt drills into a commit, remembering its place in the list so the page can
// step to the neighbouring commits. Enter means "show me this one", so it fetches
// at once.
func (d *commitDetail) openAt(c *gitlab.Commit, index, total int) tea.Cmd {
	return d.showAt(c, index, total, 0)
}

// stepAt is openAt for a step to a neighbour, which waits a moment before asking
// GitLab anything.
//
// A page is six requests, and holding h or l walks through commits faster than
// any of them could answer — sixty requests to look at the tenth one. The title
// and author are already in hand from the list, so they appear immediately; the
// rest is fetched once you stop moving.
func (d *commitDetail) stepAt(c *gitlab.Commit, index, total int) tea.Cmd {
	return d.showAt(c, index, total, stepSettleDelay)
}

// stepSettleDelay is how long a step waits for the next one before fetching. Long
// enough to swallow a key repeat, short enough that a single press feels immediate.
const stepSettleDelay = 120 * time.Millisecond

// commitFetchMsg asks the page to fetch a commit, if it is still the one shown.
type commitFetchMsg struct{ sha string }

func (d *commitDetail) showAt(c *gitlab.Commit, index, total int, delay time.Duration) tea.Cmd {
	if c == nil {
		return nil
	}
	d.active = true
	d.commit = c
	d.placeIn(index, total)
	d.pipelines, d.refs, d.mrs = nil, nil, nil
	d.jobs.close()
	d.focus = focusPage
	d.resetFiles()
	d.scroll = 0

	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	d.sha, d.requested = sha, ""

	// Already fetched: step back through commits without asking GitLab again.
	if page, ok := d.pages[sha]; ok {
		d.restore(page)
		return nil
	}

	d.loading = true
	if delay <= 0 {
		return d.load(c)
	}
	// Ask again once the dust settles; if another step has happened by then, the
	// commit on screen will have changed and this fetch is dropped.
	return tea.Tick(delay, func(time.Time) tea.Msg { return commitFetchMsg{sha: sha} })
}

// restore puts a cached page back on screen.
func (d *commitDetail) restore(page commitPage) {
	d.loading = false
	if page.commit != nil {
		d.commit = page.commit
	}
	d.pipelines, d.refs, d.mrs = page.pipelines, page.refs, page.mrs
	d.setDiffs(page.diffs)
	if p := d.pipeline(); p != nil {
		d.jobs.adopt(p.ID, page.jobs)
	}
}

// remember caches a fetched page, evicting the oldest once the handful is full.
func (d *commitDetail) remember(sha string, page commitPage) {
	if sha == "" {
		return
	}
	if d.pages == nil {
		d.pages = map[string]commitPage{}
	}
	if _, seen := d.pages[sha]; !seen {
		d.order = append(d.order, sha)
		for len(d.order) > commitPagesKept {
			delete(d.pages, d.order[0])
			d.order = d.order[1:]
		}
	}
	d.pages[sha] = page
}

// forget drops a cached page, for when an action has changed what it would show.
func (d *commitDetail) forget(sha string) {
	delete(d.pages, sha)
	for i, s := range d.order {
		if s == sha {
			d.order = append(d.order[:i], d.order[i+1:]...)
			break
		}
	}
}

// close returns to the list.
func (d *commitDetail) close() {
	d.active = false
	d.jobs.close()
	d.pipelines, d.refs, d.mrs = nil, nil, nil
	d.focus = focusPage
	d.resetFiles()
	d.sha, d.requested = "", ""
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
	d.sha, d.requested = sha, sha
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

		// Branches, tags, merge requests and the changed files round out the page;
		// a failure in any of them must not lose the rest.
		refs, _ := client.GetCommitRefs(projectID, sha)
		mrs, _ := client.ListCommitMergeRequests(projectID, sha)
		diffs, _ := client.GetCommitDiff(projectID, sha)

		return CommitDetailLoadedMsg{
			SHA: sha, Commit: commit, Pipelines: pipelines, Refs: refs, MRs: mrs,
			Jobs: jobs, Diffs: diffs,
		}
	}
}

// update absorbs every message the page and its jobs panel care about and returns
// whatever follows from it, including anything the user needs told.
//
// Routing lives here rather than in each host: the panel needs its job list,
// logs and action results forwarded, and duplicating that wiring per view is how
// a host ends up silently showing "No jobs". It reports through StatusMsg rather
// than returning a string, because a returned string is exactly what one host
// quietly threw away — every error and note on this page was invisible.
func (d *commitDetail) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case commitFetchMsg:
		// The step that asked for this may have been followed by others; only the
		// commit still on screen, still unfetched, is worth a request.
		if !d.active || m.sha != d.sha || d.requested == d.sha || d.commit == nil {
			return nil
		}
		if _, cached := d.pages[d.sha]; cached {
			return nil
		}
		return d.load(d.commit)

	case CommitDetailLoadedMsg:
		if m.SHA != d.sha {
			return nil // a stale reply for a commit we have moved off
		}
		d.loading = false
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading commit: %v", m.Err), true)
		}
		if m.Commit != nil {
			d.commit = m.Commit
		}
		d.pipelines, d.refs, d.mrs = m.Pipelines, m.Refs, m.MRs
		d.setDiffs(m.Diffs)
		d.remember(m.SHA, commitPage{
			commit: m.Commit, pipelines: m.Pipelines, refs: m.Refs,
			mrs: m.MRs, jobs: m.Jobs, diffs: m.Diffs,
		})
		// The panel takes the jobs we already have, so the rows on the page and the
		// rows you act on are the same list.
		if p := d.pipeline(); p != nil {
			d.jobs.adopt(p.ID, m.Jobs)
		}
		return nil

	case JobsLoadedMsg:
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading jobs: %v", m.Err), true)
		}
		d.jobs.setJobs(m.Jobs)
		if page, ok := d.pages[d.sha]; ok {
			page.jobs = m.Jobs
			d.pages[d.sha] = page
		}
		return nil

	case JobTraceLoadedMsg:
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading log: %v", m.Err), true)
		}
		// A manual or pending job has nothing written yet. Swapping the screen for an
		// empty panel — or doing nothing at all, as this used to — says neither.
		if strings.TrimSpace(m.Trace) == "" {
			return statusCmd("This job has not written a log yet", true)
		}
		d.jobs.setTrace(m.Trace)
		return nil

	case JobActionDoneMsg:
		// An action changes the job, so reload the list behind it.
		if m.IsErr {
			return statusCmd(m.Text, true)
		}
		return tea.Batch(d.jobs.load(), statusCmd(m.Text, false))

	case PipelineActionDoneMsg:
		if m.IsErr || d.commit == nil {
			return statusCmd(m.Text, m.IsErr)
		}
		// Retrying or starting a pipeline changes what this commit shows, so the
		// cached copy of it is void.
		d.forget(d.sha)
		return tea.Batch(d.load(d.commit), statusCmd(m.Text, false))
	}
	return nil
}

// handleKey drives the detail. Esc unwinds log -> jobs -> page -> list.
func (d *commitDetail) handleKey(key string, height int) tea.Cmd {
	// A diff being read owns the body; navigation scrolls it.
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

	// Tab moves the focus between the page's boxes in every state except while
	// reading, so it does not have to be repeated in each branch below.
	switch key {
	case keyTab:
		return d.cycleFocus(1)
	case keyShiftTab:
		return d.cycleFocus(-1)
	}

	// Focus inside the changed files: Enter reads the highlighted one.
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

	// An open log is read full-screen; navigation scrolls it.
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

	// Focus inside the jobs listed on the page: the rows in front of you are the
	// ones the keys act on, and the page keeps its context above them.
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

	if act := components.NavFor(key); act != components.NavNone {
		// The message scrolls; there is nothing to select in it.
		d.scroll = components.ApplyNav(act, d.scroll, len(d.topLines(0)), listRows(height))
		return nil
	}

	switch key {
	case keyEscape:
		d.close()
		return nil
	case keyCopy, keyCopyLink:
		return d.copyKeys(key)
	case keyOpenBrowse:
		if d.commit == nil {
			return nil
		}
		if cmd := openBrowserCmd(d.commit.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
		return nil
	case keyEnter:
		// Step into the sections already on the page rather than replacing it. The
		// changes are what a commit is, so they come first; the jobs are one Tab on.
		if len(d.diffs) > 0 {
			d.focus = focusFiles
			return nil
		}
		return d.focusJobs()
	case keyRetry:
		return d.retryPipeline()
	case keyRun:
		return d.runOnRef()
	}
	return nil
}

// cycleFocus hands the keys to the next of the page's boxes.
func (d *commitDetail) cycleFocus(step int) tea.Cmd {
	d.focus = cycleFocus(d.focus, step, len(d.diffs) > 0, len(d.jobs.jobs) > 0)
	if d.focus == focusJobs {
		return d.focusJobs()
	}
	return nil
}

// readingBody reports whether something long-form has the screen — a file's diff
// or a job's log — so the view hosting the page knows the arrows are not its to
// act on. Stepping to another commit from inside either would swap what you are
// reading for something from a different commit.
func (d *commitDetail) readingBody() bool { return d.reading || d.jobs.showingTrace() }

// focusJobs hands the keys to the jobs listed on the page.
func (d *commitDetail) focusJobs() tea.Cmd {
	if len(d.jobs.jobs) == 0 {
		if d.pipeline() == nil {
			return func() tea.Msg {
				return StatusMsg{Text: "No pipeline ran for this commit", IsErr: true}
			}
		}
		// The pipeline exists but its jobs have not arrived yet.
		return d.jobs.load()
	}
	d.focus = focusJobs
	return nil
}

// copyKeys serves y and Y wherever the commit itself has the focus: the SHA you
// would type, the link you would send. In the jobs box they belong to the job, so
// this is not reached from there.
func (d *commitDetail) copyKeys(key string) tea.Cmd {
	switch key {
	case keyCopy:
		return d.copySHA()
	case keyCopyLink:
		if d.commit == nil {
			return nil
		}
		return copyLink(d.commit.ShortID, d.commit.WebURL)
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
//
// Each state says what the arrows do there, because it differs: on the page they
// step commits, in a diff they step that commit's files, and while a log is open
// they do nothing at all. A footer that only ever named "←/→" left the h/l pair
// undocumented and the difference unsaid.
func (d *commitDetail) keyHints() []KeyHint {
	if d.reading {
		return d.readerHints("Copy SHA/link")
	}
	if d.focus == focusFiles {
		return []KeyHint{
			{"Enter", "Read diff"},
			{"j/k", "File"},
			{"←/→ h/l", "Commit"},
			{"Tab", "Jobs"},
			{"y/Y", "Copy SHA/link"},
			{"Esc", "Back"},
		}
	}
	if d.jobs.showingTrace() {
		// A log has the screen; the commit's own keys are out of reach until Esc.
		return d.jobs.keyHints()
	}
	if d.focus == focusJobs {
		return append(d.jobs.keyHints(),
			KeyHint{"←/→ h/l", "Commit"},
			KeyHint{"Tab", "Changes"},
		)
	}
	hints := []KeyHint{
		{"←/→ h/l", "Prev/next commit"},
		{"Enter", "Step in"},
		{"Tab", "Changes/Jobs"},
	}
	// Only worth saying when the message does not fit; otherwise j/k are inert here
	// and the footer would be promising movement that cannot happen.
	if d.messageScrollable {
		hints = append(hints, KeyHint{"j/k", "Scroll"})
	}
	return append(hints,
		KeyHint{"R", "Retry pipeline"},
		KeyHint{"p", "Run on branch"},
		KeyHint{"y/Y", "Copy SHA/link"},
		KeyHint{"o", "Open"},
		KeyHint{"Esc", "Back"},
	)
}

// ============================================================================
// Rendering
// ============================================================================

// body renders the detail as the view's whole body, between two margins that
// carry the arrows for stepping to the neighbouring commits.
func (d *commitDetail) body(width, height int) string {
	// A diff needs the room, like a log does.
	if d.reading {
		if d.selectedFile() != nil {
			// withArrows pads to pageWidth, which the page normally sets; a diff has
			// to set it too or the right arrow lands against the text.
			d.pageWidth = width - 2*arrowGutter
			return d.withArrows(components.RenderPanel(d.readerTitle(),
				splitLines(d.diffView(d.pageWidth, height)), d.pageWidth, height, true),
				d.fileCursor > 0, d.fileCursor < len(d.diffs)-1)
		}
	}

	// A log needs the room, so it is the one thing that replaces the page.
	if d.jobs.showingTrace() {
		return components.RenderPanel(d.jobs.detailTitle(),
			splitLines(d.jobs.traceView(width, height)), width, height, true)
	}

	pageWidth := width - 2*arrowGutter
	if pageWidth < 20 {
		// Too narrow for margins; drop them rather than squeezing the text.
		return d.page(width, height)
	}
	return d.withArrows(d.page(pageWidth, height), d.hasPrev(), d.hasNext())
}

// page renders the commit itself.
func (d *commitDetail) page(width, height int) string {
	d.pageWidth = width

	// The message reads across the top; the two lists that you act on sit side by
	// side below it. Stacked, they made the page a long vertical scroll while half
	// the terminal stayed empty.
	top := d.topLines(width - 2)
	topHeight := len(top) + 1 // a heading of its own
	if maxTop := height * 3 / 5; topHeight > maxTop {
		topHeight = maxTop
	}
	if topHeight < 3 {
		topHeight = 3
	}

	const gap = 1
	bottomHeight := height - topHeight - gap
	if bottomHeight < 4 {
		// A short terminal: keep the message and drop the columns, which would be
		// two headings and nothing else.
		d.messageScrollable = len(top) > height-1
		return components.RenderPanel(d.title(len(top), height-1), d.window(top, height-1), width, height, true)
	}

	d.messageScrollable = len(top) > topHeight-1

	return lipgloss.JoinVertical(lipgloss.Left,
		components.RenderPanel(d.title(len(top), topHeight-1), d.window(top, topHeight-1), width, topHeight, d.focus == focusPage),
		"",
		d.columns(width, bottomHeight),
	)
}

// title names the page, with the commit's place in the list and, when the message
// does not fit, where in it you are.
func (d *commitDetail) title(lines, rows int) string {
	out := "Commit"
	if d.commit != nil {
		out = "Commit " + d.commit.ShortID
	}
	out += d.counter()
	if lines > rows {
		end := min(d.scroll+rows, lines)
		out = fmt.Sprintf("%s  ·  %d-%d of %d", out, d.scroll+1, end, lines)
	}
	return out
}

// window is the visible slice of the message, scrolled by the page's own offset.
func (d *commitDetail) window(lines []string, rows int) []string {
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

// columns renders the changed files beside the jobs, each scrolling on its own
// and the focused one taking the bolder heading.
func (d *commitDetail) columns(width, height int) string {
	leftWidth := width / 2
	if leftWidth < 20 {
		leftWidth = 20
	}
	rightWidth := width - leftWidth - 3 // the rule and its spaces

	files := d.filesBox(leftWidth, height, d.focus == focusFiles, d.loading)

	jobRows, jobToRow := d.jobs.items()
	jobCursor := -1
	if d.focus == focusJobs && d.jobs.cursor < len(jobToRow) {
		jobCursor = jobToRow[d.jobs.cursor]
	}
	jobs := renderListBox(rightWidth, height,
		fmt.Sprintf("Jobs (%d)", len(d.jobs.jobs)), jobRows, jobCursor, &d.jobs.scroll)

	return lipgloss.JoinHorizontal(lipgloss.Top, files, " ", components.VRule(height), " ", jobs)
}

// topLines is the message and what the commit belongs to, including its pipeline.
func (d *commitDetail) topLines(width int) []string {
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

	// The metadata comes before the message body: the CI status and the branch are
	// what you check, and a long message would push them below the fold.
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
	out = append(out, d.pipelineLine())

	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.Message), c.Title))
	if body != "" {
		add("")
		for _, line := range strings.Split(body, "\n") {
			wrap(line)
		}
	}

	add("")
	wrap(components.HelpDescStyle.Render(c.WebURL))
	return out
}

// pipelineLine is the commit's pipeline as one line of metadata: a single status
// does not need a section of its own.
func (d *commitDetail) pipelineLine() string {
	switch {
	case d.loading && len(d.pipelines) == 0:
		return components.HelpDescStyle.Render("ci      loading…")
	case len(d.pipelines) == 0:
		// Nothing ran; p starts one on the branch, which is not the same thing.
		return components.HelpDescStyle.Render("ci      no pipeline (p runs one on the branch head)")
	}

	p := d.pipelines[0]
	status := p.Status
	if p.HasWarnings {
		status = components.StatusWarning
	}
	label := p.StatusLabel
	if label == "" {
		label = p.Status
	}
	line := components.HelpDescStyle.Render("ci      ") +
		components.StatusIconPadded(status) +
		fmt.Sprintf("#%d %s", p.ID, label)
	if len(d.pipelines) > 1 {
		line += components.MutedStyle.Render(fmt.Sprintf("  (+%d more)", len(d.pipelines)-1))
	}
	return line
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
