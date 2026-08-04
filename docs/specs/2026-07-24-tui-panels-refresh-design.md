# Design: configurable panels, auto-refresh & pipeline panel redesign

Date: 2026-07-24
Status: approved (pending written-spec review)

## Goal

Make lazyglab's TUI more usable and adaptable:

1. **Configurable panels** — users choose which of the four sidebar panels are
   shown and in what order (someone who never uses Issues can hide it).
2. **Auto-refresh** — the active panel's data refreshes on a timer so a running
   pipeline doesn't look frozen, without spamming the GitLab API.
3. **Pipeline panel redesign** — the list is currently ragged and hard to scan;
   make it a clean, aligned, color-coded list, and preview a pipeline's jobs in
   the detail panel on hover.
4. **Polish** — a set of small correctness/readability fixes found while
   reviewing the running app.

All settings live in the existing `~/.config/lazyglab/config.yml`. With no
`settings:` block, behaviour is identical to today (backwards compatible).

## Non-goals (separate future work)

- Search/filter overlay (`/`)
- Full MR / Issue detail views on Enter
- Richer project/MR detail content
- Per-host settings (settings are global)

## Configuration schema

New optional `settings:` section in `~/.config/lazyglab/config.yml`:

```yaml
default_host: gitlab.example.com
hosts:
  gitlab.example.com:
    token: "…"
settings:
  panels: [projects, pipelines, merge_requests, issues]  # order = display order; omit to hide
  refresh_interval: 30                                     # seconds; 0 disables auto-refresh
```

Panel config names: `projects`, `pipelines`, `merge_requests`, `issues`.

### Validation rules

- Missing `settings` → all four panels, default order, refresh 30s.
- Missing `panels` → all four, default order.
- `projects` is always forced present: if omitted or unknown, it is prepended.
  (Projects is the selector that drives every other panel; hiding it is not
  supported.)
- Unknown names and duplicates are dropped; a warning is printed to stderr
  before the TUI starts.
- `refresh_interval`: `0` disables; negative or missing → default 30. A minimum
  floor of 5s is enforced to protect the API (values 1–4 are clamped to 5).

The `Config`/`HostConfig` structs in `internal/app/config.go` gain a `Settings`
field. Parsing/validation of settings lives in a small dedicated unit so it is
independently testable.

## Component design

### 1. Panel registry — `internal/tui/panels.go` (new)

`app.go` is already ~1900 lines; panel metadata and config parsing move here to
keep it from growing further.

- `PanelID` stays the stable identity enum (`PanelProjects`=0 … `PanelIssues`=3).
  All switch statements, the `cursor [4]int` array, and detail rendering keep
  keying off `PanelID` — identity does not change, only which panels are visible
  and their order.
- `panelConfigName(PanelID) string` / `panelIDFromName(string) (PanelID, bool)`
  map between config names and IDs.
- `ParsePanels(names []string) ([]PanelID, []string)` returns the ordered
  visible list plus any warnings (unknown/duplicate). Guarantees `projects`
  first-or-present per the rules above.

### 2. App model changes — `internal/tui/app.go`

- New field `panels []PanelID` — the ordered, visible panels (from config).
- New fields for refresh: `refreshInterval time.Duration`.
- New fields for job preview: `previewJobs []gitlab.Job`, `previewPipelineID int`,
  `jobsCache map[int]jobsCacheEntry` (keyed by pipeline ID), and a debounce
  generation counter `previewGen int`.
- **Tab / h / l cycling**: replace `(activePanel+1) % 4` with index-into-`panels`
  arithmetic so cycling only visits visible panels in configured order.
- **Number keys `1..N`**: select the N-th *visible* panel (not the fixed ID), so
  they always map to what the user sees. Keys beyond `len(panels)` are ignored.
- **View()**: iterate `a.panels` to build the sidebar with `JoinVertical`
  instead of four hardcoded `renderSidePanelSmart` calls. The Pipelines↔Jobs
  content swap is preserved.

### 3. Layout — `internal/tui/layout.go`

- `ComputeLayout` takes the visible panel list (`[]PanelID`) instead of assuming
  four. `PanelHeights` stays a `[4]int` keyed by `PanelID`; only visible panels
  get non-zero heights, distributed evenly with remainder spread.
- The "Projects collapses to 3 lines when not focused" special case stays
  (Projects is always visible). When Projects is not in focus, the remaining
  height is split among the other *visible* panels.
- Existing `layout_test.go` is updated to pass the panel list and to cover
  hidden/reordered configurations.

### 4. Auto-refresh

- `Init()` schedules a `tea.Tick(refreshInterval)` when `refreshInterval > 0`.
- On each tick: if a project is selected AND no overlay is active (help,
  confirm, branch picker) AND not currently reading a job trace, issue the load
  command for the **active panel only** (projects/pipelines/MRs/issues, or jobs
  when in the jobs view). Then reschedule the next tick.
- Refresh preserves cursor position (existing load handlers already clamp the
  cursor; jobs handler already preserves `jobCursor`).
- `refresh_interval: 0` → no tick is ever scheduled (zero overhead).
- A `tickMsg` type carries the schedule; ticks reschedule themselves so the
  interval is stable.

### 5. Pipeline panel redesign

**Fixed-width, aligned, color-coded single-line rows.** New helper in `app.go`
(or `panels.go`) builds each row from aligned columns using `lipgloss.Width`
for unicode/ANSI-safe padding:

```
<time> <icon> <ref>            <commit title…>
 <1m   ◉     main             fix(#0): stop double-counting…
 26m   ✓     main             test(#0): fix the flaky timezone…
  3h   ✗     feature/long-branch… feat: add the payment gateway…
```

- **Time** — fixed width 4, right-aligned. Requires fixing `TimeAgoShort`
  (`internal/util/timeago.go`) to return a consistent width (currently `" <1m"`
  and `"12mo"` are 4 chars while `"3h"`/`"26m"` are 3). All outputs are padded to
  a uniform width.
- **Icon** — colored via `PipelineStatusColor(status)` and padded to 2 display
  cells so 2-cell glyphs (e.g. `❚❚`) don't shift the columns.
- **Ref** — fixed width (target ~14), left-aligned, truncated with `…`.
- **Commit title** — fills the remaining width, truncated with `…`.
- **No em-dash** — spacing separates columns.
- **Branch filter active** — the ref column is dropped (as today), giving the
  title more room.

### 6. Job preview in the detail panel (hover, debounced + cached)

When the Pipelines panel is focused and NOT in the full jobs view, the detail
(right) panel shows the selected pipeline's metadata **plus its job list**.

- **Debounce**: moving the cursor sets `previewPipelineID` and bumps
  `previewGen`, then schedules a `tea.Tick(400ms)` carrying that generation.
  When the tick fires, jobs are fetched only if the generation still matches
  (i.e. the cursor has rested). Fast scrolling fires zero fetches.
- **Cache**: fetched jobs are stored in `jobsCache[pipelineID]` with a 30s TTL
  (aligned with auto-refresh). A cache hit renders immediately with no request.
- **Rendering**: the detail shows pipeline status/ref/commit/time, a separator,
  then the jobs (icon + name + status/duration). If jobs are still loading,
  show "Loading jobs…".
- **Enter** still opens the existing full jobs view (with job detail and log
  trace) — hover is preview, Enter is drill-in.
- Auto-refresh of the Pipelines panel invalidates the cache for the visible
  pipelines so the preview stays fresh.

### 7. Polish fixes

- **Help overlay alignment** (`app.go` `renderHelp`): render the help body as a
  single left-aligned fixed-width block, then center that block as a unit
  (fixes the current per-line centering that makes keys ragged).
- **Load feedback**: MR/pipeline/issue loads set a status message ("Loaded N …")
  like projects already do; a lightweight "loading…" indicator uses the existing
  `loading` flag in the status bar.
- **Status bar width** (`app.go` `renderStatusBar`): use `lipgloss.Width`
  instead of `len()` so unicode project names don't misalign the right side.
- **Log word-wrap** (`app.go` `jobTraceView`): wrap on width boundaries without
  splitting inside a word where avoidable, instead of the current hard byte cut.

## Data flow

```
config.yml (settings) ──▶ app.resolveConfig ──▶ tui.NewApp(clients, hosts,
    detected, panels, refreshInterval)
                                   │
                                   ▼
                             App.Init ──▶ loadProjects + (tick if interval>0)
                                   │
        key/tick/data msgs ──▶ App.Update ──▶ tea.Cmd (async API calls)
                                   │
                                   ▼
                              App.View (iterate a.panels)
```

## Error handling

- Config/settings parse errors: invalid settings never crash startup — unknown
  panels/durations are corrected with warnings to stderr; a completely malformed
  `settings` block falls back to defaults with a warning.
- API errors during auto-refresh: surfaced in the status bar (existing pattern);
  a failed refresh does not stop future ticks.
- Job-preview fetch errors: shown as "Failed to load jobs" in the detail panel;
  do not disturb the pipeline list or cursor.

## Testing

- `internal/app/config_test.go`: settings parsing — defaults, omitted block,
  `refresh_interval` clamping (0 disables, 1–4 → 5, negative → default).
- `internal/tui/panels_test.go` (new): `ParsePanels` — default, hide Issues,
  reorder, unknown/duplicate dropped, projects forced present, warnings emitted.
- `internal/tui/layout_test.go`: layout for 4/3/2 visible panels and reordered
  sets; heights sum to usable height; Projects-collapse still applies.
- `internal/util/timeago_test.go`: `TimeAgoShort` returns a uniform width across
  all branches.
- Tab-cycling / number-key selection over a reduced/reordered panel set.
- Job cache TTL and debounce-generation logic (pure helpers kept testable).

## Rollout / compatibility

Fully backwards compatible: existing configs (no `settings:`) behave exactly as
before. New behaviour is opt-in via config, except the pipeline-panel redesign
and polish fixes, which apply unconditionally (they change appearance, not
configuration).
