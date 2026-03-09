package tui

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	showBranchPicker bool
	branchCursor     int

	// Dimensions
	width  int
	height int
}

// NewApp creates the root application model.
func NewApp(clients map[string]*gitlab.Client, hostNames []string) *App {
	activeHost := ""
	if len(hostNames) > 0 {
		activeHost = hostNames[0]
	}
	return &App{
		clients:     clients,
		hostNames:   hostNames,
		activeHost:  activeHost,
		activePanel: PanelProjects,
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
		a.activePanel = PanelMergeRequests
		return a, nil
	case KeyPanel3:
		a.activePanel = PanelPipelines
		return a, nil
	case KeyPanel4:
		a.activePanel = PanelIssues
		return a, nil
	case KeyRefresh:
		return a, a.refreshActivePanel()
	case KeyBranch:
		return a, a.loadBranches()
	}

	// Escape: go back from job view, or clear branch filter
	if key == KeyEscape {
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

	if isNavigateUp(msg) {
		if a.jobCursor > 0 {
			a.jobCursor--
		}
		return a, nil
	}
	if isNavigateDown(msg) {
		if a.jobCursor < len(a.jobs)-1 {
			a.jobCursor++
		}
		return a, nil
	}
	if key == KeyTop {
		a.jobCursor = 0
		return a, nil
	}
	if key == KeyBottom {
		a.jobCursor = len(a.jobs) - 1
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		return a, nil
	}

	switch key {
	case KeyHalfDown:
		halfPage := (a.layout.ContentHeight - 4) / 2
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
		return a, nil
	case KeyHalfUp:
		halfPage := (a.layout.ContentHeight - 4) / 2
		if halfPage < 1 {
			halfPage = 1
		}
		a.jobCursor -= halfPage
		if a.jobCursor < 0 {
			a.jobCursor = 0
		}
		return a, nil
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
		return a, a.retryJob()
	case KeyCancel:
		return a, a.cancelJob()
	case KeyPlayJob:
		return a, a.playJob()
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
			return a, a.approveMR()
		case KeyMerge:
			return a, a.mergeMR()
		case KeyOpenBrowse:
			return a, a.openInBrowser()
		}
	case PanelPipelines:
		switch key {
		case KeyRetry:
			return a, a.retryPipeline()
		case KeyCancel:
			return a, a.cancelPipeline()
		case KeyRun:
			return a, a.runPipeline()
		case KeyOpenBrowse:
			return a, a.openInBrowser()
		}
	case PanelIssues:
		switch key {
		case KeyComment:
			return a, a.toggleIssue()
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

		sidebar := lipgloss.JoinVertical(lipgloss.Left,
			a.renderSidePanel(PanelProjects, "Projects", a.projectItems()),
			a.renderSidePanel(PanelMergeRequests, "Merge Requests", a.mrItems()),
			a.renderSidePanel(PanelPipelines, a.pipelinePanelTitle(), a.pipelineItems()),
			a.renderSidePanel(PanelIssues, "Issues", a.issueItems()),
		)

		detail := a.renderDetail()
		main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)
		keybindBar := a.renderKeybindBar()
		statusBar := a.renderStatusBar()
		content = lipgloss.JoinVertical(lipgloss.Left, main, keybindBar, statusBar)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) pipelinePanelTitle() string {
	if a.activeBranch != nil {
		return fmt.Sprintf("Pipelines [%s]", truncate(a.activeBranch.Name, 15))
	}
	return "Pipelines"
}

func (a *App) renderSidePanel(id PanelID, title string, items []string) string {
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

	cursor := a.cursor[id]

	// Scroll offset: keep cursor visible
	scrollOffset := 0
	if cursor >= innerHeight {
		scrollOffset = cursor - innerHeight + 1
	}

	var contentLines []string
	for i := scrollOffset; i < len(items) && len(contentLines) < innerHeight; i++ {
		displayItem := truncate(items[i], innerWidth)
		if i == cursor && isActive {
			displayItem = SelectedItemStyle.Render(displayItem)
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
		return "Jobs"
	}
	switch a.activePanel {
	case PanelProjects:
		return "Project"
	case PanelMergeRequests:
		return "Merge Request"
	case PanelPipelines:
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
				content = a.jobsDetail()
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
	return renderBox(a.detailTitle(), lines, totalWidth, totalHeight, ColorSecondary, ColorPrimary)
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
		{"q", "quit"},
		{"?", "help"},
		{"h/l", "panel"},
		{"j/k", "nav"},
		{"^d/u", "page"},
		{"r", "refresh"},
		{"b", "branch"},
	}

	// Context-specific hints
	var ctx []hint

	switch {
	case a.showBranchPicker:
		ctx = []hint{
			{"Enter", "select"},
			{"Esc", "cancel"},
			{"g/G", "top/bottom"},
		}
	case a.viewingJobs && a.activePanel == PanelPipelines:
		ctx = []hint{
			{"R", "retry job"},
			{"C", "cancel job"},
			{"p", "play manual"},
			{"o", "open"},
			{"Esc", "back"},
		}
	default:
		switch a.activePanel {
		case PanelProjects:
			ctx = []hint{
				{"Enter", "select"},
				{"o", "open"},
			}
		case PanelMergeRequests:
			ctx = []hint{
				{"a", "approve"},
				{"m", "merge"},
				{"o", "open"},
			}
		case PanelPipelines:
			ctx = []hint{
				{"Enter", "jobs"},
				{"p", "run new"},
				{"R", "retry"},
				{"C", "cancel"},
				{"o", "open"},
			}
			if a.activeBranch != nil {
				ctx = append(ctx, hint{"Esc", "clear branch"})
			}
		case PanelIssues:
			ctx = []hint{
				{"c", "close/reopen"},
				{"o", "open"},
			}
		}
	}

	var parts []string
	for _, h := range global {
		parts = append(parts, fmt.Sprintf("%s %s",
			HelpKeyStyle.Render(h.key),
			HelpDescStyle.Render(h.desc),
		))
	}
	parts = append(parts, HelpDescStyle.Render("|"))
	for _, h := range ctx {
		parts = append(parts, fmt.Sprintf("%s %s",
			HelpKeyStyle.Render(h.key),
			HelpDescStyle.Render(h.desc),
		))
	}

	bar := " " + strings.Join(parts, "  ")
	return StatusBarStyle.
		Background(lipgloss.Color("#222222")).
		Width(a.width).
		Render(bar)
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
		items[i] = fmt.Sprintf("#%d %s %s (%s)",
			p.ID,
			PipelineStatusIcon(p.Status),
			p.Status,
			p.Ref,
		)
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

	sha := p.SHA
	if len(sha) > 8 {
		sha = sha[:8]
	}

	var lines []string
	lines = append(lines, TitleStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID)))
	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("Status:  %s %s",
			PipelineStatusIcon(p.Status),
			lipgloss.NewStyle().Foreground(PipelineStatusColor(p.Status)).Render(p.Status),
		),
	)
	lines = append(lines, fmt.Sprintf("Ref:     %s", p.Ref))
	lines = append(lines, fmt.Sprintf("SHA:     %s", sha))
	if !p.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", util.TimeAgo(p.CreatedAt)))
	}
	if !p.UpdatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Updated: %s", util.TimeAgo(p.UpdatedAt)))
	}
	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render("Enter: view jobs  R: retry  C: cancel  p: run new"))
	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render(p.WebURL))

	return strings.Join(lines, "\n")
}

func (a *App) jobsDetail() string {
	if len(a.pipelines) == 0 {
		return ""
	}
	idx := a.cursor[PanelPipelines]
	if idx >= len(a.pipelines) {
		return ""
	}
	p := a.pipelines[idx]

	var lines []string
	lines = append(lines, TitleStyle.Render(fmt.Sprintf("Pipeline #%d - Jobs", p.ID)))
	lines = append(lines, "")

	if len(a.jobs) == 0 {
		lines = append(lines, "No jobs found")
		return strings.Join(lines, "\n")
	}

	// Group jobs by stage
	currentStage := ""
	for i, job := range a.jobs {
		if job.Stage != currentStage {
			currentStage = job.Stage
			if i > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary).Render(
				fmt.Sprintf("  Stage: %s", currentStage),
			))
		}

		icon := PipelineStatusIcon(job.Status)
		statusColor := PipelineStatusColor(job.Status)
		coloredStatus := lipgloss.NewStyle().Foreground(statusColor).Render(job.Status)
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

		line := fmt.Sprintf("    %s %-20s %s%s",
			icon,
			truncate(job.Name, 20),
			coloredStatus,
			duration,
		)

		if i == a.jobCursor {
			line = SelectedItemStyle.Render(line)
		}
		lines = append(lines, line)
	}

	lines = append(lines, "")
	lines = append(lines, HelpDescStyle.Render("j/k: navigate  R: retry  C: cancel  p: play  o: open  Esc: back"))

	return strings.Join(lines, "\n")
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
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
