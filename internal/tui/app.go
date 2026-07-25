package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// App is the root Bubble Tea model for the v2 cockpit. It owns the shared view
// Context, routes global keys, hosts the modal overlays, and drives auto-refresh.
// Each full-screen view is responsible for its own data and rendering.
type App struct {
	// GitLab clients per host.
	clients    map[string]*gitlab.Client
	hostNames  []string
	activeHost string

	// Shared session state handed to every view.
	ctx *views.Context

	// Enabled views in tab order and their constructed instances.
	viewIDs []views.ViewID
	views   map[views.ViewID]views.View
	active  int

	// Modal overlay state.
	overlay           overlayKind
	projects          []gitlab.Project
	projectCursor     int
	wantProjectPicker bool // pressing p requests the picker once projects load
	branches          []gitlab.Branch
	branchCursor      int
	pendingConfirm    *confirmAction

	// Auto-detected project path from the git remote, matched once at startup.
	detectedPath string

	// Auto-refresh period; 0 disables ticking.
	refreshInterval time.Duration

	// Dimensions and status line.
	width, height int
	statusText    string
	statusIsErr   bool
}

// NewApp builds the cockpit shell: it selects the active host, constructs the
// shared Context and every enabled view, and records the startup detection state.
func NewApp(
	clients map[string]*gitlab.Client,
	hostNames []string,
	detectedHost, detectedPath string,
	viewIDs []views.ViewID,
	defaultIndex int,
	refreshInterval time.Duration,
) *App {
	activeHost := detectedHost
	if activeHost == "" && len(hostNames) > 0 {
		activeHost = hostNames[0]
	}

	ctx := &views.Context{Client: clients[activeHost]}

	built := make(map[views.ViewID]views.View, len(viewIDs))
	for _, id := range viewIDs {
		switch id {
		case views.ViewOverview:
			built[id] = views.NewOverviewView(ctx)
		case views.ViewPipelines:
			built[id] = views.NewPipelinesView(ctx)
		case views.ViewMRs:
			built[id] = views.NewMRsView(ctx)
		case views.ViewIssues:
			built[id] = views.NewIssuesView(ctx)
		case views.ViewCommits:
			built[id] = views.NewCommitsView(ctx)
		}
	}

	if defaultIndex < 0 || defaultIndex >= len(viewIDs) {
		defaultIndex = 0
	}

	return &App{
		clients:         clients,
		hostNames:       hostNames,
		activeHost:      activeHost,
		ctx:             ctx,
		viewIDs:         viewIDs,
		views:           built,
		active:          defaultIndex,
		detectedPath:    detectedPath,
		refreshInterval: refreshInterval,
	}
}

// activeView returns the currently focused view.
func (a *App) activeView() views.View { return a.views[a.viewIDs[a.active]] }

// ============================================================================
// Init / tick
// ============================================================================

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.loadProjects(), a.activeView().Focus()}
	if a.refreshInterval > 0 {
		cmds = append(cmds, a.tickCmd())
	}
	return tea.Batch(cmds...)
}

// tickMsg fires on each auto-refresh interval.
type tickMsg struct{}

func (a *App) tickCmd() tea.Cmd {
	if a.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(a.refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// ============================================================================
// Update
// ============================================================================

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, a.activeView().Update(msg)

	case tea.KeyMsg:
		if a.overlay != overlayNone {
			return a.routeOverlayKey(msg)
		}
		return a.handleKey(msg)

	case views.ProjectsLoadedMsg:
		if msg.Err != nil {
			a.setStatus(fmt.Sprintf("Error loading projects: %v", msg.Err), true)
			a.wantProjectPicker = false
			return a, nil
		}
		a.projects = msg.Projects
		a.clampProjectCursor()

		// Auto-select the project detected from the git remote (once).
		if a.detectedPath != "" && a.ctx.Project == nil {
			for i, p := range a.projects {
				if strings.EqualFold(p.PathWithNamespace, a.detectedPath) {
					a.projectCursor = i
					a.detectedPath = ""
					proj := p
					return a, func() tea.Msg { return views.ProjectSelectedMsg{Project: proj} }
				}
			}
		}

		if a.wantProjectPicker {
			a.wantProjectPicker = false
			a.overlay = overlayProject
			return a, nil
		}
		a.setStatus(fmt.Sprintf("Loaded %d projects", len(a.projects)), false)
		return a, nil

	case views.ProjectSelectedMsg:
		proj := msg.Project
		a.ctx.Project = &proj
		a.ctx.Branch = nil
		a.overlay = overlayNone
		a.setStatus(fmt.Sprintf("Selected: %s", proj.NameWithNamespace), false)
		return a, a.activeView().Focus()

	case views.BranchesLoadedMsg:
		if msg.Err != nil {
			a.setStatus(fmt.Sprintf("Error loading branches: %v", msg.Err), true)
			return a, nil
		}
		a.branches = msg.Branches
		a.branchCursor = 0
		a.overlay = overlayBranch
		return a, nil

	case views.BranchSelectedMsg:
		branch := msg.Branch
		a.ctx.Branch = &branch
		a.overlay = overlayNone
		a.setStatus(fmt.Sprintf("Branch: %s", branch.Name), false)
		return a, a.activeView().Focus()

	case views.ConfirmMsg:
		a.confirm(msg.Prompt, msg.Action)
		return a, nil

	case views.StatusMsg:
		a.setStatus(msg.Text, msg.IsErr)
		// Forward so the active view can sync its own status/state.
		return a, a.activeView().Update(msg)

	case tickMsg:
		var cmd tea.Cmd
		if a.overlay == overlayNone {
			cmd = a.activeView().Focus()
		}
		return a, tea.Batch(cmd, a.tickCmd())
	}

	// Anything else (per-view *LoadedMsg/*DoneMsg, etc.) belongs to the view.
	return a, a.activeView().Update(msg)
}

// routeOverlayKey dispatches a key press to the active overlay's handler.
func (a *App) routeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.overlay {
	case overlayConfirm:
		return a.handleConfirmKey(msg)
	case overlayBranch:
		return a.handleBranchPickerKey(msg)
	case overlayProject:
		return a.handleProjectPickerKey(msg)
	case overlayHelp:
		a.overlay = overlayNone
		return a, nil
	}
	return a, nil
}

// handleKey processes global keys, falling back to the active view.
func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case KeyQuit, "ctrl+c":
		return a, tea.Quit
	case KeyHelp:
		a.overlay = overlayHelp
		return a, nil
	case KeyTab:
		a.switchView((a.active + 1) % len(a.viewIDs))
		return a, a.activeView().Focus()
	case KeyShiftTab:
		a.switchView((a.active - 1 + len(a.viewIDs)) % len(a.viewIDs))
		return a, a.activeView().Focus()
	case KeyRun: // "p" — project switcher
		a.wantProjectPicker = true
		if len(a.projects) > 0 {
			a.wantProjectPicker = false
			a.overlay = overlayProject
			return a, nil
		}
		return a, a.loadProjects()
	case KeyBranch:
		return a, a.loadBranches()
	case KeyRefresh:
		return a, a.activeView().Focus()
	}

	// Digit keys 1..9 switch to that view by position.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		if n := int(key[0] - '1'); n < len(a.viewIDs) {
			if n != a.active {
				a.switchView(n)
				return a, a.activeView().Focus()
			}
			return a, nil
		}
	}

	// Delegate everything else to the active view.
	return a, a.activeView().Update(msg)
}

// switchView changes the active view index.
func (a *App) switchView(idx int) {
	if idx >= 0 && idx < len(a.viewIDs) {
		a.active = idx
	}
}

func (a *App) setStatus(text string, isErr bool) {
	a.statusText = text
	a.statusIsErr = isErr
}

// clampProjectCursor keeps the project cursor within bounds.
func (a *App) clampProjectCursor() {
	if a.projectCursor >= len(a.projects) {
		a.projectCursor = len(a.projects) - 1
	}
	if a.projectCursor < 0 {
		a.projectCursor = 0
	}
}

// ============================================================================
// Data loading
// ============================================================================

func (a *App) loadProjects() tea.Cmd {
	client := a.ctx.Client
	if client == nil {
		return nil
	}
	return func() tea.Msg {
		projects, err := client.ListProjects()
		return views.ProjectsLoadedMsg{Projects: projects, Err: err}
	}
}

func (a *App) loadBranches() tea.Cmd {
	client := a.ctx.Client
	if client == nil || a.ctx.Project == nil {
		return nil
	}
	projectID := a.ctx.Project.ID
	return func() tea.Msg {
		branches, err := client.ListBranches(projectID)
		return views.BranchesLoadedMsg{Branches: branches, Err: err}
	}
}

// ============================================================================
// View
// ============================================================================

func (a *App) View() tea.View {
	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	titles := make([]string, len(a.viewIDs))
	for i, id := range a.viewIDs {
		titles[i] = a.views[id].Title()
	}

	contextBar := renderContextBar(a.width, a.ctx, a.statusText, a.statusIsErr)
	tabs := renderTabs(a.width, a.viewIDs, a.active, titles)

	bodyHeight := a.height - 3
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := a.activeView().Body(a.width, bodyHeight)

	footer := renderFooter(a.width, a.globalHints(), a.activeView().KeyHints())

	frame := lipgloss.JoinVertical(lipgloss.Left, contextBar, tabs, body, footer)

	if a.overlay != overlayNone {
		if box := a.overlayBox(); box != "" {
			frame = a.mergeOverlay(frame, box)
		}
	}

	v := tea.NewView(frame)
	v.AltScreen = true
	return v
}

// globalHints returns the shell-level key hints shown in the footer.
func (a *App) globalHints() []views.KeyHint {
	return []views.KeyHint{
		{Key: "q", Desc: "Quit"},
		{Key: "?", Desc: "Help"},
		{Key: fmt.Sprintf("1-%d", len(a.viewIDs)), Desc: "View"},
		{Key: "p", Desc: "Project"},
		{Key: "b", Desc: "Branch"},
		{Key: "r", Desc: "↻"},
	}
}

// overlayBox renders the currently active overlay's box, or "" if none.
func (a *App) overlayBox() string {
	switch a.overlay {
	case overlayProject:
		return a.renderProjectPicker()
	case overlayBranch:
		return a.renderBranchPicker()
	case overlayHelp:
		return a.renderHelp()
	case overlayConfirm:
		return a.renderConfirm()
	}
	return ""
}
