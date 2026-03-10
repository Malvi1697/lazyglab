package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// App is the root Bubble Tea model.
type App struct {
	// GitLab clients per host
	clients    map[string]*gitlab.Client
	hostNames  []string
	activeHost string

	// Active project
	activeProject *gitlab.Project

	// Auto-detected project from git remote
	detectedHost string
	detectedPath string

	// Data
	projects  []gitlab.Project
	mrs       []gitlab.MergeRequest
	pipelines []gitlab.Pipeline
	issues    []gitlab.Issue
	jobs      []gitlab.Job // jobs for selected pipeline
	branches  []gitlab.Branch

	// Branch filter
	activeBranch *gitlab.Branch // nil = all branches

	// UI state
	activePanel PanelID
	cursor      [4]int // cursor position per panel
	layout      Layout
	showHelp    bool
	statusText  string
	statusIsErr bool
	loading     bool

	// Detail/overlay state
	viewingJobs      bool // true when viewing pipeline jobs in detail panel
	jobCursor        int
	jobTrace         string // log output for selected job
	jobTraceScroll   int    // scroll offset for job trace
	showBranchPicker bool
	branchCursor     int

	// Confirmation dialog
	pendingConfirm *confirmAction // non-nil when confirmation dialog is shown

	// Dimensions
	width  int
	height int
}

// confirmAction holds state for a pending confirmation dialog.
type confirmAction struct {
	prompt string  // e.g. "Merge !123?"
	action tea.Cmd // command to execute on confirm
}

// NewApp creates the root application model.
func NewApp(clients map[string]*gitlab.Client, hostNames []string, detectedHost, detectedPath string) *App {
	activeHost := ""
	if detectedHost != "" {
		activeHost = detectedHost
	} else if len(hostNames) > 0 {
		activeHost = hostNames[0]
	}
	return &App{
		clients:      clients,
		hostNames:    hostNames,
		activeHost:   activeHost,
		activePanel:  PanelPipelines,
		detectedHost: detectedHost,
		detectedPath: detectedPath,
	}
}

func (a *App) Init() tea.Cmd {
	return a.loadProjects()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	case tea.KeyMsg:
		// Confirmation dialog takes highest precedence
		if a.pendingConfirm != nil {
			return a.handleConfirmKey(msg)
		}

		// Help overlay takes precedence
		if a.showHelp {
			a.showHelp = false
			return a, nil
		}

		// Branch picker takes precedence
		if a.showBranchPicker {
			return a.handleBranchPickerKey(msg)
		}

		return a.handleKeyMsg(msg)

	// Async data messages
	case ProjectsLoadedMsg:
		a.loading = false
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading projects: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.projects = msg.Projects
		a.statusText = fmt.Sprintf("Loaded %d projects", len(msg.Projects))
		a.statusIsErr = false

		// Auto-select project from git remote detection
		if a.detectedPath != "" && a.activeProject == nil {
			for i, p := range a.projects {
				if strings.EqualFold(p.PathWithNamespace, a.detectedPath) {
					a.cursor[PanelProjects] = i
					a.detectedPath = "" // clear so it doesn't re-trigger
					return a, func() tea.Msg {
						return ProjectSelectedMsg{Project: p}
					}
				}
			}
		}

		return a, nil

	case ProjectSelectedMsg:
		a.activeProject = &msg.Project
		a.activeBranch = nil // reset branch filter
		a.statusText = fmt.Sprintf("Selected: %s", msg.Project.NameWithNamespace)
		a.statusIsErr = false
		return a, tea.Batch(
			a.loadMRs(),
			a.loadPipelines(),
			a.loadIssues(),
		)

	case MRsLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading MRs: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.mrs = msg.MRs
		return a, nil

	case PipelinesLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading pipelines: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.pipelines = msg.Pipelines
		a.viewingJobs = false
		a.jobs = nil
		return a, nil

	case JobsLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading jobs: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.jobs = msg.Jobs
		a.viewingJobs = true
		// Clamp cursor to new list (preserve position on refresh)
		if a.jobCursor >= len(a.jobs) {
			a.jobCursor = len(a.jobs) - 1
		}
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		return a, nil

	case IssuesLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading issues: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.issues = msg.Issues
		return a, nil

	case BranchesLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading branches: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.branches = msg.Branches
		a.showBranchPicker = true
		a.branchCursor = 0
		return a, nil

	case BranchSelectedMsg:
		a.showBranchPicker = false
		a.activeBranch = &msg.Branch
		a.cursor[PanelPipelines] = 0
		a.statusText = fmt.Sprintf("Branch: %s", msg.Branch.Name)
		a.statusIsErr = false
		return a, a.loadPipelines()

	case JobTraceLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading trace: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.jobTrace = msg.Trace
		a.jobTraceScroll = 0
		return a, nil

	case JobActionDoneMsg:
		a.statusText = msg.Text
		a.statusIsErr = msg.IsErr
		if !msg.IsErr {
			return a, a.loadJobs()
		}
		return a, nil

	case PipelineActionDoneMsg:
		a.statusText = msg.Text
		a.statusIsErr = msg.IsErr
		if !msg.IsErr {
			return a, a.loadPipelines()
		}
		return a, nil

	case StatusMsg:
		a.statusText = msg.Text
		a.statusIsErr = msg.IsErr
		return a, nil
	}

	return a, nil
}

func (a *App) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case KeyQuit, "ctrl+c":
		return a, tea.Quit
	case KeyHelp:
		a.showHelp = !a.showHelp
		return a, nil
	case KeyTab, KeyVimRight:
		a.activePanel = (a.activePanel + 1) % 4
		return a, nil
	case KeyShiftTab, KeyVimLeft:
		a.activePanel = (a.activePanel + 3) % 4
		return a, nil
	case KeyPanel1:
		a.activePanel = PanelProjects
		return a, nil
	case KeyPanel2:
		a.activePanel = PanelPipelines
		return a, nil
	case KeyPanel3:
		a.activePanel = PanelMergeRequests
		return a, nil
	case KeyPanel4:
		a.activePanel = PanelIssues
		return a, nil
	case KeyRefresh:
		return a, a.refreshActivePanel()
	case KeyBranch:
		return a, a.loadBranches()
	}

	// Escape: close trace → close job view → clear branch filter
	if key == KeyEscape {
		if a.viewingJobs && a.jobTrace != "" {
			a.jobTrace = ""
			return a, nil
		}
		if a.viewingJobs {
			a.viewingJobs = false
			a.jobs = nil
			return a, nil
		}
		if a.activeBranch != nil {
			a.activeBranch = nil
			a.statusText = "Branch filter cleared"
			a.statusIsErr = false
			a.cursor[PanelPipelines] = 0
			return a, a.loadPipelines()
		}
		return a, nil
	}

	// When viewing jobs in the detail panel, handle job navigation
	if a.viewingJobs && a.activePanel == PanelPipelines {
		return a.handleJobViewKey(msg)
	}

	// Navigation keys
	if isNavigateUp(msg) {
		a.moveCursor(-1)
		return a, nil
	}
	if isNavigateDown(msg) {
		a.moveCursor(1)
		return a, nil
	}
	if key == KeyTop {
		a.cursor[a.activePanel] = 0
		return a, nil
	}
	if key == KeyBottom {
		a.cursor[a.activePanel] = a.activeListLen() - 1
		if a.cursor[a.activePanel] < 0 {
			a.cursor[a.activePanel] = 0
		}
		return a, nil
	}
	if key == KeyHalfDown {
		halfPage := (a.layout.PanelHeights[a.activePanel] - 2) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.moveCursor(halfPage)
		return a, nil
	}
	if key == KeyHalfUp {
		halfPage := (a.layout.PanelHeights[a.activePanel] - 2) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.moveCursor(-halfPage)
		return a, nil
	}

	// Enter: select item
	if key == KeyEnter {
		return a, a.handleEnter()
	}

	// Panel-specific keys
	return a.handlePanelKey(key)
}

func (a *App) handleJobViewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// When trace is loaded, all navigation scrolls the log.
	// jobTraceView() handles max-scroll clamping after cleaning/wrapping.
	if a.jobTrace != "" {
		traceHeight := a.layout.ContentHeight - 2
		if traceHeight < 1 {
			traceHeight = 1
		}

		switch {
		case key == KeyEscape:
			a.jobTrace = ""
			return a, nil
		case isNavigateDown(msg):
			a.jobTraceScroll++
			return a, nil
		case isNavigateUp(msg):
			if a.jobTraceScroll > 0 {
				a.jobTraceScroll--
			}
			return a, nil
		case key == KeyHalfDown:
			a.jobTraceScroll += traceHeight / 2
			return a, nil
		case key == KeyHalfUp:
			a.jobTraceScroll -= traceHeight / 2
			if a.jobTraceScroll < 0 {
				a.jobTraceScroll = 0
			}
			return a, nil
		case key == KeyTop:
			a.jobTraceScroll = 0
			return a, nil
		case key == KeyBottom:
			a.jobTraceScroll = len(a.jobTrace) // will be clamped by jobTraceView
			return a, nil
		}
		// Other keys (R, C, p, o) fall through to job actions below
	}

	if isNavigateUp(msg) {
		if a.jobCursor > 0 {
			a.jobCursor--
			a.jobTrace = ""
		}
		return a, nil
	}
	if isNavigateDown(msg) {
		if a.jobCursor < len(a.jobs)-1 {
			a.jobCursor++
			a.jobTrace = ""
		}
		return a, nil
	}
	if key == KeyTop {
		a.jobCursor = 0
		a.jobTrace = ""
		return a, nil
	}
	if key == KeyBottom {
		a.jobCursor = len(a.jobs) - 1
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		a.jobTrace = ""
		return a, nil
	}

	switch key {
	case KeyHalfDown:
		halfPage := (a.layout.PanelHeights[PanelPipelines] - 2) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.jobCursor += halfPage
		if a.jobCursor >= len(a.jobs) {
			a.jobCursor = len(a.jobs) - 1
		}
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		a.jobTrace = ""
		return a, nil
	case KeyHalfUp:
		halfPage := (a.layout.PanelHeights[PanelPipelines] - 2) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.jobCursor -= halfPage
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		a.jobTrace = ""
		return a, nil
	case KeyEnter:
		return a, a.loadJobTrace()
	case KeyOpenBrowse:
		if a.jobCursor < len(a.jobs) {
			url := a.jobs[a.jobCursor].WebURL
			cmd := openBrowserCmd(url)
			if cmd != nil {
				return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
					if err != nil {
						return StatusMsg{Text: fmt.Sprintf("Failed to open browser: %v", err), IsErr: true}
					}
					return nil
				})
			}
		}
		return a, nil
	case KeyRetry:
		if a.jobCursor < len(a.jobs) {
			job := a.jobs[a.jobCursor]
			a.confirm(fmt.Sprintf("Retry job '%s'?", truncate(job.Name, 30)), a.retryJob())
			return a, nil
		}
	case KeyCancel:
		if a.jobCursor < len(a.jobs) {
			job := a.jobs[a.jobCursor]
			a.confirm(fmt.Sprintf("Cancel job '%s'?", truncate(job.Name, 30)), a.cancelJob())
			return a, nil
		}
	case KeyPlayJob:
		if a.jobCursor < len(a.jobs) {
			job := a.jobs[a.jobCursor]
			a.confirm(fmt.Sprintf("Play job '%s'?", truncate(job.Name, 30)), a.playJob())
			return a, nil
		}
	}

	return a, nil
}

func (a *App) handleBranchPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch {
	case key == KeyEscape || key == KeyQuit:
		a.showBranchPicker = false
		return a, nil
	case isNavigateUp(msg):
		if a.branchCursor > 0 {
			a.branchCursor--
		}
		return a, nil
	case isNavigateDown(msg):
		if a.branchCursor < len(a.branches)-1 {
			a.branchCursor++
		}
		return a, nil
	case key == KeyTop:
		a.branchCursor = 0
		return a, nil
	case key == KeyBottom:
		a.branchCursor = len(a.branches) - 1
		if a.branchCursor < 0 {
			a.branchCursor = 0
		}
		return a, nil
	case key == KeyHalfDown:
		halfPage := (a.layout.ContentHeight - 4) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.branchCursor += halfPage
		if a.branchCursor >= len(a.branches) {
			a.branchCursor = len(a.branches) - 1
		}
		if a.branchCursor < 0 {
			a.branchCursor = 0
		}
		return a, nil
	case key == KeyHalfUp:
		halfPage := (a.layout.ContentHeight - 4) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.branchCursor -= halfPage
		if a.branchCursor < 0 {
			a.branchCursor = 0
		}
		return a, nil
	case key == KeyEnter:
		if a.branchCursor < len(a.branches) {
			branch := a.branches[a.branchCursor]
			return a, func() tea.Msg {
				return BranchSelectedMsg{Branch: branch}
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "y", "Y", KeyEnter:
		action := a.pendingConfirm.action
		a.pendingConfirm = nil
		return a, action
	default:
		// Any other key cancels
		a.pendingConfirm = nil
		a.statusText = "Canceled"
		a.statusIsErr = false
		return a, nil
	}
}

// confirm shows a confirmation dialog. The action runs only if the user presses y/Enter.
func (a *App) confirm(prompt string, action tea.Cmd) {
	a.pendingConfirm = &confirmAction{prompt: prompt, action: action}
}

func (a *App) handleEnter() tea.Cmd {
	switch a.activePanel {
	case PanelProjects:
		if len(a.projects) > 0 && a.cursor[PanelProjects] < len(a.projects) {
			proj := a.projects[a.cursor[PanelProjects]]
			return func() tea.Msg {
				return ProjectSelectedMsg{Project: proj}
			}
		}
	case PanelPipelines:
		// Enter on pipeline: load its jobs
		if len(a.pipelines) > 0 && a.cursor[PanelPipelines] < len(a.pipelines) {
			return a.loadJobs()
		}
	case PanelMergeRequests:
		// TODO: open MR detail view
	case PanelIssues:
		// TODO: open issue detail view
	}
	return nil
}

func (a *App) handlePanelKey(key string) (tea.Model, tea.Cmd) {
	switch a.activePanel {
	case PanelMergeRequests:
		switch key {
		case KeyApprove:
			if idx := a.cursor[PanelMergeRequests]; idx < len(a.mrs) {
				mr := a.mrs[idx]
				a.confirm(fmt.Sprintf("Approve !%d %s?", mr.IID, truncate(mr.Title, 30)), a.approveMR())
				return a, nil
			}
		case KeyMerge:
			if idx := a.cursor[PanelMergeRequests]; idx < len(a.mrs) {
				mr := a.mrs[idx]
				a.confirm(fmt.Sprintf("Merge !%d %s?", mr.IID, truncate(mr.Title, 30)), a.mergeMR())
				return a, nil
			}
		case KeyOpenBrowse:
			return a, a.openInBrowser()
		}
	case PanelPipelines:
		switch key {
		case KeyRetry:
			if idx := a.cursor[PanelPipelines]; idx < len(a.pipelines) {
				p := a.pipelines[idx]
				a.confirm(fmt.Sprintf("Retry pipeline #%d?", p.ID), a.retryPipeline())
				return a, nil
			}
		case KeyCancel:
			if idx := a.cursor[PanelPipelines]; idx < len(a.pipelines) {
				p := a.pipelines[idx]
				a.confirm(fmt.Sprintf("Cancel pipeline #%d?", p.ID), a.cancelPipeline())
				return a, nil
			}
		case KeyRun:
			return a, a.runPipeline()
		case KeyOpenBrowse:
			return a, a.openInBrowser()
		}
	case PanelIssues:
		switch key {
		case KeyComment:
			if idx := a.cursor[PanelIssues]; idx < len(a.issues) {
				issue := a.issues[idx]
				action := "Close"
				if issue.State != "opened" {
					action = "Reopen"
				}
				a.confirm(fmt.Sprintf("%s #%d %s?", action, issue.IID, truncate(issue.Title, 30)), a.toggleIssue())
				return a, nil
			}
		case KeyOpenBrowse:
			return a, a.openInBrowser()
		}
	}
	return a, nil
}

func (a *App) moveCursor(delta int) {
	listLen := a.activeListLen()
	if listLen == 0 {
		return
	}
	a.cursor[a.activePanel] += delta
	if a.cursor[a.activePanel] < 0 {
		a.cursor[a.activePanel] = 0
	}
	if a.cursor[a.activePanel] >= listLen {
		a.cursor[a.activePanel] = listLen - 1
	}
}

func (a *App) activeListLen() int {
	switch a.activePanel {
	case PanelProjects:
		return len(a.projects)
	case PanelMergeRequests:
		return len(a.mrs)
	case PanelPipelines:
		return len(a.pipelines)
	case PanelIssues:
		return len(a.issues)
	}
	return 0
}

// ============================================================================
// View
// ============================================================================

func (a *App) View() tea.View {
	var content string
	switch {
	case a.width == 0:
		content = "Loading..."
	case a.showHelp:
		content = a.renderHelp()
	default:
		// Recompute layout each frame (active panel affects proportions)
		a.layout = ComputeLayout(a.width, a.height, a.activePanel)

		// Pipeline/Jobs panel: swap content when viewing jobs
		pipeTitle := a.pipelinePanelTitle()
		pipeItems := a.pipelineItems()
		pipeCollapsed := a.collapsedPipelineLine()
		pipeCursor := a.cursor[PanelPipelines]
		if a.viewingJobs {
			pipeTitle = a.jobsPanelTitle()
			var jobToDisplay []int
			pipeItems, jobToDisplay = a.jobItems()
			pipeCollapsed = a.collapsedJobLine()
			if a.jobCursor >= 0 && a.jobCursor < len(jobToDisplay) {
				pipeCursor = jobToDisplay[a.jobCursor]
			} else {
				pipeCursor = 0
			}
		}

		sidebar := lipgloss.JoinVertical(lipgloss.Left,
			a.renderSidePanelSmart(PanelProjects, "Projects", a.projectItems(), a.collapsedProjectLine(), a.cursor[PanelProjects]),
			a.renderSidePanelSmart(PanelPipelines, pipeTitle, pipeItems, pipeCollapsed, pipeCursor),
			a.renderSidePanelSmart(PanelMergeRequests, "Merge Requests", a.mrItems(), a.collapsedMRLine(), a.cursor[PanelMergeRequests]),
			a.renderSidePanelSmart(PanelIssues, "Issues", a.issueItems(), a.collapsedIssueLine(), a.cursor[PanelIssues]),
		)

		detail := a.renderDetail()
		main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)
		keybindBar := a.renderKeybindBar()
		statusBar := a.renderStatusBar()
		content = lipgloss.JoinVertical(lipgloss.Left, main, keybindBar, statusBar)
	}

	// Overlay confirmation dialog if active
	if a.pendingConfirm != nil && a.width > 0 {
		content = a.renderConfirmOverlay(content)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) jobsPanelTitle() string {
	if idx := a.cursor[PanelPipelines]; idx < len(a.pipelines) {
		return fmt.Sprintf("Jobs (#%d)", a.pipelines[idx].ID)
	}
	return "Jobs"
}

// jobItems returns display lines for jobs grouped by stage, along with a
// mapping from job index to display-row index (accounting for header lines).
// Header lines are styled and stored with a leading "\x00" marker so
// renderSidePanel knows not to highlight them.
func (a *App) jobItems() ([]string, []int) {
	var items []string
	jobToDisplay := make([]int, len(a.jobs))
	currentStage := ""
	for i, job := range a.jobs {
		if job.Stage != currentStage {
			currentStage = job.Stage
			header := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(job.Stage)
			items = append(items, "\x00"+header)
		}
		jobToDisplay[i] = len(items)
		icon := PipelineStatusIcon(job.Status)
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

func (a *App) collapsedJobLine() string {
	if len(a.jobs) == 0 {
		return "No jobs"
	}
	if a.jobCursor >= 0 && a.jobCursor < len(a.jobs) {
		job := a.jobs[a.jobCursor]
		return fmt.Sprintf("%s %s  %s", PipelineStatusIcon(job.Status), job.Name, job.Status)
	}
	job := a.jobs[0]
	return fmt.Sprintf("%s %s  %s", PipelineStatusIcon(job.Status), job.Name, job.Status)
}

func (a *App) pipelinePanelTitle() string {
	if a.activeBranch != nil {
		return fmt.Sprintf("Pipelines [%s]", truncate(a.activeBranch.Name, 15))
	}
	return "Pipelines"
}

func (a *App) renderSidePanelSmart(id PanelID, title string, items []string, collapsedLine string, cursor int) string {
	// Only the Projects panel collapses (when not focused)
	if id == PanelProjects && a.activePanel != PanelProjects {
		totalWidth := a.layout.SidebarWidth
		panelHeight := a.layout.PanelHeights[id]
		titleText := fmt.Sprintf("[%d] %s", int(id)+1, title)
		line := truncate(collapsedLine, totalWidth-4)
		return renderBox(titleText, []string{line}, totalWidth, panelHeight, ColorSecondary, ColorSecondary)
	}
	return a.renderSidePanel(id, title, items, cursor)
}

func (a *App) renderSidePanel(id PanelID, title string, items []string, cursor int) string {
	isActive := a.activePanel == id
	panelHeight := a.layout.PanelHeights[id]
	totalWidth := a.layout.SidebarWidth
	innerWidth := totalWidth - 4   // border + padding on each side
	innerHeight := panelHeight - 2 // top + bottom border

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 0 {
		innerHeight = 0
	}

	// Scroll offset: keep cursor visible
	scrollOffset := 0
	if cursor >= innerHeight {
		scrollOffset = cursor - innerHeight + 1
	}

	var contentLines []string
	for i := scrollOffset; i < len(items) && len(contentLines) < innerHeight; i++ {
		item := items[i]
		isHeader := len(item) > 0 && item[0] == '\x00'
		if isHeader {
			item = item[1:] // strip marker
		}
		displayItem := truncate(item, innerWidth)
		if i == cursor && isActive && !isHeader {
			plain := ansi.Strip(displayItem)
			visW := lipgloss.Width(plain)
			if visW < innerWidth {
				plain += strings.Repeat(" ", innerWidth-visW)
			}
			displayItem = SelectedItemStyle.Render(plain)
		}
		contentLines = append(contentLines, displayItem)
	}

	borderColor := ColorSecondary
	titleColor := ColorSecondary
	if isActive {
		borderColor = ColorPrimary
		titleColor = ColorPrimary
	}

	titleText := fmt.Sprintf("[%d] %s", int(id)+1, title)
	return renderBox(titleText, contentLines, totalWidth, panelHeight, borderColor, titleColor)
}

func (a *App) detailTitle() string {
	if a.showBranchPicker {
		return "Select Branch"
	}
	if a.viewingJobs && a.activePanel == PanelPipelines {
		if a.jobCursor < len(a.jobs) {
			job := a.jobs[a.jobCursor]
			if a.jobTrace != "" {
				return fmt.Sprintf("Log: %s", job.Name)
			}
			return fmt.Sprintf("Job: %s", job.Name)
		}
		return "Job"
	}
	switch a.activePanel {
	case PanelProjects:
		return "Project"
	case PanelMergeRequests:
		return "Merge Request"
	case PanelPipelines:
		if idx := a.cursor[PanelPipelines]; idx < len(a.pipelines) {
			return fmt.Sprintf("Pipeline (#%d)", a.pipelines[idx].ID)
		}
		return "Pipeline"
	case PanelIssues:
		return "Issue"
	}
	return "Detail"
}

func (a *App) renderDetail() string {
	totalWidth := a.layout.ContentWidth
	totalHeight := a.layout.ContentHeight
	innerHeight := totalHeight - 2

	if innerHeight < 0 {
		innerHeight = 0
	}

	var content string
	if a.showBranchPicker {
		content = a.renderBranchPicker(innerHeight)
	} else {
		switch a.activePanel {
		case PanelProjects:
			content = a.projectDetail()
		case PanelMergeRequests:
			content = a.mrDetail()
		case PanelPipelines:
			if a.viewingJobs {
				content = a.jobDetail()
			} else {
				content = a.pipelineDetail()
			}
		case PanelIssues:
			content = a.issueDetail()
		}
	}

	if content == "" {
		content = "Select an item to view details"
	}

	lines := strings.Split(content, "\n")
	borderColor := ColorSecondary
	if a.viewingJobs && a.jobTrace != "" {
		borderColor = ColorPrimary
	}
	return renderBox(a.detailTitle(), lines, totalWidth, totalHeight, borderColor, ColorPrimary)
}

// renderBox draws a bordered box with a title embedded in the top border line.
func renderBox(title string, lines []string, totalWidth, totalHeight int, borderColor, titleColor color.Color) string {
	contentWidth := totalWidth - 4
	contentHeight := totalHeight - 2

	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	bs := lipgloss.NewStyle().Foreground(borderColor)
	ts := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	truncStyle := lipgloss.NewStyle().MaxWidth(contentWidth)

	// Top border: ╭─[1] Title──────────╮
	fill := totalWidth - len(title) - 3
	if fill < 0 {
		fill = 0
	}
	top := bs.Render("╭─") + ts.Render(title) + bs.Render(strings.Repeat("─", fill)+"╮")

	leftB := bs.Render("│")
	rightB := bs.Render("│")

	var result []string
	result = append(result, top)

	for i := 0; i < contentHeight; i++ {
		var line string
		if i < len(lines) {
			line = truncStyle.Render(lines[i])
		}
		visLen := lipgloss.Width(line)
		pad := contentWidth - visLen
		if pad < 0 {
			pad = 0
		}
		result = append(result, leftB+" "+line+strings.Repeat(" ", pad)+" "+rightB)
	}

	bottom := bs.Render("╰" + strings.Repeat("─", totalWidth-2) + "╯")
	result = append(result, bottom)

	return strings.Join(result, "\n")
}

func (a *App) renderStatusBar() string {
	host := a.activeHost
	project := ""
	if a.activeProject != nil {
		project = a.activeProject.NameWithNamespace
	}
	branch := ""
	if a.activeBranch != nil {
		branch = " | " + a.activeBranch.Name
	}

	left := fmt.Sprintf(" %s | %s%s", host, project, branch)
	right := ""
	if a.statusText != "" {
		right = a.statusText
	}

	gap := a.width - len(left) - len(right) - 2
	if gap < 0 {
		gap = 0
	}

	bar := left + strings.Repeat(" ", gap) + right + " "
	style := StatusBarStyle
	if a.statusIsErr {
		style = style.Foreground(ColorError)
	}
	return style.Width(a.width).Render(bar)
}

func (a *App) renderKeybindBar() string {
	type hint struct{ key, desc string }

	// Always-present global hints
	global := []hint{
		{"q", "Quit"},
		{"?", "Help"},
		{"h/l", "Panel"},
		{"j/k", "Navigate"},
		{"^d/u", "Page"},
		{"r", "Refresh"},
		{"b", "Branch"},
	}

	// Context-specific hints
	var ctx []hint

	switch {
	case a.showBranchPicker:
		ctx = []hint{
			{"Enter", "Select"},
			{"Esc", "Cancel"},
			{"g/G", "Top/bottom"},
		}
	case a.viewingJobs && a.activePanel == PanelPipelines:
		ctx = []hint{
			{"Enter", "Log"},
			{"R", "Retry job"},
			{"C", "Cancel job"},
			{"p", "Play manual"},
			{"o", "Open"},
			{"Esc", "Back"},
		}
	default:
		switch a.activePanel {
		case PanelProjects:
			ctx = []hint{
				{"Enter", "Select"},
				{"o", "Open"},
			}
		case PanelMergeRequests:
			ctx = []hint{
				{"a", "Approve"},
				{"m", "Merge"},
				{"o", "Open"},
			}
		case PanelPipelines:
			ctx = []hint{
				{"Enter", "Jobs"},
				{"p", "Run new"},
				{"R", "Retry"},
				{"C", "Cancel"},
				{"o", "Open"},
			}
			if a.activeBranch != nil {
				ctx = append(ctx, hint{"Esc", "Clear branch"})
			}
		case PanelIssues:
			ctx = []hint{
				{"c", "Close/reopen"},
				{"o", "Open"},
			}
		}
	}

	var parts []string
	for _, h := range global {
		parts = append(parts, fmt.Sprintf("%s: %s",
			HelpDescStyle.Render(h.desc),
			HelpKeyStyle.Render(h.key),
		))
	}
	for _, h := range ctx {
		parts = append(parts, fmt.Sprintf("%s: %s",
			HelpDescStyle.Render(h.desc),
			HelpKeyStyle.Render(h.key),
		))
	}

	sep := HelpSepStyle.Render(" | ")
	bar := " " + strings.Join(parts, sep)
	return lipgloss.NewStyle().
		Width(a.width).
		Render(bar)
}

func (a *App) renderConfirmOverlay(background string) string {
	prompt := a.pendingConfirm.prompt
	hint := "y/Enter: confirm  n/Esc: cancel"

	// Box width: prompt or hint length + padding, whichever is wider
	innerWidth := len(prompt)
	if len(hint) > innerWidth {
		innerWidth = len(hint)
	}
	innerWidth += 4 // padding
	boxWidth := innerWidth + 4

	lines := []string{
		"",
		"  " + lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render(prompt),
		"",
		"  " + HelpDescStyle.Render(hint),
		"",
	}

	box := renderBox("Confirm", lines, boxWidth, len(lines)+2, ColorWarning, ColorWarning)
	overlay := lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)

	// Merge overlay on top of background line by line
	bgLines := strings.Split(background, "\n")
	ovLines := strings.Split(overlay, "\n")
	for i, ovLine := range ovLines {
		trimmed := strings.TrimRight(ovLine, " ")
		if trimmed != "" && i < len(bgLines) {
			bgLines[i] = ovLine
		}
	}
	return strings.Join(bgLines, "\n")
}

func (a *App) renderHelp() string {
	help := []struct{ key, desc string }{
		{"q", "Quit"},
		{"?", "Toggle help"},
		{"1-4", "Switch panel"},
		{"Tab/S-Tab", "Next/prev panel"},
		{"h/l", "Prev/next panel"},
		{"j/k", "Navigate down/up"},
		{"g/G", "Go to top/bottom"},
		{"Ctrl+d/u", "Half page down/up"},
		{"Enter", "Select / view jobs"},
		{"Esc", "Go back / clear filter"},
		{"r", "Refresh"},
		{"o", "Open in browser"},
		{"b", "Select branch"},
		{"", ""},
		{"--- MR ---", ""},
		{"a", "Approve MR"},
		{"m", "Merge MR"},
		{"", ""},
		{"--- Pipeline ---", ""},
		{"Enter", "View jobs"},
		{"p", "Run new pipeline"},
		{"R", "Retry pipeline"},
		{"C", "Cancel pipeline"},
		{"", ""},
		{"--- Jobs (in job view) ---", ""},
		{"R", "Retry job"},
		{"C", "Cancel job"},
		{"p", "Play manual job"},
		{"", ""},
		{"--- Issue ---", ""},
		{"c", "Close/reopen issue"},
	}

	var lines []string
	lines = append(lines, TitleStyle.Render("Keybindings"))
	lines = append(lines, "")
	for _, h := range help {
		if h.key == "" {
			lines = append(lines, "")
			continue
		}
		if strings.HasPrefix(h.key, "---") {
			lines = append(lines, HelpDescStyle.Render(h.key))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s  %s",
			HelpKeyStyle.Width(12).Render(h.key),
			HelpDescStyle.Render(h.desc),
		))
	}
	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render("Press any key to close"))

	content := strings.Join(lines, "\n")
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, content)
}

func (a *App) renderBranchPicker(maxHeight int) string {
	var lines []string
	lines = append(lines, TitleStyle.Render("Select Branch"))
	lines = append(lines, "")

	if len(a.branches) == 0 {
		lines = append(lines, "No branches found")
		return strings.Join(lines, "\n")
	}

	// Reserve space for header (2 lines) and footer (2 lines)
	maxVisible := maxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}
	scrollOffset := 0
	if a.branchCursor >= maxVisible {
		scrollOffset = a.branchCursor - maxVisible + 1
	}

	for i := scrollOffset; i < len(a.branches) && len(lines)-2 < maxVisible; i++ {
		b := a.branches[i]
		marker := "  "
		if b.Default {
			marker = "* "
		} else if b.Protected {
			marker = "P "
		}

		activity := ""
		if !b.LastActivity.IsZero() {
			activity = HelpDescStyle.Render(" " + util.TimeAgo(b.LastActivity))
		}

		line := fmt.Sprintf("%s%s%s", marker, b.Name, activity)
		if i == a.branchCursor {
			line = SelectedItemStyle.Render(fmt.Sprintf("%s%s", marker, b.Name))
			if activity != "" {
				line += activity
			}
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render("Enter: select  Esc: cancel  j/k: navigate"))

	return strings.Join(lines, "\n")
}

// ============================================================================
// Item renderers (sidebar lists)
// ============================================================================

func (a *App) projectItems() []string {
	items := make([]string, len(a.projects))
	for i, p := range a.projects {
		marker := "  "
		if a.activeProject != nil && a.activeProject.ID == p.ID {
			marker = "* "
		}
		items[i] = marker + p.NameWithNamespace
	}
	return items
}

func (a *App) collapsedProjectLine() string {
	if a.activeProject == nil {
		return "No project selected"
	}
	branch := a.activeProject.DefaultBranch
	if a.activeBranch != nil {
		branch = a.activeBranch.Name
	}
	return a.activeProject.NameWithNamespace + " → " + branch
}

func (a *App) collapsedMRLine() string {
	idx := a.cursor[PanelMergeRequests]
	if idx >= 0 && idx < len(a.mrs) {
		mr := a.mrs[idx]
		return fmt.Sprintf("!%d %s", mr.IID, mr.Title)
	}
	if len(a.mrs) == 0 {
		return "No merge requests"
	}
	return fmt.Sprintf("!%d %s", a.mrs[0].IID, a.mrs[0].Title)
}

func (a *App) collapsedPipelineLine() string {
	idx := a.cursor[PanelPipelines]
	if idx >= 0 && idx < len(a.pipelines) {
		p := a.pipelines[idx]
		return fmt.Sprintf("#%d %s %s (%s)", p.ID, PipelineStatusIcon(p.Status), p.Status, p.Ref)
	}
	if len(a.pipelines) == 0 {
		return "No pipelines"
	}
	p := a.pipelines[0]
	return fmt.Sprintf("#%d %s %s", p.ID, PipelineStatusIcon(p.Status), p.Status)
}

func (a *App) collapsedIssueLine() string {
	idx := a.cursor[PanelIssues]
	if idx >= 0 && idx < len(a.issues) {
		issue := a.issues[idx]
		return fmt.Sprintf("#%d %s", issue.IID, issue.Title)
	}
	if len(a.issues) == 0 {
		return "No issues"
	}
	return fmt.Sprintf("#%d %s", a.issues[0].IID, a.issues[0].Title)
}

func (a *App) mrItems() []string {
	items := make([]string, len(a.mrs))
	for i, mr := range a.mrs {
		prefix := ""
		if mr.Draft {
			prefix = "[Draft] "
		}
		pipeIcon := ""
		if mr.Pipeline != nil {
			pipeIcon = " " + PipelineStatusIcon(mr.Pipeline.Status)
		}
		items[i] = fmt.Sprintf("!%d %s%s%s", mr.IID, prefix, mr.Title, pipeIcon)
	}
	return items
}

func (a *App) pipelineItems() []string {
	items := make([]string, len(a.pipelines))
	for i, p := range a.pipelines {
		desc := p.CommitTitle
		if desc == "" {
			desc = p.Ref
		}
		if a.activeBranch != nil {
			// Branch already shown in panel title, skip ref
			items[i] = fmt.Sprintf("%s %s %s",
				util.TimeAgoShort(p.CreatedAt),
				PipelineStatusIcon(p.Status),
				desc,
			)
		} else {
			items[i] = fmt.Sprintf("%s %s %s — %s",
				util.TimeAgoShort(p.CreatedAt),
				PipelineStatusIcon(p.Status),
				p.Ref,
				desc,
			)
		}
	}
	return items
}

func (a *App) issueItems() []string {
	items := make([]string, len(a.issues))
	for i, issue := range a.issues {
		items[i] = fmt.Sprintf("#%d %s", issue.IID, issue.Title)
	}
	return items
}

// ============================================================================
// Detail renderers (right panel)
// ============================================================================

func (a *App) projectDetail() string {
	if len(a.projects) == 0 {
		return "No projects loaded"
	}
	idx := a.cursor[PanelProjects]
	if idx >= len(a.projects) {
		return ""
	}
	p := a.projects[idx]
	return fmt.Sprintf("%s\n\n%s\n\nDefault branch: %s\n%s",
		TitleStyle.Render(p.NameWithNamespace),
		p.Name,
		p.DefaultBranch,
		HelpDescStyle.Render(p.WebURL),
	)
}

func (a *App) mrDetail() string {
	if len(a.mrs) == 0 {
		return "No merge requests"
	}
	idx := a.cursor[PanelMergeRequests]
	if idx >= len(a.mrs) {
		return ""
	}
	mr := a.mrs[idx]

	pipeStatus := "none"
	if mr.Pipeline != nil {
		pipeStatus = mr.Pipeline.Status
	}
	draft := ""
	if mr.Draft {
		draft = " [Draft]"
	}

	return fmt.Sprintf("%s%s\n\n%s -> %s\nAuthor: %s\nPipeline: %s\n\n%s\n\n%s",
		TitleStyle.Render(fmt.Sprintf("!%d %s", mr.IID, mr.Title)),
		draft,
		mr.SourceBranch, mr.TargetBranch,
		mr.Author,
		pipeStatus,
		mr.Description,
		HelpDescStyle.Render(mr.WebURL),
	)
}

func (a *App) pipelineDetail() string {
	if len(a.pipelines) == 0 {
		return "No pipelines"
	}
	idx := a.cursor[PanelPipelines]
	if idx >= len(a.pipelines) {
		return ""
	}
	p := a.pipelines[idx]

	var lines []string
	lines = append(lines,
		fmt.Sprintf("Status:  %s %s",
			PipelineStatusIcon(p.Status),
			lipgloss.NewStyle().Foreground(PipelineStatusColor(p.Status)).Render(p.Status),
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
	lines = append(lines, HelpDescStyle.Render(p.WebURL))

	return strings.Join(lines, "\n")
}

func (a *App) jobDetail() string {
	if len(a.jobs) == 0 {
		return "No jobs"
	}
	if a.jobCursor >= len(a.jobs) {
		return ""
	}

	// Show trace log if loaded
	if a.jobTrace != "" {
		return a.jobTraceView()
	}

	job := a.jobs[a.jobCursor]

	statusColor := PipelineStatusColor(job.Status)
	coloredStatus := lipgloss.NewStyle().Foreground(statusColor).Render(job.Status)

	var lines []string
	lines = append(lines,
		fmt.Sprintf("Status:   %s %s", PipelineStatusIcon(job.Status), coloredStatus),
	)
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
	lines = append(lines, HelpDescStyle.Render("Press Enter to view log"))

	return strings.Join(lines, "\n")
}

func (a *App) jobTraceView() string {
	contentWidth := a.layout.ContentWidth - 4
	if contentWidth < 1 {
		contentWidth = 1
	}
	viewHeight := a.layout.ContentHeight - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	// Clean GitLab trace: strip ANSI, carriage returns, section markers
	var cleaned []string
	for _, line := range strings.Split(a.jobTrace, "\n") {
		line = ansi.Strip(line)
		// Remove carriage returns (progress bars etc.)
		line = strings.ReplaceAll(line, "\r", "")
		// Skip GitLab CI section markers
		if strings.HasPrefix(line, "section_start:") || strings.HasPrefix(line, "section_end:") {
			continue
		}
		// Word-wrap long lines
		for len(line) > contentWidth {
			cleaned = append(cleaned, line[:contentWidth])
			line = line[contentWidth:]
		}
		cleaned = append(cleaned, line)
	}

	// Update max scroll based on cleaned lines
	totalLines := len(cleaned)
	maxScroll := totalLines - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if a.jobTraceScroll > maxScroll {
		a.jobTraceScroll = maxScroll
	}

	// Apply scroll offset
	start := a.jobTraceScroll
	if start < 0 {
		start = 0
	}
	end := start + viewHeight
	if end > len(cleaned) {
		end = len(cleaned)
	}

	visible := cleaned[start:end]
	return strings.Join(visible, "\n")
}

func (a *App) issueDetail() string {
	if len(a.issues) == 0 {
		return "No issues"
	}
	idx := a.cursor[PanelIssues]
	if idx >= len(a.issues) {
		return ""
	}
	issue := a.issues[idx]

	labels := "none"
	if len(issue.Labels) > 0 {
		labels = strings.Join(issue.Labels, ", ")
	}
	assignees := "unassigned"
	if len(issue.Assignees) > 0 {
		assignees = strings.Join(issue.Assignees, ", ")
	}

	return fmt.Sprintf("%s\n\nAuthor: %s\nAssignees: %s\nLabels: %s\n\n%s\n\n%s",
		TitleStyle.Render(fmt.Sprintf("#%d %s", issue.IID, issue.Title)),
		issue.Author,
		assignees,
		labels,
		issue.Description,
		HelpDescStyle.Render(issue.WebURL),
	)
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (a *App) loadProjects() tea.Cmd {
	client := a.clients[a.activeHost]
	if client == nil {
		return nil
	}
	a.loading = true
	return func() tea.Msg {
		projects, err := client.ListProjects()
		return ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

func (a *App) loadMRs() tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	return func() tea.Msg {
		mrs, err := client.ListMergeRequests(projectID)
		return MRsLoadedMsg{MRs: mrs, Err: err}
	}
}

func (a *App) loadPipelines() tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID

	// If branch is selected, filter by ref
	if a.activeBranch != nil {
		ref := a.activeBranch.Name
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

func (a *App) loadIssues() tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	return func() tea.Msg {
		issues, err := client.ListIssues(projectID)
		return IssuesLoadedMsg{Issues: issues, Err: err}
	}
}

func (a *App) loadJobs() tea.Cmd {
	if a.activeProject == nil || len(a.pipelines) == 0 {
		return nil
	}
	idx := a.cursor[PanelPipelines]
	if idx >= len(a.pipelines) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	pipelineID := a.pipelines[idx].ID
	return func() tea.Msg {
		jobs, err := client.ListPipelineJobs(projectID, pipelineID)
		return JobsLoadedMsg{Jobs: jobs, Err: err}
	}
}

func (a *App) loadJobTrace() tea.Cmd {
	if a.activeProject == nil || len(a.jobs) == 0 {
		return nil
	}
	if a.jobCursor >= len(a.jobs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	jobID := a.jobs[a.jobCursor].ID
	return func() tea.Msg {
		trace, err := client.GetJobTrace(projectID, jobID)
		return JobTraceLoadedMsg{Trace: trace, Err: err}
	}
}

func (a *App) loadBranches() tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	return func() tea.Msg {
		branches, err := client.ListBranches(projectID)
		return BranchesLoadedMsg{Branches: branches, Err: err}
	}
}

func (a *App) refreshActivePanel() tea.Cmd {
	switch a.activePanel {
	case PanelProjects:
		return a.loadProjects()
	case PanelMergeRequests:
		return a.loadMRs()
	case PanelPipelines:
		return a.loadPipelines()
	case PanelIssues:
		return a.loadIssues()
	}
	return nil
}

func (a *App) approveMR() tea.Cmd {
	if a.activeProject == nil || len(a.mrs) == 0 {
		return nil
	}
	idx := a.cursor[PanelMergeRequests]
	if idx >= len(a.mrs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	mrIID := a.mrs[idx].IID
	return func() tea.Msg {
		err := client.ApproveMergeRequest(projectID, mrIID)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Approve failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Approved !%d", mrIID)}
	}
}

func (a *App) mergeMR() tea.Cmd {
	if a.activeProject == nil || len(a.mrs) == 0 {
		return nil
	}
	idx := a.cursor[PanelMergeRequests]
	if idx >= len(a.mrs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	mrIID := a.mrs[idx].IID
	return func() tea.Msg {
		err := client.MergeMergeRequest(projectID, mrIID)
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Merge failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Merged !%d", mrIID)}
	}
}

func (a *App) retryPipeline() tea.Cmd {
	if a.activeProject == nil || len(a.pipelines) == 0 {
		return nil
	}
	idx := a.cursor[PanelPipelines]
	if idx >= len(a.pipelines) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	pipelineID := a.pipelines[idx].ID
	return func() tea.Msg {
		err := client.RetryPipeline(projectID, pipelineID)
		if err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Retried pipeline #%d", pipelineID)}
	}
}

func (a *App) cancelPipeline() tea.Cmd {
	if a.activeProject == nil || len(a.pipelines) == 0 {
		return nil
	}
	idx := a.cursor[PanelPipelines]
	if idx >= len(a.pipelines) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	pipelineID := a.pipelines[idx].ID
	return func() tea.Msg {
		err := client.CancelPipeline(projectID, pipelineID)
		if err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Cancel failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Canceled pipeline #%d", pipelineID)}
	}
}

func (a *App) runPipeline() tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID

	// Use selected branch or default branch
	ref := a.activeProject.DefaultBranch
	if a.activeBranch != nil {
		ref = a.activeBranch.Name
	}

	return func() tea.Msg {
		p, err := client.RunPipeline(projectID, ref)
		if err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Run failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Pipeline #%d started on %s", p.ID, ref)}
	}
}

func (a *App) retryJob() tea.Cmd {
	if a.activeProject == nil || len(a.jobs) == 0 {
		return nil
	}
	if a.jobCursor >= len(a.jobs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	job := a.jobs[a.jobCursor]
	return func() tea.Msg {
		err := client.RetryJob(projectID, job.ID)
		if err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Retry job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Retried job '%s'", job.Name)}
	}
}

func (a *App) cancelJob() tea.Cmd {
	if a.activeProject == nil || len(a.jobs) == 0 {
		return nil
	}
	if a.jobCursor >= len(a.jobs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	job := a.jobs[a.jobCursor]
	return func() tea.Msg {
		err := client.CancelJob(projectID, job.ID)
		if err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Cancel job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Canceled job '%s'", job.Name)}
	}
}

func (a *App) playJob() tea.Cmd {
	if a.activeProject == nil || len(a.jobs) == 0 {
		return nil
	}
	if a.jobCursor >= len(a.jobs) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	job := a.jobs[a.jobCursor]
	if job.Status != "manual" {
		return func() tea.Msg {
			return StatusMsg{Text: "Only manual jobs can be played", IsErr: true}
		}
	}
	return func() tea.Msg {
		err := client.PlayJob(projectID, job.ID)
		if err != nil {
			return JobActionDoneMsg{Text: fmt.Sprintf("Play job failed: %v", err), IsErr: true}
		}
		return JobActionDoneMsg{Text: fmt.Sprintf("Playing job '%s'", job.Name)}
	}
}

func (a *App) toggleIssue() tea.Cmd {
	if a.activeProject == nil || len(a.issues) == 0 {
		return nil
	}
	idx := a.cursor[PanelIssues]
	if idx >= len(a.issues) {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	issue := a.issues[idx]
	return func() tea.Msg {
		var err error
		if issue.State == "opened" {
			err = client.CloseIssue(projectID, issue.IID)
		} else {
			err = client.ReopenIssue(projectID, issue.IID)
		}
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Toggle issue failed: %v", err), IsErr: true}
		}
		action := "Closed"
		if issue.State != "opened" {
			action = "Reopened"
		}
		return StatusMsg{Text: fmt.Sprintf("%s #%d", action, issue.IID)}
	}
}

func (a *App) openInBrowser() tea.Cmd {
	var url string
	switch a.activePanel {
	case PanelProjects:
		if idx := a.cursor[PanelProjects]; idx < len(a.projects) {
			url = a.projects[idx].WebURL
		}
	case PanelMergeRequests:
		if idx := a.cursor[PanelMergeRequests]; idx < len(a.mrs) {
			url = a.mrs[idx].WebURL
		}
	case PanelPipelines:
		if idx := a.cursor[PanelPipelines]; idx < len(a.pipelines) {
			url = a.pipelines[idx].WebURL
		}
	case PanelIssues:
		if idx := a.cursor[PanelIssues]; idx < len(a.issues) {
			url = a.issues[idx].WebURL
		}
	}
	cmd := openBrowserCmd(url)
	if cmd == nil {
		return nil
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Failed to open browser: %v", err), IsErr: true}
		}
		return nil
	})
}

// truncate shortens a string to maxLen.
func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return ansi.Truncate(s, maxLen, "")
	}
	return ansi.Truncate(s, maxLen-3, "") + "..."
}
