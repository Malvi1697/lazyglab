package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// jobsPanel is a pipeline's jobs, navigable and actionable: a stage-grouped list
// with a cursor, the selected job's detail or log, and the actions GitLab allows
// on a job (retry, cancel, play a manual one).
//
// It is a component rather than part of a view so a pipeline can be driven from
// anywhere it is shown — the Pipelines view and the commit page today.
type jobsPanel struct {
	ctx *Context

	pipelineID int
	jobs       []gitlab.Job
	cursor     int
	scroll     int

	trace       string
	traceScroll int
}

// active reports whether the panel is showing a pipeline.
func (p *jobsPanel) active() bool { return p.pipelineID != 0 }

// showingTrace reports whether a job log is open.
func (p *jobsPanel) showingTrace() bool { return p.trace != "" }

// open points the panel at a pipeline and loads its jobs.
func (p *jobsPanel) open(pipelineID int) tea.Cmd {
	if pipelineID == 0 {
		return nil
	}
	if pipelineID != p.pipelineID {
		p.jobs = nil
		p.cursor = 0
		p.scroll = 0
	}
	p.pipelineID = pipelineID
	p.trace = ""
	p.traceScroll = 0
	return p.load()
}

// close forgets the pipeline.
func (p *jobsPanel) close() {
	p.pipelineID = 0
	p.jobs = nil
	p.cursor = 0
	p.scroll = 0
	p.trace = ""
	p.traceScroll = 0
}

// closeTrace closes an open log, staying on the job list.
func (p *jobsPanel) closeTrace() {
	p.trace = ""
	p.traceScroll = 0
}

// setJobs absorbs a loaded job list, keeping the cursor in range.
func (p *jobsPanel) setJobs(jobs []gitlab.Job) {
	p.jobs = jobs
	if p.cursor >= len(p.jobs) {
		p.cursor = len(p.jobs) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// setTrace absorbs a loaded log.
func (p *jobsPanel) setTrace(trace string) {
	p.trace = trace
	p.traceScroll = 0
}

// selected returns the highlighted job, or nil.
func (p *jobsPanel) selected() *gitlab.Job {
	if p.cursor < 0 || p.cursor >= len(p.jobs) {
		return nil
	}
	return &p.jobs[p.cursor]
}

// ============================================================================
// Keys
// ============================================================================

// handleKey drives the panel and reports whether the key was consumed, so the
// host view can fall back to its own bindings.
func (p *jobsPanel) handleKey(key string, height int) (tea.Cmd, bool) {
	// With a log open, navigation scrolls it; the actions below still apply to
	// the job the log belongs to.
	if p.trace != "" {
		if p.scrollTrace(key, height) {
			return nil, true
		}
	}

	// Job list navigation. Only reached with no log open: an open log takes the
	// navigation keys for itself, and Esc closes it.
	if act := components.NavFor(key); act != components.NavNone {
		p.cursor = components.ApplyNav(act, p.cursor, len(p.jobs), listRows(height))
		return nil, true
	}

	job := p.selected()
	switch key {
	case keyEnter:
		if job == nil {
			return nil, true
		}
		return p.loadTrace(), true
	case keyOpenBrowse:
		if job == nil {
			return nil, true
		}
		if cmd := openBrowserCmd(job.WebURL); cmd != nil {
			return execBrowser(cmd), true
		}
		return nil, true
	case keyRetry:
		if job == nil {
			return nil, true
		}
		return confirmCmd(fmt.Sprintf("Retry job '%s'?", components.Truncate(job.Name, 30)), p.retryJob()), true
	case keyCancel:
		if job == nil {
			return nil, true
		}
		return confirmCmd(fmt.Sprintf("Cancel job '%s'?", components.Truncate(job.Name, 30)), p.cancelJob()), true
	case keyRun: // "p" plays a manual job
		if job == nil {
			return nil, true
		}
		return confirmCmd(fmt.Sprintf("Play job '%s'?", components.Truncate(job.Name, 30)), p.playJob()), true
	}
	return nil, false
}

// scrollTrace moves through an open log, using the same keys that move a list
// cursor. Reports whether the key was a scroll.
func (p *jobsPanel) scrollTrace(key string, height int) bool {
	rows := listRows(height)
	switch components.NavFor(key) {
	case components.NavDown:
		p.traceScroll++
	case components.NavUp:
		p.traceScroll = max(0, p.traceScroll-1)
	case components.NavHalfDown:
		p.traceScroll += rows / 2
	case components.NavHalfUp:
		p.traceScroll = max(0, p.traceScroll-rows/2)
	case components.NavPageDown:
		p.traceScroll += rows
	case components.NavPageUp:
		p.traceScroll = max(0, p.traceScroll-rows)
	case components.NavTop:
		p.traceScroll = 0
	case components.NavBottom:
		// Clamped while rendering, which knows the wrapped line count.
		p.traceScroll = len(p.trace)
	default:
		return false
	}
	return true
}

// keyHints are the panel's footer hints.
func (p *jobsPanel) keyHints() []KeyHint {
	if p.trace != "" {
		return []KeyHint{
			{"Esc", "Back"},
			{"j/k", "Scroll"},
			{"R", "Retry"},
			{"C", "Cancel"},
			{"o", "Open"},
		}
	}
	return []KeyHint{
		{"Enter", "Log"},
		{"R", "Retry"},
		{"C", "Cancel"},
		{"p", "Play"},
		{"o", "Open"},
		{"Esc", "Back"},
	}
}

// ============================================================================
// Commands
// ============================================================================

func (p *jobsPanel) load() tea.Cmd {
	if p.ctx == nil || p.ctx.Project == nil || p.ctx.Client == nil || p.pipelineID == 0 {
		return nil
	}
	client := p.ctx.Client
	projectID := p.ctx.Project.ID
	pipelineID := p.pipelineID
	return func() tea.Msg {
		jobs, err := client.ListPipelineJobs(projectID, pipelineID)
		return JobsLoadedMsg{Jobs: jobs, Err: err}
	}
}

func (p *jobsPanel) loadTrace() tea.Cmd {
	job := p.selected()
	if job == nil || p.ctx == nil || p.ctx.Project == nil || p.ctx.Client == nil {
		return nil
	}
	client := p.ctx.Client
	projectID := p.ctx.Project.ID
	jobID := job.ID
	return func() tea.Msg {
		trace, err := client.GetJobTrace(projectID, jobID)
		return JobTraceLoadedMsg{Trace: trace, Err: err}
	}
}

func (p *jobsPanel) retryJob() tea.Cmd {
	job := p.selected()
	if job == nil || p.ctx == nil || p.ctx.Project == nil || p.ctx.Client == nil {
		return nil
	}
	client := p.ctx.Client
	projectID := p.ctx.Project.ID
	j := *job
	return func() tea.Msg {
		if err := client.RetryJob(projectID, j.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Retry job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Retried job '%s'", j.Name)}
	}
}

func (p *jobsPanel) cancelJob() tea.Cmd {
	job := p.selected()
	if job == nil || p.ctx == nil || p.ctx.Project == nil || p.ctx.Client == nil {
		return nil
	}
	client := p.ctx.Client
	projectID := p.ctx.Project.ID
	j := *job
	return func() tea.Msg {
		if err := client.CancelJob(projectID, j.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Cancel job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Canceled job '%s'", j.Name)}
	}
}

func (p *jobsPanel) playJob() tea.Cmd {
	job := p.selected()
	if job == nil {
		return nil
	}
	// Only a manual job can be played. This is checked before the client, so the
	// explanation reaches the user either way.
	if job.Status != "manual" {
		return func() tea.Msg {
			return StatusMsg{Text: "Only manual jobs can be played", IsErr: true}
		}
	}
	if p.ctx == nil || p.ctx.Project == nil || p.ctx.Client == nil {
		return nil
	}
	client := p.ctx.Client
	projectID := p.ctx.Project.ID
	j := *job
	return func() tea.Msg {
		if err := client.PlayJob(projectID, j.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Play job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Playing job '%s'", j.Name)}
	}
}

// ============================================================================
// Rendering
// ============================================================================

// items renders the job rows grouped by stage, plus the mapping from job index
// to display row (stage headers occupy rows of their own).
func (p *jobsPanel) items() ([]string, []int) {
	var items []string
	jobToDisplay := make([]int, len(p.jobs))
	stage := ""
	for i, job := range p.jobs {
		if job.Stage != stage {
			stage = job.Stage
			header := lipgloss.NewStyle().Bold(true).Foreground(components.ColorSecondary).Render(job.Stage)
			items = append(items, "\x00"+header)
		}
		jobToDisplay[i] = len(items)
		items = append(items, fmt.Sprintf("  %s %s  %s%s",
			components.StatusIcon(job.Status), job.Name, job.Status, durationSuffix(&job)))
	}
	return items, jobToDisplay
}

// cursorRow is the display row of the selected job.
func (p *jobsPanel) cursorRow() int {
	_, jobToDisplay := p.items()
	if p.cursor >= 0 && p.cursor < len(jobToDisplay) {
		return jobToDisplay[p.cursor]
	}
	return 0
}

// listBox renders the job list as a bordered, scrollable box.
func (p *jobsPanel) listBox(width, height int, title string) string {
	items, _ := p.items()
	return renderListBox(width, height, title, items, p.cursorRow(), &p.scroll)
}

// listTitle names the job list. It stays the job list even with a log open, so
// the two boxes are not both titled after the log.
func (p *jobsPanel) listTitle() string {
	return fmt.Sprintf("Jobs (#%d)", p.pipelineID)
}

// detailTitle names the pane beside the list: the selected job, or its log.
func (p *jobsPanel) detailTitle() string {
	job := p.selected()
	switch {
	case p.trace != "" && job != nil:
		return "Log: " + job.Name
	case p.trace != "":
		return "Log"
	case job != nil:
		return "Job: " + job.Name
	}
	return "Job"
}

// detail renders the selected job — its log when one is open, otherwise its
// particulars.
func (p *jobsPanel) detail(width, height int) string {
	if len(p.jobs) == 0 {
		return "No jobs"
	}
	job := p.selected()
	if job == nil {
		return ""
	}
	if p.trace != "" {
		return p.traceView(width, height)
	}

	status := lipgloss.NewStyle().Foreground(components.StatusColor(job.Status)).Render(job.Status)
	lines := []string{
		fmt.Sprintf("Status:   %s %s", components.StatusIcon(job.Status), status),
		fmt.Sprintf("Stage:    %s", job.Stage),
	}
	if job.Duration > 0 {
		lines = append(lines, "Duration: "+strings.Trim(durationSuffix(job), " ()"))
	}
	if !job.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created:  %s", util.TimeAgo(job.CreatedAt)))
	}
	if !job.StartedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Started:  %s", util.TimeAgo(job.StartedAt)))
	}
	lines = append(lines, "", components.HelpDescStyle.Render("Press Enter to view log"))
	return strings.Join(lines, "\n")
}

// traceView renders the open log, cleaned of ANSI and GitLab's section markers.
func (p *jobsPanel) traceView(width, height int) string {
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	viewHeight := height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	var cleaned []string
	for _, line := range strings.Split(p.trace, "\n") {
		line = ansi.Strip(line)
		line = strings.ReplaceAll(line, "\r", "")
		if strings.HasPrefix(line, "section_start:") || strings.HasPrefix(line, "section_end:") {
			continue
		}
		cleaned = append(cleaned, components.WrapLine(line, contentWidth)...)
	}

	if maxScroll := len(cleaned) - viewHeight; p.traceScroll > maxScroll {
		p.traceScroll = max(0, maxScroll)
	}
	if p.traceScroll < 0 {
		p.traceScroll = 0
	}
	end := min(p.traceScroll+viewHeight, len(cleaned))
	return strings.Join(cleaned[p.traceScroll:end], "\n")
}

// body renders the panel on its own: the job list beside the selected job's
// detail or log. Used where the panel is the whole screen.
func (p *jobsPanel) body(width, height int) string {
	leftWidth := width * 45 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}

	left := p.listBox(leftWidth, height, p.listTitle())
	detail := p.detail(width-leftWidth, height)
	if detail == "" {
		detail = "Select a job"
	}
	// A log open means the pane is the thing being read, so give it the accent.
	border := components.ColorSecondary
	if p.trace != "" {
		border = components.ColorPrimary
	}
	right := components.RenderBox(p.detailTitle(), splitLines(detail), width-leftWidth, height,
		border, components.ColorPrimary)
	return joinH(left, right)
}

// durationSuffix renders " (1m22s)" for a job that ran, or "" for one that has not.
func durationSuffix(job *gitlab.Job) string {
	if job.Duration <= 0 {
		return ""
	}
	secs := int(job.Duration)
	if secs < 60 {
		return fmt.Sprintf(" (%ds)", secs)
	}
	return fmt.Sprintf(" (%dm%ds)", secs/60, secs%60)
}
