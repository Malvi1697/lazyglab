package views

import (
	"fmt"
	"image/color"
	"strings"

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

	// diffs are the commit's changed files. fileCursor picks one; reading is the
	// full-body diff, the same shape as reading a job log.
	diffs      []gitlab.FileDiff
	fileCursor int
	fileScroll int
	reading    bool
	diffScroll int

	sha     string // request in flight, to ignore stale replies
	loading bool
	scroll  int

	// index and total place the commit within the list it was opened from, so the
	// page can offer the neighbours and say where you are.
	index, total int
	// pageWidth is the last width the page was rendered at, so a selected job row
	// can span it.
	pageWidth int

	// jobs is the same interactive panel the Pipelines view uses. Its rows are
	// rendered inline in the page, and Enter moves the focus into them rather than
	// swapping the screen for a panel — the jobs are already in front of you.
	jobs jobsPanel

	// focus says whether the keys drive the page or the jobs listed on it.
	focus commitFocus
}

// commitFocus is which part of the commit page the keys drive.
type commitFocus int

const (
	focusPage commitFocus = iota
	focusFiles
	focusJobs
)

// newCommitDetail builds the page, wiring the shared context into the nested
// jobs panel too — forgetting that leaves a panel that silently cannot load.
func newCommitDetail(ctx *Context) commitDetail {
	return commitDetail{ctx: ctx, jobs: jobsPanel{ctx: ctx}}
}

// openAt drills into a commit, remembering its place in the list so the page can
// step to the neighbouring commits.
func (d *commitDetail) openAt(c *gitlab.Commit, index, total int) tea.Cmd {
	if c == nil {
		return nil
	}
	d.active = true
	d.commit = c
	d.index, d.total = index, total
	d.pipelines, d.refs, d.mrs, d.diffs = nil, nil, nil, nil
	d.jobs.close()
	d.focus = focusPage
	d.fileCursor, d.reading, d.diffScroll = 0, false, 0
	d.scroll = 0
	return d.load(c)
}

// hasPrev and hasNext report whether the page can step to a neighbour.
func (d *commitDetail) hasPrev() bool { return d.total > 0 && d.index > 0 }
func (d *commitDetail) hasNext() bool { return d.total > 0 && d.index < d.total-1 }

// close returns to the list.
func (d *commitDetail) close() {
	d.active = false
	d.jobs.close()
	d.pipelines, d.refs, d.mrs, d.diffs = nil, nil, nil, nil
	d.focus = focusPage
	d.fileCursor, d.reading, d.diffScroll = 0, false, 0
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

// update absorbs every message the page and its jobs panel care about, and
// returns any follow-up command plus a status line for the host view.
//
// Routing lives here rather than in each host: the panel needs its job list,
// logs and action results forwarded, and duplicating that wiring per view is how
// a host ends up silently showing "No jobs".
func (d *commitDetail) update(msg tea.Msg) (tea.Cmd, string) {
	switch m := msg.(type) {
	case CommitDetailLoadedMsg:
		if m.SHA != d.sha {
			return nil, "" // a stale reply for a commit we have moved off
		}
		d.loading = false
		if m.Err != nil {
			return nil, fmt.Sprintf("Error loading commit: %v", m.Err)
		}
		if m.Commit != nil {
			d.commit = m.Commit
		}
		d.pipelines, d.refs, d.mrs, d.diffs = m.Pipelines, m.Refs, m.MRs, m.Diffs
		if d.fileCursor >= len(d.diffs) {
			d.fileCursor = 0
		}
		// The panel takes the jobs we already have, so the rows on the page and the
		// rows you act on are the same list.
		if p := d.pipeline(); p != nil {
			d.jobs.adopt(p.ID, m.Jobs)
		}
		return nil, ""

	case JobsLoadedMsg:
		if m.Err != nil {
			return nil, fmt.Sprintf("Error loading jobs: %v", m.Err)
		}
		d.jobs.setJobs(m.Jobs)
		return nil, ""

	case JobTraceLoadedMsg:
		if m.Err != nil {
			return nil, fmt.Sprintf("Error loading log: %v", m.Err)
		}
		d.jobs.setTrace(m.Trace)
		return nil, ""

	case JobActionDoneMsg:
		// An action changes the job, so reload the list behind it.
		if m.IsErr {
			return nil, m.Text
		}
		return d.jobs.load(), m.Text

	case PipelineActionDoneMsg:
		if m.IsErr || d.commit == nil {
			return nil, m.Text
		}
		// Retrying or starting a pipeline changes what this commit shows.
		return d.load(d.commit), m.Text
	}
	return nil, ""
}

// handleKey drives the detail. Esc unwinds log -> jobs -> page -> list.
func (d *commitDetail) handleKey(key string, height int) tea.Cmd {
	// A diff being read owns the body; navigation scrolls it.
	if d.reading {
		if key == keyEscape {
			d.reading = false
			d.diffScroll = 0
			return nil
		}
		// The arrows step within what you are looking at: files here, commits on
		// the page. Stepping commits from inside a diff would swap the file under
		// you for one from another commit.
		if step, ok := commitStep(key); ok {
			d.stepFile(step)
			return nil
		}
		if act := components.NavFor(key); act != components.NavNone {
			d.diffScroll = scrollBy(act, d.diffScroll, listRows(height))
			return nil
		}
		if key == keyCopy {
			return d.copySHA()
		}
		return nil
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
		switch key {
		case keyEscape:
			d.focus = focusPage
			return nil
		case keyEnter:
			if d.selectedFile() != nil {
				d.reading = true
				d.diffScroll = 0
			}
			return nil
		case keyCopy:
			return d.copySHA()
		}
		if act := components.NavFor(key); act != components.NavNone {
			d.fileCursor = components.ApplyNav(act, d.fileCursor, len(d.diffs), listRows(height))
			return nil
		}
		return nil
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
		switch key {
		case keyEscape:
			d.focus = focusPage
			return nil
		case keyCopy:
			return d.copySHA()
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

// cycleFocus moves the focus between the page's boxes: the message, the changed
// files and the jobs, in that order, wrapping in the given direction. Boxes with
// nothing in them are skipped, so Tab never lands somewhere empty.
func (d *commitDetail) cycleFocus(step int) tea.Cmd {
	order := []commitFocus{focusPage}
	if len(d.diffs) > 0 {
		order = append(order, focusFiles)
	}
	if len(d.jobs.jobs) > 0 {
		order = append(order, focusJobs)
	}
	if len(order) == 1 {
		return nil // only the message; nothing to cycle to
	}

	at := 0
	for i, f := range order {
		if f == d.focus {
			at = i
			break
		}
	}
	d.focus = order[(at+step+len(order))%len(order)]
	if d.focus == focusJobs {
		return d.focusJobs()
	}
	return nil
}

// stepFile moves to the neighbouring changed file, keeping its diff open.
func (d *commitDetail) stepFile(step int) {
	next := d.fileCursor + step
	if next < 0 || next >= len(d.diffs) {
		return // already at an end
	}
	d.fileCursor = next
	d.diffScroll = 0
}

// readingBody reports whether something long-form has the screen — a file's diff
// or a job's log — so the view hosting the page knows the arrows are not its to
// act on. Stepping to another commit from inside either would swap what you are
// reading for something from a different commit.
func (d *commitDetail) readingBody() bool { return d.reading || d.jobs.showingTrace() }

// scrollBy moves a scroll offset by a navigation action, for content that is read
// rather than selected from.
func scrollBy(act components.NavAction, offset, rows int) int {
	switch act {
	case components.NavDown:
		return offset + 1
	case components.NavUp:
		return max(0, offset-1)
	case components.NavHalfDown:
		return offset + rows/2
	case components.NavHalfUp:
		return max(0, offset-rows/2)
	case components.NavPageDown:
		return offset + rows
	case components.NavPageUp:
		return max(0, offset-rows)
	case components.NavTop:
		return 0
	case components.NavBottom:
		return offset + 1<<20 // clamped while rendering, which knows the length
	}
	return offset
}

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
func (d *commitDetail) keyHints() []KeyHint {
	if d.reading {
		return []KeyHint{{"Esc", "Back"}, {"j/k", "Scroll"}, {"y", "Copy SHA"}}
	}
	if d.focus == focusFiles {
		return []KeyHint{
			{"Enter", "Read diff"},
			{"j/k", "File"},
			{"Tab", "Jobs"},
			{"Esc", "Back"},
		}
	}
	if d.jobs.showingTrace() || d.focus == focusJobs {
		return d.jobs.keyHints()
	}
	return []KeyHint{
		{"←/→", "Prev/next commit"},
		{"Enter", "Step in"},
		{"Tab", "Changes/Jobs"},
		{"R", "Retry pipeline"},
		{"p", "Run on branch"},
		{"y", "Copy SHA"},
		{"o", "Open"},
		{"Esc", "Back"},
	}
}

// ============================================================================
// Rendering
// ============================================================================

// arrowGutter is the width of the margins that carry the ‹ › step arrows.
const arrowGutter = 3

// body renders the detail as the view's whole body, between two margins that
// carry the arrows for stepping to the neighbouring commits.
func (d *commitDetail) body(width, height int) string {
	// A diff needs the room, like a log does.
	if d.reading {
		if f := d.selectedFile(); f != nil {
			// Which file of how many, the same way the page says which commit.
			title := fmt.Sprintf("%s  %d/%d", f.Path(), d.fileCursor+1, len(d.diffs))

			// withArrows pads to pageWidth, which the page normally sets; a diff has
			// to set it too or the right arrow lands against the text.
			d.pageWidth = width - 2*arrowGutter
			return d.withArrows(components.RenderPanel(title,
				splitLines(d.diffView(d.pageWidth, height)), d.pageWidth, height, true))
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
	return d.withArrows(d.page(pageWidth, height))
}

// withArrows frames the page with ‹ and › in the left and right margins, level
// with the middle of the page. They are the shape of the keys that move between
// commits, put where the movement happens rather than only named in the footer;
// an arrow with nowhere to go is faint.
func (d *commitDetail) withArrows(page string) string {
	lines := strings.Split(page, "\n")
	middle := len(lines) / 2

	style := func(available bool) lipgloss.Style {
		if available {
			return components.TitleStyle
		}
		return components.FaintStyle
	}

	// In a diff the arrows step files, on the page they step commits.
	prev, next := d.hasPrev(), d.hasNext()
	if d.reading {
		prev, next = d.fileCursor > 0, d.fileCursor < len(d.diffs)-1
	}

	pad := strings.Repeat(" ", arrowGutter)
	out := make([]string, len(lines))
	for i, line := range lines {
		left, right := pad, pad
		if i == middle {
			left = " " + style(prev).Render("‹") + " "
			right = " " + style(next).Render("›") + " "
		}
		// Pad to the page width, or the right arrow trails the text instead of
		// sitting in the margin.
		out[i] = left + components.PadRight(line, d.pageWidth) + right
	}
	return strings.Join(out, "\n")
}

// commitStep maps a key to a move between commits: the arrows, plus H/L for
// hands that stay on the home row.
func commitStep(key string) (int, bool) {
	switch key {
	case "left", "H":
		return -1, true
	case "right", "L":
		return 1, true
	}
	return 0, false
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
		return components.RenderPanel(d.title(len(top), height-1), d.window(top, height-1), width, height, true)
	}

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
	if d.total > 0 {
		out = fmt.Sprintf("%s  %d/%d", out, d.index+1, d.total)
	}
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

	files := renderListBox(leftWidth, height,
		fmt.Sprintf("Changes (%d)", len(d.diffs)), d.fileRows(),
		cursorWhen(d.focus == focusFiles, d.fileCursor), &d.fileScroll)

	jobRows, jobToRow := d.jobs.items()
	jobCursor := -1
	if d.focus == focusJobs && d.jobs.cursor < len(jobToRow) {
		jobCursor = jobToRow[d.jobs.cursor]
	}
	jobs := renderListBox(rightWidth, height,
		fmt.Sprintf("Jobs (%d)", len(d.jobs.jobs)), jobRows, jobCursor, &d.jobs.scroll)

	return lipgloss.JoinHorizontal(lipgloss.Top, files, " ", components.VRule(height), " ", jobs)
}

// cursorWhen returns the cursor only for a focused list, so an unfocused one has
// no highlighted row.
func cursorWhen(focused bool, cursor int) int {
	if focused {
		return cursor
	}
	return -1
}

// fileRows renders the changed files as list rows.
func (d *commitDetail) fileRows() []string {
	if d.loading && len(d.diffs) == 0 {
		return []string{components.HelpDescStyle.Render("Loading…")}
	}
	if len(d.diffs) == 0 {
		return []string{components.HelpDescStyle.Render("No changes reported")}
	}
	rows := make([]string, len(d.diffs))
	for i, f := range d.diffs {
		rows[i] = fmt.Sprintf("%s %s%s", fileMark(f), f.Path(), diffStat(f))
	}
	return rows
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

// fileMark is the one-letter state of a changed file, coloured like a diff.
func fileMark(f gitlab.FileDiff) string {
	style := func(c color.Color, s string) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
	}
	switch {
	case f.New:
		return style(components.ColorSuccess, "A")
	case f.Deleted:
		return style(components.ColorError, "D")
	case f.Renamed:
		return style(components.ColorRunning, "R")
	default:
		return style(components.ColorWarning, "M")
	}
}

// diffStat renders "+12 -3" for a file, or says the diff was withheld.
func diffStat(f gitlab.FileDiff) string {
	if f.Withheld {
		return components.MutedStyle.Render("  (too large to show)")
	}
	out := ""
	if f.Added > 0 {
		out += lipgloss.NewStyle().Foreground(components.ColorSuccess).Render(fmt.Sprintf("  +%d", f.Added))
	}
	if f.Removed > 0 {
		out += lipgloss.NewStyle().Foreground(components.ColorError).Render(fmt.Sprintf("  -%d", f.Removed))
	}
	return out
}

// selectedFile returns the highlighted file, or nil.
func (d *commitDetail) selectedFile() *gitlab.FileDiff {
	if d.fileCursor < 0 || d.fileCursor >= len(d.diffs) {
		return nil
	}
	return &d.diffs[d.fileCursor]
}

// diffView renders the selected file's unified diff, coloured by line kind.
func (d *commitDetail) diffView(width, height int) string {
	f := d.selectedFile()
	if f == nil {
		return ""
	}
	if f.Withheld {
		return components.MutedStyle.Render("GitLab did not send this diff: the change is too large.")
	}

	contentWidth := width - 2
	if contentWidth < 1 {
		contentWidth = 1
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimRight(f.Diff, "\n"), "\n") {
		for _, wrapped := range components.WrapLine(line, contentWidth) {
			lines = append(lines, styleDiffLine(wrapped))
		}
	}

	rows := height - 1
	if rows < 1 {
		rows = 1
	}
	if maxScroll := len(lines) - rows; d.diffScroll > maxScroll {
		d.diffScroll = max(0, maxScroll)
	}
	if d.diffScroll < 0 {
		d.diffScroll = 0
	}
	end := min(d.diffScroll+rows, len(lines))
	return strings.Join(lines[d.diffScroll:end], "\n")
}

// styleDiffLine colours one line of a unified diff.
func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "@@"):
		// The hunk header is the only structure in a diff, so it gets the accent.
		return components.TitleStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return lipgloss.NewStyle().Foreground(components.ColorSuccess).Render(line)
	case strings.HasPrefix(line, "-"):
		return lipgloss.NewStyle().Foreground(components.ColorError).Render(line)
	default:
		return line
	}
}

// pipelineLines renders the commit's pipelines and the newest one's jobs grouped
// by stage — GitLab's row of status circles, spelled out.
// jobDuration renders a job's runtime, or its status when it has not run.
