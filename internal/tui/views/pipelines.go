package views

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// Local key constants (the tui package's key table can't be imported here
// without a cycle, so the relevant subset is duplicated).
const (
	keyEnter      = "enter"
	keyEscape     = "esc"
	keyOpenBrowse = "o"
	keyRetry      = "R"
	keyCancel     = "C"
	keyRun        = "p" // run new pipeline / play manual job
	keyCopy       = "y" // copy the selected item's identifier, as in lazygit
)

// PipelinesView is the self-contained cockpit view for pipelines, their jobs,
// and job logs.
type PipelinesView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	pipelines []gitlab.Pipeline
	cursor    int
	// pendingSHA is a commit whose pipeline should be selected as soon as the
	// pipeline list contains it, set when arriving from a commit list.
	pendingSHA string
	scroll     int // first visible row of the pipeline list, kept across frames

	viewingJobs    bool
	jobs           []gitlab.Job
	jobCursor      int
	jobScroll      int // first visible row of the job list, kept across frames
	jobTrace       string
	jobTraceScroll int

	status string
}

// NewPipelinesView creates a PipelinesView bound to the shared session context.
func NewPipelinesView(ctx *Context) *PipelinesView { return &PipelinesView{ctx: ctx} }

// Title implements View.
func (v *PipelinesView) Title() string { return "Pipelines" }

// Focus implements View: loads pipelines for the active project/branch.
// While an open job log is being viewed, do not disturb it on auto-refresh.
func (v *PipelinesView) Focus() tea.Cmd {
	if v.viewingJobs && v.jobTrace != "" {
		return nil
	}
	return v.load()
}

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *PipelinesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case PipelinesLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading pipelines: %v", msg.Err)
			return nil
		}
		v.pipelines = msg.Pipelines
		v.viewingJobs = false
		v.jobs = nil
		v.clampCursor()
		v.status = fmt.Sprintf("Loaded %d pipelines", len(msg.Pipelines))
		if sha := v.pendingSHA; sha != "" && !v.selectPendingPipeline() {
			// The commit is real but has no pipeline among those loaded — say so
			// rather than leaving the cursor somewhere arbitrary.
			v.pendingSHA = ""
			v.status = fmt.Sprintf("No pipeline for commit %s", sha)
		}
		return nil

	case JobsLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading jobs: %v", msg.Err)
			return nil
		}
		v.jobs = msg.Jobs
		v.viewingJobs = true
		if v.jobCursor >= len(v.jobs) {
			v.jobCursor = len(v.jobs) - 1
		}
		if v.jobCursor < 0 {
			v.jobCursor = 0
		}
		return nil

	case JobTraceLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading trace: %v", msg.Err)
			return nil
		}
		v.jobTrace = msg.Trace
		v.jobTraceScroll = 0
		return nil

	case JobActionDoneMsg:
		v.status = msg.Text
		if !msg.IsErr {
			return v.loadJobs()
		}
		return nil

	case PipelineActionDoneMsg:
		v.status = msg.Text
		if !msg.IsErr {
			return v.load()
		}
		return nil

	case StatusMsg:
		v.status = msg.Text
		return nil

	case ShowCommitPipelineMsg:
		// Arriving from a commit list: leave any job/log drill-down and aim at the
		// pipeline for that commit, now or as soon as the list arrives.
		v.pendingSHA = msg.ShortSHA
		v.viewingJobs = false
		v.jobs = nil
		v.jobTrace = ""
		v.selectPendingPipeline()
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// selectPendingPipeline moves the cursor onto the pipeline built for pendingSHA
// and reports whether it was found. Pipeline SHAs are full, commit SHAs short,
// so the match is by prefix.
func (v *PipelinesView) selectPendingPipeline() bool {
	if v.pendingSHA == "" {
		return true
	}
	for i, p := range v.pipelines {
		if strings.HasPrefix(p.SHA, v.pendingSHA) {
			v.cursor = i
			v.pendingSHA = ""
			return true
		}
	}
	return false
}

func (v *PipelinesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Esc: trace -> job list -> back to pipelines
	if key == keyEscape {
		if v.viewingJobs && v.jobTrace != "" {
			v.jobTrace = ""
			return nil
		}
		if v.viewingJobs {
			v.viewingJobs = false
			v.jobs = nil
			return nil
		}
		return nil
	}

	if v.viewingJobs {
		return v.handleJobViewKey(msg)
	}

	// Pipeline list navigation
	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.pipelines), listRows(v.height))
		return nil
	}

	// Enter: load jobs for the selected pipeline
	if key == keyEnter {
		if len(v.pipelines) > 0 && v.cursor < len(v.pipelines) {
			return v.loadJobs()
		}
		return nil
	}

	// Pipeline actions
	switch key {
	case keyRetry:
		if v.cursor < len(v.pipelines) {
			p := v.pipelines[v.cursor]
			return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", p.ID), v.retryPipeline())
		}
	case keyCancel:
		if v.cursor < len(v.pipelines) {
			p := v.pipelines[v.cursor]
			return confirmCmd(fmt.Sprintf("Cancel pipeline #%d?", p.ID), v.cancelPipeline())
		}
	case keyRun:
		ref := v.runRef()
		if ref == "" {
			return nil
		}
		return confirmCmd(fmt.Sprintf("Run new pipeline on %s?", ref), v.runPipeline())
	case keyOpenBrowse:
		return v.openPipelineInBrowser()
	}
	return nil
}

func (v *PipelinesView) handleJobViewKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// When a trace is loaded, navigation scrolls the log. Actions (R/C/p/o)
	// fall through to the job-action handling below.
	if v.jobTrace != "" {
		rows := listRows(v.height)
		// The log is scrolled with the same keys that move a list cursor; the end
		// is clamped by jobTraceView, which knows the wrapped line count.
		switch components.NavFor(key) {
		case components.NavDown:
			v.jobTraceScroll++
			return nil
		case components.NavUp:
			if v.jobTraceScroll > 0 {
				v.jobTraceScroll--
			}
			return nil
		case components.NavHalfDown:
			v.jobTraceScroll += rows / 2
			return nil
		case components.NavHalfUp:
			v.jobTraceScroll = max(0, v.jobTraceScroll-rows/2)
			return nil
		case components.NavPageDown:
			v.jobTraceScroll += rows
			return nil
		case components.NavPageUp:
			v.jobTraceScroll = max(0, v.jobTraceScroll-rows)
			return nil
		case components.NavTop:
			v.jobTraceScroll = 0
			return nil
		case components.NavBottom:
			v.jobTraceScroll = len(v.jobTrace)
			return nil
		}
	}

	// Job list navigation. Moving off a job drops the log shown beside it.
	if act := components.NavFor(key); act != components.NavNone {
		if moved := components.ApplyNav(act, v.jobCursor, len(v.jobs), listRows(v.height)); moved != v.jobCursor {
			v.jobCursor = moved
			v.jobTrace = ""
		}
		return nil
	}

	switch key {
	case keyEnter:
		return v.loadJobTrace()
	case keyOpenBrowse:
		if v.jobCursor < len(v.jobs) {
			cmd := openBrowserCmd(v.jobs[v.jobCursor].WebURL)
			if cmd != nil {
				return execBrowser(cmd)
			}
		}
		return nil
	case keyRetry:
		if v.jobCursor < len(v.jobs) {
			job := v.jobs[v.jobCursor]
			return confirmCmd(fmt.Sprintf("Retry job '%s'?", components.Truncate(job.Name, 30)), v.retryJob())
		}
	case keyCancel:
		if v.jobCursor < len(v.jobs) {
			job := v.jobs[v.jobCursor]
			return confirmCmd(fmt.Sprintf("Cancel job '%s'?", components.Truncate(job.Name, 30)), v.cancelJob())
		}
	case keyRun: // "p" plays a manual job in the job view
		if v.jobCursor < len(v.jobs) {
			job := v.jobs[v.jobCursor]
			return confirmCmd(fmt.Sprintf("Play job '%s'?", components.Truncate(job.Name, 30)), v.playJob())
		}
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail/trace panel on the right.
func (v *PipelinesView) Body(width, height int) string {
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

	// Left list. Pipelines and jobs are separate lists, so each keeps its own
	// scroll position rather than inheriting the other's.
	var (
		listTitle string
		items     []string
		cursor    int
		scroll    *int
	)
	if v.viewingJobs {
		listTitle = v.jobsTitle()
		var jobToDisplay []int
		items, jobToDisplay = v.jobItems()
		if v.jobCursor >= 0 && v.jobCursor < len(jobToDisplay) {
			cursor = jobToDisplay[v.jobCursor]
		}
		scroll = &v.jobScroll
	} else {
		listTitle = "Pipelines"
		items = v.pipelineItems()
		cursor = v.cursor
		scroll = &v.scroll
	}
	left := renderListBox(leftWidth, height, listTitle, items, cursor, scroll)

	// Right detail.
	var detail string
	if v.viewingJobs {
		detail = v.jobDetail(rightWidth, height)
	} else {
		detail = v.pipelineDetail()
	}
	if detail == "" {
		detail = "Select an item to view details"
	}
	borderColor := components.ColorSecondary
	if v.viewingJobs && v.jobTrace != "" {
		borderColor = components.ColorPrimary
	}
	right := components.RenderBox(v.detailTitle(), strings.Split(detail, "\n"), rightWidth, height, borderColor, components.ColorPrimary)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *PipelinesView) detailTitle() string {
	if v.viewingJobs {
		if v.jobCursor < len(v.jobs) {
			job := v.jobs[v.jobCursor]
			if v.jobTrace != "" {
				return fmt.Sprintf("Log: %s", job.Name)
			}
			return fmt.Sprintf("Job: %s", job.Name)
		}
		return "Job"
	}
	if idx := v.cursor; idx < len(v.pipelines) {
		return fmt.Sprintf("Pipeline (#%d)", v.pipelines[idx].ID)
	}
	return "Pipeline"
}

func (v *PipelinesView) jobsTitle() string {
	if idx := v.cursor; idx < len(v.pipelines) {
		return fmt.Sprintf("Jobs (#%d)", v.pipelines[idx].ID)
	}
	return "Jobs"
}

// pipelineItems renders the pipeline list rows (no ref column).
func (v *PipelinesView) pipelineItems() []string {
	items := make([]string, len(v.pipelines))
	for i, p := range v.pipelines {
		title := p.CommitTitle
		if title == "" {
			title = p.Ref
		}
		items[i] = fmt.Sprintf("%s %s %s",
			util.TimeAgoShort(p.CreatedAt),
			components.StatusIconPadded(p.Status),
			title,
		)
	}
	return items
}

// jobItems returns display lines for jobs grouped by stage, plus a mapping from
// job index to display-row index (accounting for stage header lines). Header
// lines carry a leading "\x00" marker so renderListBox skips highlighting.
func (v *PipelinesView) jobItems() ([]string, []int) {
	var items []string
	jobToDisplay := make([]int, len(v.jobs))
	currentStage := ""
	for i, job := range v.jobs {
		if job.Stage != currentStage {
			currentStage = job.Stage
			header := lipgloss.NewStyle().Bold(true).Foreground(components.ColorSecondary).Render(job.Stage)
			items = append(items, "\x00"+header)
		}
		jobToDisplay[i] = len(items)
		icon := components.StatusIcon(job.Status)
		duration := ""
		if job.Duration > 0 {
			mins := int(job.Duration) / 60
			secs := int(job.Duration) % 60
			if mins > 0 {
				duration = fmt.Sprintf(" (%dm%ds)", mins, secs)
			} else {
				duration = fmt.Sprintf(" (%ds)", secs)
			}
		}
		items = append(items, fmt.Sprintf("  %s %s  %s%s", icon, job.Name, job.Status, duration))
	}
	return items, jobToDisplay
}

func (v *PipelinesView) pipelineDetail() string {
	if len(v.pipelines) == 0 {
		return "No pipelines"
	}
	if v.cursor >= len(v.pipelines) {
		return ""
	}
	p := v.pipelines[v.cursor]

	var lines []string
	lines = append(lines,
		fmt.Sprintf("Status:  %s %s",
			components.StatusIcon(p.Status),
			lipgloss.NewStyle().Foreground(components.StatusColor(p.Status)).Render(p.Status),
		),
	)
	lines = append(lines, fmt.Sprintf("Ref:     %s", p.Ref))
	if p.CommitTitle != "" {
		lines = append(lines, fmt.Sprintf("Commit:  %s", p.CommitTitle))
	}
	if !p.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", util.TimeAgo(p.CreatedAt)))
	}
	if !p.UpdatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Updated: %s", util.TimeAgo(p.UpdatedAt)))
	}
	lines = append(lines, "")
	lines = append(lines, components.HelpDescStyle.Render(p.WebURL))
	return strings.Join(lines, "\n")
}

func (v *PipelinesView) jobDetail(width, height int) string {
	if len(v.jobs) == 0 {
		return "No jobs"
	}
	if v.jobCursor >= len(v.jobs) {
		return ""
	}

	// Show trace log if loaded.
	if v.jobTrace != "" {
		return v.jobTraceView(width, height)
	}

	job := v.jobs[v.jobCursor]
	coloredStatus := lipgloss.NewStyle().Foreground(components.StatusColor(job.Status)).Render(job.Status)

	var lines []string
	lines = append(lines, fmt.Sprintf("Status:   %s %s", components.StatusIcon(job.Status), coloredStatus))
	lines = append(lines, fmt.Sprintf("Stage:    %s", job.Stage))
	if job.Duration > 0 {
		mins := int(job.Duration) / 60
		secs := int(job.Duration) % 60
		if mins > 0 {
			lines = append(lines, fmt.Sprintf("Duration: %dm%ds", mins, secs))
		} else {
			lines = append(lines, fmt.Sprintf("Duration: %ds", secs))
		}
	}
	if !job.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created:  %s", util.TimeAgo(job.CreatedAt)))
	}
	if !job.StartedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Started:  %s", util.TimeAgo(job.StartedAt)))
	}
	lines = append(lines, "")
	lines = append(lines, components.HelpDescStyle.Render("Press Enter to view log"))
	return strings.Join(lines, "\n")
}

func (v *PipelinesView) jobTraceView(width, height int) string {
	contentWidth := width - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	viewHeight := height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	// Clean GitLab trace: strip ANSI, carriage returns, section markers.
	var cleaned []string
	for _, line := range strings.Split(v.jobTrace, "\n") {
		line = ansi.Strip(line)
		line = strings.ReplaceAll(line, "\r", "")
		if strings.HasPrefix(line, "section_start:") || strings.HasPrefix(line, "section_end:") {
			continue
		}
		cleaned = append(cleaned, components.WrapLine(line, contentWidth)...)
	}

	// Clamp scroll based on cleaned lines.
	maxScroll := len(cleaned) - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if v.jobTraceScroll > maxScroll {
		v.jobTraceScroll = maxScroll
	}

	start := v.jobTraceScroll
	if start < 0 {
		start = 0
	}
	end := start + viewHeight
	if end > len(cleaned) {
		end = len(cleaned)
	}
	return strings.Join(cleaned[start:end], "\n")
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *PipelinesView) KeyHints() []KeyHint {
	if v.viewingJobs {
		return []KeyHint{
			{"Enter", "Log"},
			{"R", "Retry job"},
			{"C", "Cancel job"},
			{"p", "Play"},
			{"o", "Open"},
			{"Esc", "Back"},
		}
	}
	return []KeyHint{
		{"Enter", "Jobs"},
		{"p", "Run"},
		{"R", "Retry"},
		{"C", "Cancel"},
		{"o", "Open"},
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *PipelinesView) load() tea.Cmd {
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

func (v *PipelinesView) loadJobs() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.pipelines) == 0 {
		return nil
	}
	if v.cursor >= len(v.pipelines) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := v.pipelines[v.cursor].ID
	return func() tea.Msg {
		jobs, err := client.ListPipelineJobs(projectID, pipelineID)
		return JobsLoadedMsg{Jobs: jobs, Err: err}
	}
}

func (v *PipelinesView) loadJobTrace() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.jobs) == 0 {
		return nil
	}
	if v.jobCursor >= len(v.jobs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	jobID := v.jobs[v.jobCursor].ID
	return func() tea.Msg {
		trace, err := client.GetJobTrace(projectID, jobID)
		return JobTraceLoadedMsg{Trace: trace, Err: err}
	}
}

func (v *PipelinesView) retryPipeline() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.pipelines) == 0 {
		return nil
	}
	if v.cursor >= len(v.pipelines) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := v.pipelines[v.cursor].ID
	return func() tea.Msg {
		if err := client.RetryPipeline(projectID, pipelineID); err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Retried pipeline #%d", pipelineID)}
	}
}

func (v *PipelinesView) cancelPipeline() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.pipelines) == 0 {
		return nil
	}
	if v.cursor >= len(v.pipelines) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := v.pipelines[v.cursor].ID
	return func() tea.Msg {
		if err := client.CancelPipeline(projectID, pipelineID); err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Cancel failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Canceled pipeline #%d", pipelineID)}
	}
}

// runRef returns the ref a new pipeline would run on (selected branch or the
// project default), or "" when unavailable.
func (v *PipelinesView) runRef() string {
	if v.ctx == nil || v.ctx.Project == nil {
		return ""
	}
	if v.ctx.Branch != nil {
		return v.ctx.Branch.Name
	}
	return v.ctx.Project.DefaultBranch
}

func (v *PipelinesView) runPipeline() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	ref := v.runRef()
	return func() tea.Msg {
		p, err := client.RunPipeline(projectID, ref)
		if err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Run failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Pipeline #%d started on %s", p.ID, ref)}
	}
}

func (v *PipelinesView) retryJob() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.jobs) == 0 {
		return nil
	}
	if v.jobCursor >= len(v.jobs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	job := v.jobs[v.jobCursor]
	return func() tea.Msg {
		if err := client.RetryJob(projectID, job.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Retry job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Retried job '%s'", job.Name)}
	}
}

func (v *PipelinesView) cancelJob() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.jobs) == 0 {
		return nil
	}
	if v.jobCursor >= len(v.jobs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	job := v.jobs[v.jobCursor]
	return func() tea.Msg {
		if err := client.CancelJob(projectID, job.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Cancel job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Canceled job '%s'", job.Name)}
	}
}

func (v *PipelinesView) playJob() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.jobs) == 0 {
		return nil
	}
	if v.jobCursor >= len(v.jobs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	job := v.jobs[v.jobCursor]
	if job.Status != "manual" {
		return func() tea.Msg {
			return StatusMsg{Text: "Only manual jobs can be played", IsErr: true}
		}
	}
	return func() tea.Msg {
		if err := client.PlayJob(projectID, job.ID); err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Play job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Playing job '%s'", job.Name)}
	}
}

func (v *PipelinesView) openPipelineInBrowser() tea.Cmd {
	if v.cursor >= len(v.pipelines) {
		return nil
	}
	cmd := openBrowserCmd(v.pipelines[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *PipelinesView) clampCursor() {
	n := len(v.pipelines)
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}

// confirmCmd returns a command that asks the shell to confirm a destructive
// action before running it.
func confirmCmd(prompt string, action tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		return ConfirmMsg{Prompt: prompt, Action: action}
	}
}

// execBrowser runs an openBrowserCmd and reports failures via StatusMsg.
func execBrowser(cmd *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Failed to open browser: %v", err), IsErr: true}
		}
		return nil
	})
}

// listRows is how many list rows a view of the given height shows, i.e. the box
// height minus its borders. It is the page size for navigation.
func listRows(height int) int {
	rows := height - 2
	if rows < 1 {
		rows = 1
	}
	return rows
}
