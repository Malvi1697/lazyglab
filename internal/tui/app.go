package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
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

	// Incremental "/" search inside the project and branch pickers.
	projectFilter components.Filter
	branchFilter  components.Filter

	// First visible row of each picker, kept across frames so the cursor moves
	// inside the window instead of dragging it.
	projectScroll  int
	branchScroll   int
	favoriteScroll int
	helpScroll     int

	// Re-authentication overlay: opened automatically when the stored token is
	// rejected, or on demand with "A".
	reconfigure         ReconfigureFunc
	reconfig            *reconfigState
	authPromptDismissed bool

	// Favorites: starred project paths on the active host, plus the picker's state.
	favorites      []string
	saveFavorites  SaveFavoritesFunc
	favoriteCursor int
	pickerStatus   string // inline note shown inside whichever picker is open

	// Session resume: the project to reopen at startup and how to record it.
	lastProject     string
	saveLastProject SaveLastProjectFunc

	// Auto-detected project path from the git remote, matched once at startup.
	detectedPath string

	// Auto-refresh period; 0 disables ticking.
	refreshInterval time.Duration

	// Refresh feedback. Pressing r used to look like it did nothing: the request
	// went out, the numbers came back the same, and nothing on screen said so.
	// These drive the note at the top right — in flight, just updated, and how
	// long until the next automatic fetch.
	refreshing  bool
	spinning    bool      // a spinner tick chain is running
	spinFrame   int       // which frame of it
	lastRefresh time.Time // when data last arrived
	nextRefresh time.Time // when the auto-refresh will fire

	// focused is whether the terminal is looked at. Polling GitLab every thirty
	// seconds for a window nobody is watching is pure waste, so the tick pauses
	// while we are in the background and catches up on the way back.
	focused      bool
	clockRunning bool // the one-second clock; also stopped while unfocused

	// viewFetched is when each view's data last arrived, so stepping through the
	// tabs shows what was just fetched instead of refetching all of it per tab.
	viewFetched map[views.ViewID]time.Time

	// now returns the current time. A field so tests can pin it.
	now func() time.Time

	// A newer release, found once at startup. Kept out of the status line: that is
	// overwritten by the next thing that happens, and this is worth still being
	// there an hour later.
	checkUpdate   CheckUpdateFunc
	updateVersion string

	// Dimensions and status line.
	width, height int
	statusText    string
	statusIsErr   bool
}

// CheckUpdateFunc returns the newest released version when it is newer than the
// running one, and "" when there is nothing to say. It blocks on the network, so
// the shell only ever calls it from a command.
type CheckUpdateFunc func() string

// Options is everything the cockpit shell needs from the app layer: the GitLab
// clients, the preferences read from the config file, and the callbacks that
// write changes back to it (the TUI cannot import the app package itself).
type Options struct {
	Clients      map[string]*gitlab.Client
	HostNames    []string
	DetectedHost string
	DetectedPath string

	ViewIDs          []views.ViewID
	DefaultViewIndex int
	RefreshInterval  time.Duration

	// Favorites are starred project paths ("group/project") on the active host.
	Favorites []string
	// LastProject is the path selected on the previous run, restored at startup
	// unless a project was detected from the git remote.
	LastProject string

	// Reconfigure persists a new host/token; nil disables re-authentication.
	Reconfigure ReconfigureFunc
	// SaveFavorites persists the favorites of a host; nil keeps stars
	// in-session only.
	SaveFavorites SaveFavoritesFunc
	// SaveLastProject records the active project; nil means the next launch will
	// not resume it.
	SaveLastProject SaveLastProjectFunc
	// CheckUpdate looks for a newer release; nil skips the check entirely.
	CheckUpdate CheckUpdateFunc
}

// NewApp builds the cockpit shell: it selects the active host, constructs the
// shared Context and every enabled view, and records the startup detection state.
func NewApp(o Options) *App {
	clients, hostNames := o.Clients, o.HostNames
	viewIDs, defaultIndex := o.ViewIDs, o.DefaultViewIndex

	activeHost := o.DetectedHost
	if activeHost == "" && len(hostNames) > 0 {
		activeHost = hostNames[0]
	}

	ctx := &views.Context{Client: clients[activeHost]}

	built := make(map[views.ViewID]views.View, len(viewIDs))
	for _, id := range viewIDs {
		switch id {
		case views.ViewDashboard:
			built[id] = views.NewDashboardView(ctx)
		case views.ViewPipelines:
			built[id] = views.NewPipelinesView(ctx)
		case views.ViewMRs:
			built[id] = views.NewMRsView(ctx)
		case views.ViewIssues:
			built[id] = views.NewIssuesView(ctx)
		case views.ViewTodos:
			built[id] = views.NewTodosView(ctx)
		case views.ViewCommits:
			built[id] = views.NewCommitsView(ctx)
		}
	}

	if defaultIndex < 0 || defaultIndex >= len(viewIDs) {
		defaultIndex = 0
	}

	return &App{
		now:             time.Now,
		focused:         true, // assumed until the terminal says otherwise
		viewFetched:     make(map[views.ViewID]time.Time, len(viewIDs)),
		clients:         clients,
		hostNames:       hostNames,
		activeHost:      activeHost,
		ctx:             ctx,
		viewIDs:         viewIDs,
		views:           built,
		active:          defaultIndex,
		detectedPath:    o.DetectedPath,
		refreshInterval: o.RefreshInterval,
		reconfigure:     o.Reconfigure,
		favorites:       o.Favorites,
		saveFavorites:   o.SaveFavorites,
		lastProject:     o.LastProject,
		saveLastProject: o.SaveLastProject,
		checkUpdate:     o.CheckUpdate,
	}
}

// activeView returns the currently focused view.
func (a *App) activeView() views.View { return a.views[a.viewIDs[a.active]] }

// ============================================================================
// Init / tick
// ============================================================================

func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.refresh(), a.clockCmd(), a.updateCheckCmd()}
	if a.refreshInterval > 0 {
		a.nextRefresh = a.clock().Add(a.refreshInterval)
		cmds = append(cmds, a.tickCmd())
	}

	// Open the project this directory is about, or the one from the previous run —
	// being inside a repo is the stronger signal, so it wins. Both resolve by path,
	// which is one request; listing every project the user can see to find one of
	// them was several, and on a big instance the slowest thing about starting up.
	switch {
	case a.detectedPath != "":
		cmds = append(cmds, a.selectProjectByPath(a.detectedPath))
	case a.lastProject != "":
		cmds = append(cmds, a.selectProjectByPath(a.lastProject))
	default:
		// Nothing to open: the picker is the next thing that happens, so the list is
		// worth fetching now.
		cmds = append(cmds, a.loadProjects())
	}
	return tea.Batch(cmds...)
}

// tickMsg fires on each auto-refresh interval.
type tickMsg struct{}

// clockMsg fires once a second, so the countdown to the next refresh and the
// "updated Ns ago" note stay true without anything else happening.
type clockMsg struct{}

// spinMsg advances the spinner while a refresh is in flight.
type spinMsg struct{}

func (a *App) tickCmd() tea.Cmd {
	if a.refreshInterval <= 0 {
		return nil
	}
	return tea.Tick(a.refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// clockCmd starts the one-second clock that keeps the countdown and the "updated
// Ns ago" note honest. Only one chain runs at a time, and it stops while the
// terminal is in the background: nobody is reading a countdown they cannot see.
func (a *App) clockCmd() tea.Cmd {
	if a.clockRunning {
		return nil
	}
	a.clockRunning = true
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return clockMsg{} })
}

// spinCmd starts the spinner's tick chain, unless one is already running — two
// chains would make it spin at double speed and never stop.
func (a *App) spinCmd() tea.Cmd {
	if a.spinning {
		return nil
	}
	a.spinning = true
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return spinMsg{} })
}

// clock is the current time, through the pinnable field.
func (a *App) clock() time.Time {
	if a.now == nil {
		return time.Now()
	}
	return a.now()
}

// viewFreshFor is how long a view's data is reused when you come back to it.
// Flipping through the tabs to check something used to refetch every one of them
// on the way past; within this window the data on screen is the data you just
// looked at, and the note at the top right says how old it is.
const viewFreshFor = 10 * time.Second

// refreshIfStale reloads the active view unless its data is younger than
// viewFreshFor. Pressing r never goes through here: an explicit refresh must
// always reach GitLab.
func (a *App) refreshIfStale() tea.Cmd {
	if at, ok := a.viewFetched[a.viewIDs[a.active]]; ok && a.clock().Sub(at) < viewFreshFor {
		return nil
	}
	return a.refresh()
}

// refresh reloads the active view and says so on screen. Every path that reloads
// data goes through here, so the note at the top right is never a lie about
// whether something is in flight.
func (a *App) refresh() tea.Cmd {
	cmd := a.activeView().Focus()
	if cmd == nil {
		// Nothing to fetch (no project or no client yet); claiming a refresh would
		// leave a spinner turning forever.
		return nil
	}
	a.refreshing = true
	a.spinFrame = 0
	return tea.Batch(cmd, a.spinCmd())
}

// ============================================================================
// Update
// ============================================================================

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Any load rejected because the stored token is unusable pops the
	// re-authentication overlay, whichever view asked for the data — otherwise
	// every view just shows a red 401 with no way to fix it from inside the app.
	// The message still flows on, so status text and view state stay correct.
	a.maybePromptReauth(views.LoadErr(msg))

	// Data coming back ends the refresh, error or not. A view may fan out several
	// requests, and the first one home is what makes the screen change — that is
	// the moment worth reporting.
	if views.IsLoadResult(msg) {
		a.refreshing = false
		a.lastRefresh = a.clock()
		// Attributed to the active view, which is the one that asked.
		a.viewFetched[a.viewIDs[a.active]] = a.lastRefresh
	}

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

	case tea.PasteMsg:
		// With bracketed paste (on by default) pasted text arrives as its own
		// message rather than as key presses, so the re-authentication form has to
		// accept it explicitly — a token is pasted far more often than typed.
		switch a.overlay {
		case overlayReconfig:
			a.pasteIntoReconfig(msg.Content)
		case overlayProject:
			if a.projectFilter.Paste(msg.Content) {
				a.projectCursor = 0
			}
		case overlayBranch:
			if a.branchFilter.Paste(msg.Content) {
				a.branchCursor = 0
			}
		case overlayNone:
			// A view with its own "/" search open takes the paste, the same way the
			// pickers do.
			return a, a.activeView().Update(msg)
		}
		return a, nil

	case tea.PasteStartMsg, tea.PasteEndMsg:
		// Paste delimiters carry no content; no view needs them.
		return a, nil

	case tea.BlurMsg:
		// Nobody is looking: stop asking GitLab anything until they are.
		a.focused = false
		return a, nil

	case tea.FocusMsg:
		a.focused = true
		cmds := []tea.Cmd{a.clockCmd()}
		// Back after a while, so what is on screen is stale by definition.
		if a.refreshInterval > 0 && a.clock().Sub(a.lastRefresh) >= a.refreshInterval {
			a.nextRefresh = a.clock().Add(a.refreshInterval)
			cmds = append(cmds, a.refresh())
		}
		return a, tea.Batch(cmds...)

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
			a.cursorOnActiveProject()
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
		return a, tea.Batch(a.refresh(), a.rememberProject(proj.PathWithNamespace))

	case views.BranchesLoadedMsg:
		if msg.Err != nil {
			a.setStatus(fmt.Sprintf("Error loading branches: %v", msg.Err), true)
			return a, nil
		}
		a.branches = msg.Branches
		a.branchCursor = 0
		a.branchFilter.Reset()
		a.overlay = overlayBranch
		return a, nil

	case views.BranchSelectedMsg:
		branch := msg.Branch
		a.ctx.Branch = &branch
		a.overlay = overlayNone
		a.setStatus(fmt.Sprintf("Branch: %s", branch.Name), false)
		return a, a.refresh()

	case views.ConfirmMsg:
		a.confirm(msg.Prompt, msg.Action)
		return a, nil

	case views.StatusMsg:
		a.setStatus(msg.Text, msg.IsErr)
		// Forward so the active view can sync its own status/state.
		return a, a.activeView().Update(msg)

	case reconfigDoneMsg:
		if msg.err != nil {
			if a.reconfig != nil {
				a.reconfig.busy = false
				a.reconfig.err = msg.err.Error()
			}
			return a, nil
		}
		a.applyReconfig(msg.host, msg.client)
		a.setStatus(fmt.Sprintf("Authenticated as %s on %s", msg.username, msg.host), false)
		return a, tea.Batch(a.loadProjects(), a.refresh())

	case favoritesSavedMsg:
		// The star is already applied in memory; only a failed write needs saying.
		if msg.err != nil {
			a.setStatus(fmt.Sprintf("Could not save favorites: %v", msg.err), true)
		}
		return a, nil

	case lastProjectSavedMsg:
		// Failing to remember the project must not disturb the selection itself,
		// but silently losing session state would be worse.
		if msg.err != nil {
			a.setStatus(fmt.Sprintf("Could not save the last project: %v", msg.err), true)
		}
		return a, nil

	case updateFoundMsg:
		a.updateVersion = msg.version
		return a, nil

	case tickMsg:
		var cmd tea.Cmd
		// A modal overlay means the user is mid-decision, and an unfocused terminal
		// means nobody would see the result: neither is worth a request.
		if a.overlay == overlayNone && a.focused {
			cmd = a.refresh()
			if a.refreshInterval > 0 {
				a.nextRefresh = a.clock().Add(a.refreshInterval)
			}
		}
		return a, tea.Batch(cmd, a.tickCmd())

	case clockMsg:
		// Nothing to do but re-arm: the render reads the clock itself, so the
		// countdown and the "updated Ns ago" note move on their own. Unfocused, there
		// is nothing to keep moving, so the clock stops until we are looked at again.
		a.clockRunning = false
		if !a.focused {
			return a, nil
		}
		return a, a.clockCmd()

	case spinMsg:
		if !a.refreshing {
			a.spinning = false
			return a, nil
		}
		a.spinFrame++
		a.spinning = false // released so spinCmd starts the next tick
		return a, a.spinCmd()
	}

	// Anything else (per-view *LoadedMsg/*DoneMsg, etc.) belongs to the view.
	return a, a.activeView().Update(msg)
}

// maybePromptReauth opens the re-authentication overlay when err means the
// stored token is unusable. It stays quiet if the overlay is already up or the
// user dismissed it, so a failing auto-refresh cannot reopen it every tick.
func (a *App) maybePromptReauth(err error) {
	if err == nil || a.reconfigure == nil || !gitlab.IsAuthError(err) {
		return
	}
	if a.overlay == overlayReconfig || a.authPromptDismissed {
		return
	}
	a.openReconfig(authFailureReason(err))
}

// routeOverlayKey dispatches a key press to the active overlay's handler.
func (a *App) routeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.overlay {
	case overlayConfirm:
		return a.handleConfirmKey(msg)
	case overlayReconfig:
		return a.handleReconfigKey(msg)
	case overlayFavorites:
		return a.handleFavoritesKey(msg)
	case overlayBranch:
		return a.handleBranchPickerKey(msg)
	case overlayProject:
		return a.handleProjectPickerKey(msg)
	case overlayHelp:
		return a.handleHelpKey(msg)
	}
	return a, nil
}

// handleKey processes global keys, falling back to the active view.
func (a *App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// A view typing a "/" search owns the keyboard: "q" has to be a letter in the
	// query rather than quitting the app, and "b" a letter rather than the branch
	// picker. Ctrl+c still quits, since a wedged search must not trap anyone.
	if key != "ctrl+c" {
		if tc, ok := a.activeView().(views.TextCapturer); ok && tc.CapturingText() {
			return a, a.activeView().Update(msg)
		}
	}

	switch key {
	case KeyQuit, "ctrl+c":
		return a, tea.Quit
	case KeyHelp:
		a.overlay = overlayHelp
		a.helpScroll = 0
		return a, nil
	// Views are switched with the numbers, H/L and [/]. The uppercase pair moves
	// between the big tabs, which leaves lowercase h/l to move within whatever is
	// open — between commits, or between a commit's files. Tab is deliberately not
	// among them either: it belongs to whatever has focus, cycling the panels inside
	// the active view — a key that means "move within" should not mean "move away".
	case KeyNextView, KeyNextTab:
		a.switchView((a.active + 1) % len(a.viewIDs))
		return a, a.refreshIfStale()
	case KeyPrevView, KeyPrevTab:
		a.switchView((a.active - 1 + len(a.viewIDs)) % len(a.viewIDs))
		return a, a.refreshIfStale()
	case "P": // project switcher (uppercase so "p" stays free for view actions)
		// Each visit starts unfiltered; a stale query would hide most projects, and a
		// note from last time ("Copied git@…") would read as something that just
		// happened.
		a.projectFilter.Reset()
		a.pickerStatus = ""
		a.wantProjectPicker = true
		if len(a.projects) > 0 {
			a.wantProjectPicker = false
			a.overlay = overlayProject
			a.cursorOnActiveProject()
			return a, nil
		}
		return a, a.loadProjects()
	case KeyBranch:
		return a, a.loadBranches()
	case KeyRefresh:
		return a, a.refresh()
	case KeyReauth:
		if a.reconfigure != nil {
			a.openReconfig("")
		}
		return a, nil
	case KeyFavorite:
		a.openFavorites()
		return a, nil
	}

	// Digit keys 1..9 switch to that view by position.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		if n := int(key[0] - '1'); n < len(a.viewIDs) {
			if n != a.active {
				a.switchView(n)
				return a, a.refreshIfStale()
			}
			return a, nil
		}
	}

	// Delegate everything else to the active view.
	return a, a.activeView().Update(msg)
}

// viewIndex returns the position of a view in the enabled list, or -1 when the
// config has it disabled.
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

// cursorOnActiveProject opens the picker on the project you are already in, when it
// is in the list. It is the row you most often want something from — to copy its
// clone URL, or to see where you are — and hunting for it in a list of hundreds was
// the picker's own fault.
func (a *App) cursorOnActiveProject() {
	a.projectCursor = 0
	if a.ctx == nil || a.ctx.Project == nil {
		return
	}
	for i, p := range a.visibleProjects() {
		if strings.EqualFold(p.PathWithNamespace, a.ctx.Project.PathWithNamespace) {
			a.projectCursor = i
			return
		}
	}
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

	contextBar := renderContextBar(a.width, a.ctx, a.statusText, a.statusIsErr,
		a.refreshNote(a.clock()))
	tabs := renderTabs(a.width, a.viewIDs, a.active, titles, a.updateNote())

	// A blank row between the tabs and the body: without it the context line, the
	// tabs and the first heading pile up as three rows of bold text.
	bodyHeight := a.height - 4
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := a.activeView().Body(a.width, bodyHeight)

	footer := renderFooter(a.width, a.globalHints(), a.activeView().KeyHints())

	frame := lipgloss.JoinVertical(lipgloss.Left, contextBar, tabs, "", body, footer)

	if a.overlay != overlayNone {
		if box := a.overlayBox(); box != "" {
			frame = a.mergeOverlay(frame, box)
		}
	}

	v := tea.NewView(frame)
	v.AltScreen = true
	// So the terminal tells us when it is in the background and the refresh can
	// pause instead of polling for nobody.
	v.ReportFocus = true
	return v
}

// globalHints returns the shell-level key hints shown in the footer.
func (a *App) globalHints() []views.KeyHint {
	return []views.KeyHint{
		{Key: "q", Desc: "Quit"},
		{Key: "?", Desc: "Help"},
		{Key: fmt.Sprintf("1-%d", len(a.viewIDs)), Desc: "View"},
		{Key: "P", Desc: "Project"},
		{Key: "f", Desc: "★"},
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
	case overlayReconfig:
		return a.renderReconfig()
	case overlayFavorites:
		return a.renderFavorites()
	}
	return ""
}
