package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/janvseticek/lazyglab/internal/gitlab"
)

// App is the root Bubble Tea model.
type App struct {
	// GitLab clients per host
	clients   map[string]*gitlab.Client
	hostNames []string
	activeHost string

	// Active project
	activeProject *gitlab.Project

	// Data
	projects  []gitlab.Project
	mrs       []gitlab.MergeRequest
	pipelines []gitlab.Pipeline
	issues    []gitlab.Issue

	// UI state
	activePanel PanelID
	cursor      [4]int // cursor position per panel
	layout      Layout
	showHelp    bool
	statusText  string
	statusIsErr bool
	loading     bool

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
	// Load projects on startup
	return a.loadProjects()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.layout = ComputeLayout(msg.Width, msg.Height)
		return a, nil

	case tea.KeyMsg:
		// Help overlay takes precedence
		if a.showHelp {
			a.showHelp = false
			return a, nil
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
		a.statusText = fmt.Sprintf("Selected: %s", msg.Project.NameWithNamespace)
		a.statusIsErr = false
		// Load MRs, pipelines, issues in parallel
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
		return a, nil

	case IssuesLoadedMsg:
		if msg.Err != nil {
			a.statusText = fmt.Sprintf("Error loading issues: %v", msg.Err)
			a.statusIsErr = true
			return a, nil
		}
		a.issues = msg.Issues
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
	case KeyTab:
		a.activePanel = (a.activePanel + 1) % 4
		return a, nil
	case KeyShiftTab:
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

	// Enter: select item
	if key == KeyEnter {
		return a, a.handleEnter()
	}

	// Panel-specific keys
	return a.handlePanelKey(key)
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
	case PanelMergeRequests:
		// TODO: open MR detail view
	case PanelPipelines:
		// TODO: open pipeline detail view
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

func (a *App) View() tea.View {
	var content string
	if a.width == 0 {
		content = "Loading..."
	} else if a.showHelp {
		content = a.renderHelp()
	} else {
		// Render sidebar panels
		sidebar := lipgloss.JoinVertical(lipgloss.Left,
			a.renderSidePanel(PanelProjects, "Projects", a.projectItems()),
			a.renderSidePanel(PanelMergeRequests, "Merge Requests", a.mrItems()),
			a.renderSidePanel(PanelPipelines, "Pipelines", a.pipelineItems()),
			a.renderSidePanel(PanelIssues, "Issues", a.issueItems()),
		)

		// Render detail panel
		detail := a.renderDetail()

		// Join sidebar and detail
		main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, detail)

		// Status bar
		statusBar := a.renderStatusBar()

		content = lipgloss.JoinVertical(lipgloss.Left, main, statusBar)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) renderSidePanel(id PanelID, title string, items []string) string {
	style := InactiveBorderStyle
	if a.activePanel == id {
		style = ActiveBorderStyle
	}

	panelHeight := a.layout.PanelHeights[id]
	innerWidth := a.layout.SidebarWidth - 4 // borders + padding
	innerHeight := panelHeight - 2          // borders

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	// Build content
	header := TitleStyle.Render(fmt.Sprintf("[%d] %s", int(id)+1, title))
	lines := []string{header}

	cursor := a.cursor[id]
	for i, item := range items {
		if len(lines) >= innerHeight {
			break
		}
		// Truncate item to fit
		displayItem := truncate(item, innerWidth)
		if i == cursor && a.activePanel == id {
			displayItem = SelectedItemStyle.Render(displayItem)
		}
		lines = append(lines, displayItem)
	}

	content := strings.Join(lines, "\n")

	return style.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

func (a *App) renderDetail() string {
	innerWidth := a.layout.ContentWidth - 4
	innerHeight := a.layout.ContentHeight - 2

	if innerWidth < 1 {
		innerWidth = 1
	}
	if innerHeight < 1 {
		innerHeight = 1
	}

	var content string
	switch a.activePanel {
	case PanelProjects:
		content = a.projectDetail()
	case PanelMergeRequests:
		content = a.mrDetail()
	case PanelPipelines:
		content = a.pipelineDetail()
	case PanelIssues:
		content = a.issueDetail()
	}

	if content == "" {
		content = "Select an item to view details"
	}

	return InactiveBorderStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

func (a *App) renderStatusBar() string {
	host := a.activeHost
	project := ""
	if a.activeProject != nil {
		project = a.activeProject.NameWithNamespace
	}

	left := fmt.Sprintf(" %s | %s", host, project)
	right := ""
	if a.statusText != "" {
		right = a.statusText
	}

	// Pad to fill width
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

func (a *App) renderHelp() string {
	help := []struct{ key, desc string }{
		{"q", "Quit"},
		{"?", "Toggle help"},
		{"1-4", "Switch panel"},
		{"Tab/S-Tab", "Next/prev panel"},
		{"j/k", "Navigate down/up"},
		{"g/G", "Go to top/bottom"},
		{"Enter", "Select/open detail"},
		{"Esc", "Go back"},
		{"r", "Refresh"},
		{"o", "Open in browser"},
		{"", ""},
		{"--- MR ---", ""},
		{"a", "Approve MR"},
		{"m", "Merge MR"},
		{"", ""},
		{"--- Pipeline ---", ""},
		{"R", "Retry pipeline"},
		{"C", "Cancel pipeline"},
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

// --- Item renderers ---

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

// --- Detail renderers ---

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

	return fmt.Sprintf("%s\n\nStatus: %s\nRef: %s\nSHA: %s\n\n%s",
		TitleStyle.Render(fmt.Sprintf("Pipeline #%d", p.ID)),
		p.Status,
		p.Ref,
		p.SHA[:8],
		HelpDescStyle.Render(p.WebURL),
	)
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

// --- Commands ---

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
			return StatusMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Retried pipeline #%d", pipelineID)}
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
			return StatusMsg{Text: fmt.Sprintf("Cancel failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Canceled pipeline #%d", pipelineID)}
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
	if url == "" {
		return nil
	}
	return tea.ExecProcess(openBrowserCmd(url), func(err error) tea.Msg {
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
