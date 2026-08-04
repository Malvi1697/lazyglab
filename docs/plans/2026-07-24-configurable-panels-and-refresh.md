# Configurable Panels, Auto-Refresh & Pipeline Redesign — Implementation Plan

**Goal:** Make the lazyglab TUI adaptable and readable — configurable sidebar panels (show/hide + reorder), active-panel auto-refresh, a redesigned pipeline list with a hover job-preview, and a set of polish fixes.

**Architecture:** Settings live in a new optional `settings:` block in `~/.config/lazyglab/config.yml` (backwards compatible). `PanelID` remains the stable identity enum; a new `a.panels []PanelID` holds the visible/ordered set. Layout and the sidebar view iterate that list. Auto-refresh uses `tea.Tick`; the job preview uses a debounced tick plus a per-pipeline jobs cache.

**Tech Stack:** Go 1.26, charm.land/bubbletea/v2, charm.land/lipgloss/v2, gitlab.com/gitlab-org/api/client-go, gopkg.in/yaml.v3.

**Spec:** `docs/specs/2026-07-24-tui-panels-refresh-design.md`

**Prereq for every commit:** `~/go/bin` must be on `PATH` (pre-commit hooks call `golangci-lint`/`goimports`). Run `export PATH="$PATH:$(go env GOPATH)/bin"` once per shell.

---

## File structure

| File | Responsibility | Action |
|---|---|---|
| `internal/util/timeago.go` | Fixed-width short relative time | Modify (`TimeAgoShort`) |
| `internal/app/config.go` | Config schema incl. `Settings` + normalization | Modify |
| `internal/tui/panels.go` | Panel registry: ID↔name, `ParsePanels` | **Create** |
| `internal/app/app.go` | Wire settings → `tui.NewApp` | Modify |
| `internal/tui/app.go` | Model fields, cycling, view loop, pipeline rows, auto-refresh, job preview, polish | Modify |
| `internal/tui/layout.go` | `ComputeLayout` takes visible panel list | Modify |
| `internal/tui/messages.go` | New msg types (`tickMsg`, `previewTickMsg`, `previewJobsLoadedMsg`) | Modify |
| `internal/tui/panels_test.go` | `ParsePanels` tests | **Create** |
| `internal/app/config_test.go` | Settings normalization tests | Modify |
| `internal/util/timeago_test.go` | Uniform-width test | Modify |
| `internal/tui/layout_test.go` | Update signature + hidden/reordered cases | Modify |

---

## Task 1: Fixed-width `TimeAgoShort`

**Files:**
- Modify: `internal/util/timeago.go:10-25`
- Test: `internal/util/timeago_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/util/timeago_test.go`:

```go
func TestTimeAgoShort_UniformWidth(t *testing.T) {
	now := time.Now()
	cases := []time.Time{
		now.Add(-30 * time.Second),      // <1m
		now.Add(-5 * time.Minute),       // 5m
		now.Add(-26 * time.Minute),      // 26m
		now.Add(-3 * time.Hour),         // 3h
		now.Add(-5 * 24 * time.Hour),    // 5d
		now.Add(-90 * 24 * time.Hour),   // 3mo
	}
	for _, tc := range cases {
		got := TimeAgoShort(tc)
		if w := lipgloss.Width(got); w != 4 {
			t.Errorf("TimeAgoShort(%v) = %q has width %d, want 4", tc, got, w)
		}
	}
}
```

Add imports at top of the test file if missing: `"time"` and `"charm.land/lipgloss/v2"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/util/ -run TestTimeAgoShort_UniformWidth -v`
Expected: FAIL (current `" <1m"` is 4 but `"3h"`/`"26m"` are 3).

- [ ] **Step 3: Implement uniform width**

Replace the body of `TimeAgoShort` in `internal/util/timeago.go`:

```go
// TimeAgoShort returns a compact relative time string padded to a fixed width
// of 4 (right-aligned) so it forms an aligned column, e.g. " <1m", " 26m",
// "  3h", "  5d", " 3mo".
func TimeAgoShort(t time.Time) string {
	d := time.Since(t)

	var s string
	switch {
	case d < time.Minute:
		s = "<1m"
	case d < time.Hour:
		s = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		s = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		s = fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		s = fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%4s", s)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/util/ -v`
Expected: PASS (all timeago tests).

- [ ] **Step 5: Commit**

```bash
git add internal/util/timeago.go internal/util/timeago_test.go
git commit -m "fix: make TimeAgoShort a uniform fixed width for aligned columns"
```

---

## Task 2: `Settings` config schema + normalization

**Files:**
- Modify: `internal/app/config.go:11-22`
- Test: `internal/app/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/app/config_test.go`:

```go
func TestSettings_RefreshSeconds(t *testing.T) {
	ptr := func(i int) *int { return &i }
	cases := []struct {
		name string
		in   *int
		want int
	}{
		{"unset defaults to 30", nil, 30},
		{"zero disables", ptr(0), 0},
		{"negative defaults to 30", ptr(-5), 30},
		{"below floor clamps to 5", ptr(3), 5},
		{"normal value kept", ptr(45), 45},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := Settings{RefreshInterval: tc.in}
			if got := s.RefreshSeconds(); got != tc.want {
				t.Errorf("RefreshSeconds() = %d, want %d", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run TestSettings_RefreshSeconds -v`
Expected: FAIL to compile (`Settings` undefined).

- [ ] **Step 3: Add `Settings` to config**

In `internal/app/config.go`, replace the `Config`/`HostConfig` block (lines 11-22) with:

```go
// Config is lazyglab's own configuration.
type Config struct {
	DefaultHost string                `yaml:"default_host"`
	Hosts       map[string]HostConfig `yaml:"hosts"`
	Settings    Settings              `yaml:"settings"`
}

// HostConfig holds per-host GitLab configuration.
type HostConfig struct {
	Token   string `yaml:"token"`
	APIHost string `yaml:"api_host,omitempty"` // optional: if API is on different host
}

// Settings holds global UI preferences.
type Settings struct {
	// Panels lists the visible sidebar panels in display order, by config name
	// (projects, pipelines, merge_requests, issues). Empty = all four.
	Panels []string `yaml:"panels"`
	// RefreshInterval is the auto-refresh period in seconds. nil = default (30),
	// 0 = disabled. Interpreted via RefreshSeconds().
	RefreshInterval *int `yaml:"refresh_interval"`
}

// RefreshSeconds returns the normalized auto-refresh interval in seconds:
// nil/negative -> 30, 0 -> disabled, 1..4 -> clamped to 5, else the value.
func (s Settings) RefreshSeconds() int {
	if s.RefreshInterval == nil {
		return 30
	}
	v := *s.RefreshInterval
	switch {
	case v == 0:
		return 0
	case v < 0:
		return 30
	case v < 5:
		return 5
	default:
		return v
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run TestSettings_RefreshSeconds -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "feat: add Settings block (panels + refresh_interval) to config"
```

---

## Task 3: Panel registry (`ParsePanels`)

**Files:**
- Create: `internal/tui/panels.go`
- Test: `internal/tui/panels_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/panels_test.go`:

```go
package tui

import (
	"reflect"
	"testing"
)

func TestParsePanels(t *testing.T) {
	all := []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests, PanelIssues}

	t.Run("empty returns default order", func(t *testing.T) {
		got, warn := ParsePanels(nil)
		if !reflect.DeepEqual(got, all) {
			t.Errorf("got %v, want %v", got, all)
		}
		if len(warn) != 0 {
			t.Errorf("unexpected warnings: %v", warn)
		}
	})

	t.Run("hide issues", func(t *testing.T) {
		got, _ := ParsePanels([]string{"projects", "pipelines", "merge_requests"})
		want := []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		got, _ := ParsePanels([]string{"projects", "issues", "pipelines", "merge_requests"})
		want := []PanelID{PanelProjects, PanelIssues, PanelPipelines, PanelMergeRequests}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("unknown dropped with warning", func(t *testing.T) {
		got, warn := ParsePanels([]string{"projects", "bogus", "issues"})
		want := []PanelID{PanelProjects, PanelIssues}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})

	t.Run("duplicate dropped with warning", func(t *testing.T) {
		got, warn := ParsePanels([]string{"projects", "pipelines", "pipelines"})
		want := []PanelID{PanelProjects, PanelPipelines}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})

	t.Run("projects forced present when omitted", func(t *testing.T) {
		got, warn := ParsePanels([]string{"pipelines", "issues"})
		want := []PanelID{PanelProjects, PanelPipelines, PanelIssues}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestParsePanels -v`
Expected: FAIL to compile (`ParsePanels` undefined).

- [ ] **Step 3: Implement the registry**

Create `internal/tui/panels.go`:

```go
package tui

import "fmt"

// panelConfigName maps a PanelID to its config file name.
func panelConfigName(id PanelID) string {
	switch id {
	case PanelProjects:
		return "projects"
	case PanelPipelines:
		return "pipelines"
	case PanelMergeRequests:
		return "merge_requests"
	case PanelIssues:
		return "issues"
	}
	return ""
}

// panelIDFromName maps a config name to a PanelID.
func panelIDFromName(name string) (PanelID, bool) {
	switch name {
	case "projects":
		return PanelProjects, true
	case "pipelines":
		return PanelPipelines, true
	case "merge_requests":
		return PanelMergeRequests, true
	case "issues":
		return PanelIssues, true
	}
	return 0, false
}

// defaultPanels is the full ordered set of panels.
func defaultPanels() []PanelID {
	return []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests, PanelIssues}
}

// ParsePanels converts config panel names into an ordered, deduplicated list of
// visible panels. Rules: empty input -> all panels in default order; unknown and
// duplicate names are dropped (with a warning); the Projects panel is always
// present (prepended if the user omitted it). Returns the panels and warnings.
func ParsePanels(names []string) ([]PanelID, []string) {
	if len(names) == 0 {
		return defaultPanels(), nil
	}

	var out []PanelID
	var warnings []string
	seen := make(map[PanelID]bool)

	for _, n := range names {
		id, ok := panelIDFromName(n)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown panel %q ignored", n))
			continue
		}
		if seen[id] {
			warnings = append(warnings, fmt.Sprintf("duplicate panel %q ignored", n))
			continue
		}
		seen[id] = true
		out = append(out, id)
	}

	if !seen[PanelProjects] {
		out = append([]PanelID{PanelProjects}, out...)
		warnings = append(warnings, "projects panel is required; added automatically")
	}

	return out, warnings
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run TestParsePanels -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/panels.go internal/tui/panels_test.go
git commit -m "feat: add panel registry and ParsePanels with validation"
```

---

## Task 4: Dynamic layout (`ComputeLayout` takes visible panels)

**Files:**
- Modify: `internal/tui/layout.go:24-83`
- Test: `internal/tui/layout_test.go`

- [ ] **Step 1: Update tests to new signature + add cases**

In `internal/tui/layout_test.go`, every call to `ComputeLayout(w, h, panel)` becomes
`ComputeLayout(w, h, panel, defaultPanels())`. Then add:

```go
func TestComputeLayout_HiddenPanel(t *testing.T) {
	panels := []PanelID{PanelProjects, PanelPipelines, PanelIssues} // no MRs
	l := ComputeLayout(120, 40, PanelPipelines, panels)

	if l.PanelHeights[PanelMergeRequests] != 0 {
		t.Errorf("hidden MR panel should have height 0, got %d", l.PanelHeights[PanelMergeRequests])
	}
	if l.PanelHeights[PanelProjects] != 3 {
		t.Errorf("collapsed Projects should be 3, got %d", l.PanelHeights[PanelProjects])
	}
	sum := l.PanelHeights[PanelProjects] + l.PanelHeights[PanelPipelines] + l.PanelHeights[PanelIssues]
	usable := 40 - l.StatusBarHeight - l.KeybindBarHeight
	if sum != usable {
		t.Errorf("visible heights sum %d, want %d", sum, usable)
	}
}

func TestComputeLayout_ProjectsActiveSplitsEvenly(t *testing.T) {
	panels := []PanelID{PanelProjects, PanelPipelines} // 2 visible
	l := ComputeLayout(120, 40, PanelProjects, panels)
	usable := 40 - l.StatusBarHeight - l.KeybindBarHeight
	if l.PanelHeights[PanelProjects]+l.PanelHeights[PanelPipelines] != usable {
		t.Errorf("two-panel heights must sum to usable %d", usable)
	}
	if l.PanelHeights[PanelMergeRequests] != 0 || l.PanelHeights[PanelIssues] != 0 {
		t.Errorf("hidden panels must be 0")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/ -run TestComputeLayout -v`
Expected: FAIL to compile (signature mismatch).

- [ ] **Step 3: Rewrite `ComputeLayout`**

Replace `internal/tui/layout.go` lines 24-83 with:

```go
// ComputeLayout calculates panel dimensions based on terminal size and the set
// of visible panels (in display order). PanelHeights is keyed by PanelID; hidden
// panels get height 0.
func ComputeLayout(width, height int, activePanel PanelID, panels []PanelID) Layout {
	l := Layout{
		Width:            width,
		Height:           height,
		StatusBarHeight:  1,
		KeybindBarHeight: 1,
	}

	// Sidebar takes ~45% of width, min 35, max 75
	l.SidebarWidth = width * 45 / 100
	if l.SidebarWidth < 35 {
		l.SidebarWidth = 35
	}
	if l.SidebarWidth > 75 {
		l.SidebarWidth = 75
	}

	l.ContentWidth = width - l.SidebarWidth
	if l.ContentWidth < 10 {
		l.ContentWidth = 10
	}

	usableHeight := height - l.StatusBarHeight - l.KeybindBarHeight
	if usableHeight < 12 {
		usableHeight = 12
	}
	l.ContentHeight = usableHeight

	if len(panels) == 0 {
		return l
	}

	projectsVisible := false
	for _, p := range panels {
		if p == PanelProjects {
			projectsVisible = true
		}
	}

	// distribute spreads total across ids, giving the remainder to the first few.
	distribute := func(ids []PanelID, total int) {
		n := len(ids)
		if n == 0 {
			return
		}
		base := total / n
		rem := total - base*n
		for i, id := range ids {
			h := base
			if i < rem {
				h++
			}
			l.PanelHeights[id] = h
		}
	}

	if activePanel == PanelProjects || !projectsVisible {
		distribute(panels, usableHeight)
		return l
	}

	// Projects collapsed to 3 lines; remaining height split among the others.
	const collapsed = 3
	l.PanelHeights[PanelProjects] = collapsed
	others := make([]PanelID, 0, len(panels))
	for _, p := range panels {
		if p != PanelProjects {
			others = append(others, p)
		}
	}
	distribute(others, usableHeight-collapsed)
	return l
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run TestComputeLayout -v`
Expected: PASS. (The build of `app.go` will still fail — its `ComputeLayout` call is fixed in Task 5.)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/layout.go internal/tui/layout_test.go
git commit -m "feat: ComputeLayout distributes height over visible panels"
```

---

## Task 5: App model fields + wiring settings through `NewApp`

**Files:**
- Modify: `internal/tui/app.go:17-88` (struct + `NewApp`)
- Modify: `internal/tui/app.go:701` (`ComputeLayout` call)
- Modify: `internal/app/app.go:26-46` (build panels + interval, pass to `NewApp`)

- [ ] **Step 1: Add fields to the `App` struct**

In `internal/tui/app.go`, inside `type App struct`, add after the `// UI state` group (after `loading bool`, line 48):

```go
	// Panel configuration
	panels          []PanelID     // visible panels in display order
	refreshInterval time.Duration // 0 = auto-refresh disabled

	// Job preview (hover) state
	previewPipelineID int
	previewJobs       []gitlab.Job
	previewLoading    bool
	previewGen        int
	jobsCache         map[int]jobsCacheEntry
```

Add the cache type near `confirmAction` (after line 70):

```go
// jobsCacheEntry caches a pipeline's jobs for the hover preview.
type jobsCacheEntry struct {
	jobs []gitlab.Job
	at   time.Time
}
```

Add `"time"` to the imports at the top of `internal/tui/app.go`.

- [ ] **Step 2: Update `NewApp` signature and body**

Replace `NewApp` (lines 72-88) with:

```go
// NewApp creates the root application model.
func NewApp(clients map[string]*gitlab.Client, hostNames []string, detectedHost, detectedPath string, panels []PanelID, refreshInterval time.Duration) *App {
	activeHost := ""
	if detectedHost != "" {
		activeHost = detectedHost
	} else if len(hostNames) > 0 {
		activeHost = hostNames[0]
	}

	if len(panels) == 0 {
		panels = defaultPanels()
	}

	// Default the active panel to Pipelines if visible, else the first panel.
	active := PanelPipelines
	visible := false
	for _, p := range panels {
		if p == active {
			visible = true
		}
	}
	if !visible {
		active = panels[0]
	}

	return &App{
		clients:         clients,
		hostNames:       hostNames,
		activeHost:      activeHost,
		activePanel:     active,
		detectedHost:    detectedHost,
		detectedPath:    detectedPath,
		panels:          panels,
		refreshInterval: refreshInterval,
		jobsCache:       make(map[int]jobsCacheEntry),
	}
}
```

- [ ] **Step 3: Fix the `ComputeLayout` call**

In `internal/tui/app.go` (currently line 701), change:

```go
		a.layout = ComputeLayout(a.width, a.height, a.activePanel)
```

to:

```go
		a.layout = ComputeLayout(a.width, a.height, a.activePanel, a.panels)
```

- [ ] **Step 4: Build settings in `app.Run` and pass to `NewApp`**

In `internal/app/app.go`, add `"time"` and `"os"` to imports if missing (`os` for stderr warnings; `time` for the interval). Then, inside `Run()` after `buildClients` (after line 23) and before `NewApp`, replace the model-construction block (lines 25-39) with:

```go
	// Auto-detect project from git remote
	var detectedHost, detectedPath string
	remotes := util.DetectGitRemotes()
	for _, r := range remotes {
		if _, ok := clients[r.Host]; ok {
			detectedHost = r.Host
			detectedPath = r.Path
			break
		}
	}

	// Resolve panel configuration and refresh interval from settings.
	panels, warnings := tui.ParsePanels(cfg.Settings.Panels)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "  config: %s\n", w)
	}
	refreshInterval := time.Duration(cfg.Settings.RefreshSeconds()) * time.Second

	fmt.Println("  Launching lazyglab...")
	fmt.Println()

	model := tui.NewApp(clients, hostNames, detectedHost, detectedPath, panels, refreshInterval)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
```

- [ ] **Step 5: Build and run existing tests**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: build PASSES, all tests PASS. (If `app_test.go` calls `NewApp`, update those calls to pass `nil, 0` for the two new args.)

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/app/app.go
git commit -m "feat: wire panel config and refresh interval into the App model"
```

---

## Task 6: Dynamic sidebar rendering + tab/number cycling

**Files:**
- Modify: `internal/tui/app.go` View sidebar block (lines 703-728)
- Modify: `internal/tui/app.go` panel-switch keys (lines 268-285)
- Modify: `internal/tui/app.go` `renderSidePanelSmart`/`renderSidePanel` to show display number

- [ ] **Step 1: Add cycling + numbering helpers**

Add these methods to `internal/tui/app.go` (near `activeListLen`, ~line 686):

```go
// panelIndex returns the position of a panel in the visible list, or -1.
func (a *App) panelIndex(id PanelID) int {
	for i, p := range a.panels {
		if p == id {
			return i
		}
	}
	return -1
}

// panelNumber returns the 1-based display number for a panel (its position in
// the visible list).
func (a *App) panelNumber(id PanelID) int {
	return a.panelIndex(id) + 1
}

// cyclePanel moves the active panel by delta within the visible list (wrapping).
func (a *App) cyclePanel(delta int) {
	n := len(a.panels)
	if n == 0 {
		return
	}
	idx := a.panelIndex(a.activePanel)
	if idx < 0 {
		idx = 0
	}
	a.activePanel = a.panels[(idx+delta+n)%n]
}
```

- [ ] **Step 2: Replace tab/shift-tab and number-key handling**

In `handleKeyMsg` (lines 268-285), replace the `KeyTab`/`KeyShiftTab`/`KeyPanel1..4` cases with:

```go
	case KeyTab, KeyVimRight:
		a.cyclePanel(1)
		return a, a.schedulePreview()
	case KeyShiftTab, KeyVimLeft:
		a.cyclePanel(-1)
		return a, a.schedulePreview()
	case KeyPanel1, KeyPanel2, KeyPanel3, KeyPanel4:
		if n := int(key[0] - '1'); n >= 0 && n < len(a.panels) {
			a.activePanel = a.panels[n]
		}
		return a, a.schedulePreview()
```

(`schedulePreview` is added in Task 8; until then it does not exist — implement Task 8 before building this task, or temporarily `return a, nil`. Recommended: do Tasks 6→7→8 then build. To keep commits green, this task's build step returns `nil` and Task 8 swaps it in. To avoid churn, **implement `schedulePreview` as a stub returning `nil` now** — see Step 3.)

- [ ] **Step 3: Add a temporary `schedulePreview` stub**

Add to `internal/tui/app.go` (replaced fully in Task 8):

```go
// schedulePreview is fully implemented in the job-preview task; stub for now.
func (a *App) schedulePreview() tea.Cmd { return nil }
```

- [ ] **Step 4: Replace the sidebar construction in `View`**

Replace lines 703-728 (from `// Pipeline/Jobs panel: swap content` through the `sidebar := lipgloss.JoinVertical(...)` call) with:

```go
			var panelViews []string
			for _, id := range a.panels {
				panelViews = append(panelViews, a.renderPanel(id))
			}
			sidebar := lipgloss.JoinVertical(lipgloss.Left, panelViews...)
```

- [ ] **Step 5: Add `renderPanel` and thread the display number**

Add to `internal/tui/app.go`:

```go
// renderPanel renders a single sidebar panel by ID, handling the Pipelines↔Jobs
// content swap.
func (a *App) renderPanel(id PanelID) string {
	switch id {
	case PanelProjects:
		return a.renderSidePanelSmart(PanelProjects, "Projects", a.projectItems(), a.collapsedProjectLine(), a.cursor[PanelProjects])
	case PanelPipelines:
		title := a.pipelinePanelTitle()
		items := a.pipelineItems()
		collapsed := a.collapsedPipelineLine()
		cursor := a.cursor[PanelPipelines]
		if a.viewingJobs {
			title = a.jobsPanelTitle()
			var jobToDisplay []int
			items, jobToDisplay = a.jobItems()
			collapsed = a.collapsedJobLine()
			if a.jobCursor >= 0 && a.jobCursor < len(jobToDisplay) {
				cursor = jobToDisplay[a.jobCursor]
			} else {
				cursor = 0
			}
		}
		return a.renderSidePanelSmart(PanelPipelines, title, items, collapsed, cursor)
	case PanelMergeRequests:
		return a.renderSidePanelSmart(PanelMergeRequests, "Merge Requests", a.mrItems(), a.collapsedMRLine(), a.cursor[PanelMergeRequests])
	case PanelIssues:
		return a.renderSidePanelSmart(PanelIssues, "Issues", a.issueItems(), a.collapsedIssueLine(), a.cursor[PanelIssues])
	}
	return ""
}
```

In `renderSidePanelSmart` (line ~806) change the title line to use the display number:

```go
		titleText := fmt.Sprintf("[%d] %s", a.panelNumber(id), title)
```

and in `renderSidePanel` (line ~859) likewise:

```go
	titleText := fmt.Sprintf("[%d] %s", a.panelNumber(id), title)
```

- [ ] **Step 6: Build, test, and drive**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

Drive with a hidden panel to verify visually:

```bash
mkdir -p /tmp/lgtest && cat > /tmp/lgtest/config.yml <<'YAML'
default_host: gitlab.example.com
hosts:
  gitlab.example.com: {token: "REPLACE_OR_COPY_FROM_REAL"}
settings:
  panels: [projects, pipelines, merge_requests]   # issues hidden
YAML
# copy the real token in:
cp ~/.config/lazyglab/config.yml /tmp/lgtest/config.yml  # then add the settings block by hand
LAZYGLAB_CONFIG=/tmp/lgtest/config.yml tmux new-session -d -s lgt -x 120 -y 40 'lazyglab'; sleep 5
tmux capture-pane -t lgt -p; tmux kill-session -t lgt
```
Expected: three panels, no Issues panel, numbered `[1] [2] [3]`.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: render sidebar from visible panel list; dynamic cycling and numbering"
```

---

## Task 7: Pipeline panel redesign (aligned, colored)

**Files:**
- Modify: `internal/tui/app.go` `pipelineItems` (lines 1322-1346)
- Add helpers `padRight`, `statusIcon` to `internal/tui/app.go`

- [ ] **Step 1: Add padding + colored-icon helpers**

Add to `internal/tui/app.go` (near `truncate`, ~line 1913):

```go
// padRight pads s with spaces to the given display width (no-op if already wider).
func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// statusIcon returns the pipeline/job status icon colored by status and padded
// to 2 display cells so 1- and 2-cell glyphs align in a column.
func (a *App) statusIcon(status string) string {
	icon := PipelineStatusIcon(status)
	colored := lipgloss.NewStyle().Foreground(PipelineStatusColor(status)).Render(icon)
	if lipgloss.Width(icon) < 2 {
		colored += " "
	}
	return colored
}
```

- [ ] **Step 2: Rewrite `pipelineItems`**

Replace `pipelineItems` (lines 1322-1346) with:

```go
func (a *App) pipelineItems() []string {
	const refWidth = 14
	items := make([]string, len(a.pipelines))
	for i, p := range a.pipelines {
		title := p.CommitTitle
		if title == "" {
			title = p.Ref
		}
		t := util.TimeAgoShort(p.CreatedAt) // fixed width 4
		icon := a.statusIcon(p.Status)      // 2 cells, colored
		if a.activeBranch != nil {
			// Branch shown in the panel title; drop the ref column.
			items[i] = fmt.Sprintf("%s %s %s", t, icon, title)
		} else {
			ref := padRight(truncate(p.Ref, refWidth), refWidth)
			items[i] = fmt.Sprintf("%s %s %s %s", t, icon, ref, title)
		}
	}
	return items
}
```

- [ ] **Step 3: Build and test**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 4: Drive and eyeball alignment**

```bash
tmux new-session -d -s lgt -x 120 -y 40 'lazyglab'; sleep 5
tmux send-keys -t lgt "j" Enter; sleep 5     # select a project with pipelines
tmux capture-pane -t lgt -p; tmux kill-session -t lgt
```
Expected: pipeline rows with aligned time/icon/ref columns and colored status icons; no em-dash.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: redesign pipeline list with aligned columns and colored icons"
```

---

## Task 8: Auto-refresh (active panel, debounced job preview)

**Files:**
- Modify: `internal/tui/messages.go` (new message types)
- Modify: `internal/tui/app.go` `Init`, `Update`, add refresh + preview logic; replace `schedulePreview` stub

- [ ] **Step 1: Add message types**

Append to `internal/tui/messages.go`:

```go
// tickMsg drives the auto-refresh timer.
type tickMsg struct{}

// previewTickMsg fires after the hover debounce; gen guards against stale ticks.
type previewTickMsg struct{ gen int }

// previewJobsLoadedMsg carries jobs fetched for the hover preview.
type previewJobsLoadedMsg struct {
	pipelineID int
	gen        int
	jobs       []Job
	err        error
}
```

Note: `Job` here is `gitlab.Job` — if `messages.go` does not import gitlab, use the fully-qualified type. Check the file's existing imports; the existing `JobsLoadedMsg` already references jobs, so mirror its type exactly (`[]gitlab.Job`).

- [ ] **Step 2: Implement `Init` with the tick**

Replace `Init` (lines 90-92) with:

```go
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.loadProjects()}
	if a.refreshInterval > 0 {
		cmds = append(cmds, a.tickCmd())
	}
	return tea.Batch(cmds...)
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(a.refreshInterval, func(time.Time) tea.Msg { return tickMsg{} })
}
```

- [ ] **Step 3: Handle `tickMsg` and preview messages in `Update`**

Add these cases to the `switch msg := msg.(type)` in `Update` (before the final `return a, nil`):

```go
	case tickMsg:
		var cmd tea.Cmd
		if a.canAutoRefresh() {
			cmd = a.autoRefreshCmd()
		}
		return a, tea.Batch(cmd, a.tickCmd())

	case previewTickMsg:
		if msg.gen != a.previewGen {
			return a, nil
		}
		idx := a.cursor[PanelPipelines]
		if idx < 0 || idx >= len(a.pipelines) {
			return a, nil
		}
		return a, a.loadPreviewJobs(a.pipelines[idx].ID, msg.gen)

	case previewJobsLoadedMsg:
		if msg.err == nil {
			a.jobsCache[msg.pipelineID] = jobsCacheEntry{jobs: msg.jobs, at: time.Now()}
		}
		if msg.gen == a.previewGen && msg.pipelineID == a.previewPipelineID {
			a.previewLoading = false
			if msg.err == nil {
				a.previewJobs = msg.jobs
			}
		}
		return a, nil
```

- [ ] **Step 4: Add refresh + preview logic; replace the stub**

Remove the `schedulePreview` stub from Task 6 and add:

```go
// canAutoRefresh reports whether a background refresh is currently appropriate.
func (a *App) canAutoRefresh() bool {
	if a.activeProject == nil {
		return false
	}
	if a.pendingConfirm != nil || a.showHelp || a.showBranchPicker {
		return false
	}
	if a.viewingJobs && a.jobTrace != "" { // reading a log; don't disturb
		return false
	}
	return true
}

// autoRefreshCmd refreshes the data behind the active view.
func (a *App) autoRefreshCmd() tea.Cmd {
	if a.activePanel == PanelPipelines && a.viewingJobs {
		return a.loadJobs()
	}
	// Refreshing pipelines invalidates the hover cache so previews stay fresh.
	if a.activePanel == PanelPipelines {
		a.jobsCache = make(map[int]jobsCacheEntry)
	}
	return a.refreshActivePanel()
}

// schedulePreview loads (or debounces a load of) the selected pipeline's jobs
// for the detail-panel preview. Cache hits render immediately; otherwise a 400ms
// debounce tick is returned, guarded by previewGen.
func (a *App) schedulePreview() tea.Cmd {
	if a.activePanel != PanelPipelines || a.viewingJobs {
		return nil
	}
	idx := a.cursor[PanelPipelines]
	if idx < 0 || idx >= len(a.pipelines) {
		return nil
	}
	pid := a.pipelines[idx].ID
	a.previewPipelineID = pid

	if e, ok := a.jobsCache[pid]; ok && time.Since(e.at) < 30*time.Second {
		a.previewJobs = e.jobs
		a.previewLoading = false
		return nil
	}

	a.previewGen++
	gen := a.previewGen
	a.previewJobs = nil
	a.previewLoading = true
	return tea.Tick(400*time.Millisecond, func(time.Time) tea.Msg { return previewTickMsg{gen: gen} })
}

// loadPreviewJobs fetches jobs for the hover preview.
func (a *App) loadPreviewJobs(pipelineID, gen int) tea.Cmd {
	if a.activeProject == nil {
		return nil
	}
	client := a.clients[a.activeHost]
	projectID := a.activeProject.ID
	return func() tea.Msg {
		jobs, err := client.ListPipelineJobs(projectID, pipelineID)
		return previewJobsLoadedMsg{pipelineID: pipelineID, gen: gen, jobs: jobs, err: err}
	}
}
```

- [ ] **Step 5: Trigger preview on navigation and after loads**

In `handleKeyMsg`, the generic navigation section (lines 319-357) currently returns `a, nil`. For the up/down/top/bottom/half-page cases, change the return to `a, a.afterCursorMove()`. Add the helper:

```go
// afterCursorMove returns any command to run after the cursor moves (currently
// the pipeline hover-preview debounce).
func (a *App) afterCursorMove() tea.Cmd {
	return a.schedulePreview()
}
```

Concretely, change these five returns in the nav block to `return a, a.afterCursorMove()`:
`isNavigateUp`, `isNavigateDown`, `KeyTop`, `KeyBottom`, `KeyHalfDown`, `KeyHalfUp`.

In the `PipelinesLoadedMsg` case (lines 167-176), before `return a, nil` add:

```go
		return a, a.schedulePreview()
```
(replacing the existing `return a, nil`).

In the `ProjectSelectedMsg` case, the batch already loads pipelines; the preview will trigger from `PipelinesLoadedMsg`. No change needed there.

- [ ] **Step 6: Render the preview in `pipelineDetail`**

In `pipelineDetail` (lines 1407-1438), before the final `return strings.Join(lines, "\n")`, insert (after the WebURL line):

```go
	// Hover preview: the selected pipeline's jobs.
	if a.previewPipelineID == p.ID {
		if a.previewLoading {
			lines = append(lines, "", HelpDescStyle.Render("Loading jobs…"))
		} else if len(a.previewJobs) > 0 {
			lines = append(lines, "", HelpDescStyle.Render("Jobs:"))
			for _, j := range a.previewJobs {
				dur := ""
				if j.Duration > 0 {
					dur = fmt.Sprintf("  (%ds)", int(j.Duration))
				}
				lines = append(lines, fmt.Sprintf("  %s %s  %s%s", a.statusIcon(j.Status), j.Name, j.Status, dur))
			}
		}
	}
```

- [ ] **Step 7: Build and test**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 8: Drive — verify preview + refresh**

```bash
tmux new-session -d -s lgt -x 140 -y 45 'lazyglab'; sleep 5
tmux send-keys -t lgt "j" Enter; sleep 6          # select active project
tmux send-keys -t lgt "2"; sleep 1                # pipelines panel
tmux send-keys -t lgt "j"; sleep 1                # move cursor; wait > 400ms
tmux capture-pane -t lgt -p; tmux kill-session -t lgt
```
Expected: right detail panel shows the selected pipeline's jobs (or "Loading jobs…" then jobs). Rapidly pressing `j j j` should not flood requests (no visible lag/errors).

- [ ] **Step 9: Commit**

```bash
git add internal/tui/messages.go internal/tui/app.go
git commit -m "feat: active-panel auto-refresh and debounced pipeline job preview"
```

---

## Task 9: Polish — cursor clamping on refresh

**Files:**
- Modify: `internal/tui/app.go` loaded-message handlers (MRs/Pipelines/Issues/Projects)

A refresh can shrink a list under the cursor. Clamp after each load.

- [ ] **Step 1: Add a clamp helper**

Add to `internal/tui/app.go`:

```go
// clampCursor keeps the given panel's cursor within its list bounds.
func (a *App) clampCursor(id PanelID) {
	n := 0
	switch id {
	case PanelProjects:
		n = len(a.projects)
	case PanelMergeRequests:
		n = len(a.mrs)
	case PanelPipelines:
		n = len(a.pipelines)
	case PanelIssues:
		n = len(a.issues)
	}
	if a.cursor[id] >= n {
		a.cursor[id] = n - 1
	}
	if a.cursor[id] < 0 {
		a.cursor[id] = 0
	}
}
```

- [ ] **Step 2: Call it in each loaded handler**

In `ProjectsLoadedMsg` (after `a.projects = msg.Projects`), `MRsLoadedMsg` (after `a.mrs = msg.MRs`), `PipelinesLoadedMsg` (after `a.pipelines = msg.Pipelines`), and `IssuesLoadedMsg` (after `a.issues = msg.Issues`), add `a.clampCursor(<PanelID>)` for the matching panel.

- [ ] **Step 3: Build and test**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "fix: clamp panel cursors after (re)loading lists"
```

---

## Task 10: Polish — help alignment, status-bar width, load feedback, log word-wrap

**Files:**
- Modify: `internal/tui/app.go` `renderHelp` (~1133-1190), `renderStatusBar` (~982-1010), loaded handlers, `jobTraceView` (~1486-1536)

- [ ] **Step 1: Help overlay — left-align the block, then center as a unit**

In `renderHelp`, replace the final three lines (from `content := strings.Join(lines, "\n")` to the `return`) with:

```go
	block := lipgloss.NewStyle().Align(lipgloss.Left).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, block)
```

- [ ] **Step 2: Status bar — measure with `lipgloss.Width`**

In `renderStatusBar`, replace:

```go
	gap := a.width - len(left) - len(right) - 2
```

with:

```go
	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
```

- [ ] **Step 3: Load feedback for MRs/pipelines/issues**

In the loaded handlers, set a status message like Projects already does:

- `MRsLoadedMsg` (success branch): `a.statusText = fmt.Sprintf("Loaded %d merge requests", len(msg.MRs)); a.statusIsErr = false`
- `PipelinesLoadedMsg` (success branch): `a.statusText = fmt.Sprintf("Loaded %d pipelines", len(msg.Pipelines)); a.statusIsErr = false`
- `IssuesLoadedMsg` (success branch): `a.statusText = fmt.Sprintf("Loaded %d issues", len(msg.Issues)); a.statusIsErr = false`

- [ ] **Step 4: Log word-wrap without splitting words**

In `jobTraceView`, replace the inner word-wrap loop:

```go
		// Word-wrap long lines
		for len(line) > contentWidth {
			cleaned = append(cleaned, line[:contentWidth])
			line = line[contentWidth:]
		}
		cleaned = append(cleaned, line)
```

with a rune- and word-aware wrap:

```go
		for _, wrapped := range wrapLine(line, contentWidth) {
			cleaned = append(cleaned, wrapped)
		}
```

Add the helper to `internal/tui/app.go`:

```go
// wrapLine wraps a single (ANSI-stripped) line to width w, breaking on spaces
// when possible and falling back to a hard rune break for overlong words.
func wrapLine(line string, w int) []string {
	if w < 1 {
		w = 1
	}
	if lipgloss.Width(line) <= w {
		return []string{line}
	}
	var out []string
	var cur []rune
	curW := 0
	flush := func() {
		out = append(out, string(cur))
		cur = cur[:0]
		curW = 0
	}
	for _, word := range strings.Split(line, " ") {
		wl := len([]rune(word))
		// Hard-break a single word longer than the width.
		for wl > w {
			if curW > 0 {
				flush()
			}
			r := []rune(word)
			out = append(out, string(r[:w]))
			word = string(r[w:])
			wl = len([]rune(word))
		}
		space := 0
		if curW > 0 {
			space = 1
		}
		if curW+space+wl > w {
			flush()
			space = 0
		}
		if space == 1 {
			cur = append(cur, ' ')
			curW++
		}
		cur = append(cur, []rune(word)...)
		curW += wl
	}
	if curW > 0 || len(out) == 0 {
		flush()
	}
	return out
}
```

- [ ] **Step 5: Add a unit test for `wrapLine`**

Add to `internal/tui/panels_test.go` (or a new `wrap_test.go`):

```go
func TestWrapLine(t *testing.T) {
	got := wrapLine("the quick brown fox", 9)
	for _, l := range got {
		if len([]rune(l)) > 9 {
			t.Errorf("line %q exceeds width 9", l)
		}
	}
	if len(got) < 2 {
		t.Errorf("expected wrapping into multiple lines, got %v", got)
	}

	// Overlong single word hard-breaks.
	got = wrapLine("supercalifragilistic", 5)
	if len(got) < 4 {
		t.Errorf("expected hard break into >=4 chunks, got %v", got)
	}
}
```

- [ ] **Step 6: Build, test, drive**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

Drive to confirm the help overlay is left-aligned:

```bash
tmux new-session -d -s lgt -x 120 -y 40 'lazyglab'; sleep 4
tmux send-keys -t lgt "?"; sleep 1
tmux capture-pane -t lgt -p; tmux kill-session -t lgt
```
Expected: the key column is vertically aligned (no ragged left edge).

- [ ] **Step 7: Commit**

```bash
git add internal/tui/app.go internal/tui/panels_test.go
git commit -m "fix: help alignment, unicode status bar width, load feedback, log word-wrap"
```

---

## Task 11: Documentation + final verification

**Files:**
- Modify: `README.md` (settings section), `TODO.md` (check off delivered items)

- [ ] **Step 1: Document settings in README**

Add a "Configuration" subsection to `README.md` describing the `settings:` block (panels, refresh_interval) with the example from the spec.

- [ ] **Step 2: Update TODO.md**

Check off: auto-refresh timer, loading feedback, and note configurable panels + pipeline redesign under a new "Delivered" area as appropriate.

- [ ] **Step 3: Full verification**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go test -race ./...`
Expected: build clean, vet clean, `gofmt -l` prints nothing, `golangci-lint` 0 issues, tests PASS.

- [ ] **Step 4: End-to-end drive against real GitLab**

```bash
tmux new-session -d -s lgt -x 140 -y 45 'lazyglab'; sleep 5
tmux send-keys -t lgt "j" Enter; sleep 6
tmux send-keys -t lgt "2"; sleep 1; tmux send-keys -t lgt "j"; sleep 1
tmux capture-pane -t lgt -p
tmux send-keys -t lgt "?"; sleep 1; tmux capture-pane -t lgt -p
tmux kill-session -t lgt
```
Expected: aligned colored pipeline list, job preview in the detail panel, aligned help overlay.

- [ ] **Step 5: Commit**

```bash
git add README.md TODO.md
git commit -m "docs: document panel/refresh settings and update TODO"
```

---

## Self-review notes

- **Spec coverage:** settings schema (T2), panels hide/reorder + projects-forced (T3, T5, T6), dynamic layout (T4), auto-refresh active-panel + pause (T8), pipeline redesign + TimeAgoShort (T1, T7), job preview debounce+cache in detail panel (T8), polish #2–5 (T10), cursor safety on refresh (T9), docs (T11). All spec sections mapped.
- **Type consistency:** `jobsCacheEntry`, `previewJobsLoadedMsg`, `schedulePreview`, `autoRefreshCmd`, `canAutoRefresh`, `statusIcon`, `padRight`, `wrapLine`, `panelIndex`, `panelNumber`, `cyclePanel`, `renderPanel`, `ParsePanels`, `Settings.RefreshSeconds` are defined once and referenced consistently. `previewJobsLoadedMsg.jobs` uses `[]gitlab.Job` (match existing `JobsLoadedMsg`).
- **Ordering caveat:** Task 6 introduces a `schedulePreview` stub that Task 8 replaces — build stays green between tasks. Do not skip the stub.
```
