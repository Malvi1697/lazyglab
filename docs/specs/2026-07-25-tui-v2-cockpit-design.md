# Design: lazyglab v2 — cockpit with switchable views

Date: 2026-07-25
Status: approved (pending written-spec review)

## Motivation

The v1 TUI copies lazygit's multi-panel layout: four sidebar panels (Projects,
Pipelines, MRs, Issues) plus a detail pane, all visible at once. This is the
wrong metaphor here.

lazygit's panels form a **drill-down hierarchy** of one repository's state
(branches → commits → files → diff), so showing them together is natural. Our
four panels are **siblings, not a hierarchy** — four unrelated lists side by
side. Showing them all at once wastes space, cramps each list, and forces three
irrelevant panels on screen at all times. It also has no commit history view,
which is a primary thing users want.

v2 restructures the app as a **cockpit**: a persistent context header, one
full-screen view at a time (switchable by key), and shared modal overlays.
This is the model of `k9s`, `gh dash`, and `lazydocker` — keyboard-driven,
focused, full-width per view.

## Goals

- One focused, full-screen view at a time; switch instantly by key.
- Add an **Overview** dashboard (default) and a **Commits** view (both new).
- Give each entity a view with a layout suited to it (not a universal grid).
- Break the ~1900-line `internal/tui/app.go` into a thin shell plus one file
  per view, under `internal/tui/views/`.
- Reuse the existing GitLab API layer, actions, setup/config, styles, job-log
  viewer, and auto-refresh.

## Non-goals (separate future work)

- MR line-level review / commenting
- Create-MR / create-issue wizards
- Diff viewer
- Web-UI parity beyond the views listed here

## Layout

```
┌ Context bar ──────────────────────────────────────────────┐  row 0
│ main ⌄  idiskgolf                        ● pipeline: passing│
├ Tabs ─────────────────────────────────────────────────────┤  row 1
│ [Overview] Pipelines  MRs  Issues  Commits                 │
├ View body (full width, remaining height) ─────────────────┤  rows 2..h-2
│                     active view renders here                │
├ Footer ───────────────────────────────────────────────────┤  row h-1
│ q Quit · ? Help · 1-5 View · p Project · b Branch · r ↻    │
└────────────────────────────────────────────────────────────┘
```

- **Context bar** — active branch + project (left), a global status indicator
  such as the latest pipeline result (right). Persistent across views.
- **Tabs** — the enabled views in configured order; the active one highlighted.
- **Footer** — global hints plus the active view's `KeyHints()`.
- Overlays (project switcher, branch picker, help, confirm) render centered on
  top of everything.

## Architecture

### The `View` interface

Each view is a self-contained component in `internal/tui/views/`:

```go
// View is one full-screen area of the cockpit.
type View interface {
    // Focus is called when the view becomes active (e.g. to trigger a load).
    Focus() tea.Cmd
    // Update handles a message and may return a command. Views mutate in place.
    Update(msg tea.Msg) tea.Cmd
    // Body renders the view into the given content area (already sized).
    Body(width, height int) string
    // Title is the tab label.
    Title() string
    // KeyHints are footer hints specific to this view.
    KeyHints() []KeyHint
}
```

### Shared session context

Views need the active client/project/branch. A single shared pointer is passed
to every view at construction:

```go
type Context struct {
    Client  *gitlab.Client
    Project *gitlab.Project // nil until a project is selected
    Branch  *gitlab.Branch  // nil = default branch / all
}
```

When the shell changes the project or branch (via an overlay), it updates the
shared `Context` and calls `Focus()` on the active view to reload. Views read
`Context` but never mutate it.

### The shell (`App`)

`internal/tui/app.go` becomes a thin shell that:

- owns `clients`, the shared `*Context`, the ordered `views []View`, the active
  view index, and the overlay state (project switcher / branch picker / help /
  confirm);
- handles global keys (view switch `1..5`/Tab, `p` project, `b` branch, `?`
  help, `r` refresh, `q` quit) and overlays;
- delegates all other messages/keys to the active view's `Update`;
- composes the frame: context bar + tabs + active view `Body` + footer, with any
  overlay placed on top;
- drives auto-refresh: on each tick, calls a refresh command on the active view
  (paused while an overlay is open).

### Views

Each is a file in `internal/tui/views/`:

1. **overview.go — Overview (default).**
   Top half: recent commits (`sha` short, CI icon, title, author, time).
   Bottom: three summary columns — Pipelines / MRs / Issues (counts + a few most
   recent). `Enter` (or a key per section) switches to that full view. Read-only.

2. **pipelines.go — Pipelines.**
   Master list (left ~45%): time, colored status icon, commit title.
   Detail (right): pipeline metadata + its jobs grouped by stage. `Enter` on the
   list loads jobs into focus; `Enter` on a job opens the log viewer (reused).
   Actions: run (`p`), retry (`R`), cancel (`C`), play (`p` in job focus), open
   (`o`).

3. **mrs.go — Merge Requests.**
   List (left): `!iid`, draft marker, title, pipeline icon.
   Detail (right): description, source→target, author, pipeline, approvals.
   Actions: approve (`a`), merge (`m`), open (`o`).

4. **issues.go — Issues.**
   List + detail (author, assignees, labels, description). Close/reopen (`c`),
   open (`o`).

5. **commits.go — Commits (new).**
   List: short sha, CI status icon, title, author, relative time.
   Detail: full message, author/committer, stats, associated pipeline status.
   Open (`o`).

### Overlays

Modal, shared, rendered centered over the frame:

- **Project switcher** — replaces the old Projects panel; opened with `p`. Lists
  projects, filter-as-you-type (nice-to-have), Enter selects → updates `Context`
  and refreshes the active view.
- **Branch picker** — `b`, as today.
- **Help** — `?`, left-aligned block (as fixed in v1).
- **Confirm** — reused for destructive actions.

## New GitLab API

Add to `internal/gitlab/`:

- `ListCommits(projectID int, ref string) ([]Commit, error)` — wraps
  `Commits.ListCommits` with `RefName` and a bounded `PerPage`. Domain type
  `Commit{ ShortID, Title, AuthorName, CreatedAt, WebURL, Status }`.
- CI status per commit: GitLab's list-commits response does not reliably include
  pipeline status, so status is resolved by mapping commits to recent pipelines
  by SHA (reuse the pipelines list already fetched for the project) rather than
  an extra call per commit. Commits without a matching pipeline show no icon.

## Configuration

`~/.config/lazyglab/config.yml` `settings:` block evolves:

```yaml
settings:
  views: [overview, pipelines, mrs, issues, commits]  # enabled tabs, in order
  default_view: overview                               # initial view
  refresh_interval: 30                                 # unchanged
```

- `views` — which view tabs to show and their order. Valid names: `overview`,
  `pipelines`, `mrs`, `issues`, `commits`. Unknown/duplicate dropped with a
  warning. Empty/absent → all five in default order.
- `default_view` — the view shown at launch. Absent or not in `views` → the
  first enabled view (normally `overview`).
- `refresh_interval` — unchanged (0 disables; 1–4 clamp to 5; else value; absent
  → 30).
- **Migration:** the v1 `panels` key is obsolete. If present, it is ignored with
  a one-line stderr warning pointing to `views`. No crash.

## File structure

| File | Responsibility | Action |
|---|---|---|
| `internal/tui/app.go` | Thin shell: state, global keys, overlays, frame, router | Rewrite (much smaller) |
| `internal/tui/view.go` | `View` interface, `KeyHint`, `Context`, view registry/parsing | Create |
| `internal/tui/frame.go` | Context bar, tabs, footer rendering | Create |
| `internal/tui/views/overview.go` | Overview dashboard | Create |
| `internal/tui/views/pipelines.go` | Pipelines view (list + jobs + log) | Create (port from app.go) |
| `internal/tui/views/mrs.go` | MRs view | Create (port) |
| `internal/tui/views/issues.go` | Issues view | Create (port) |
| `internal/tui/views/commits.go` | Commits view | Create (new) |
| `internal/tui/components/box.go` | `renderBox`, list rendering helpers | Create (extract) |
| `internal/tui/components/text.go` | `truncate`, `padRight`, `wrapLine`, `statusIcon` | Create (extract) |
| `internal/tui/overlays.go` | Project switcher, branch picker, help, confirm | Create (extract) |
| `internal/tui/styles.go`, `keys.go`, `messages.go` | Styles, keys, messages | Keep/extend |
| `internal/gitlab/commits.go` | `ListCommits` + `Commit` type | Create |
| `internal/app/config.go` | `views` + `default_view` settings | Modify |
| `internal/app/app.go` | Build views + wiring into shell | Modify |

The old `panels.go` / panel-cycling logic is removed; `PanelID` is replaced by
the view registry.

## Auto-refresh

The shell holds the tick (interval from config). On each tick, if no overlay is
open, it calls the active view's refresh (each view exposes a refresh via its own
`Focus()` or a dedicated method). Reading a job log pauses refresh (as in v1).
Cursor/selection state is preserved by each view across refreshes.

## Testing

- `internal/gitlab/commits_test.go`: `ListCommits` mapping (httptest, mirroring
  existing gitlab tests).
- `internal/tui/view_test.go`: view registry parsing (`views`, `default_view`,
  unknown/duplicate handling, defaults, obsolete `panels` ignored).
- `internal/app/config_test.go`: new settings fields parse and normalize.
- Pure helpers in `components/` keep their existing tests (moved with them).
- View logic that is pure (row formatting, sha→pipeline status mapping, overview
  aggregation) gets unit tests; rendering verified via the tmux harness.

## Rollout (incremental, each step builds & runs)

1. Extract shared helpers into `components/` (no behaviour change).
2. Introduce `View` interface, `Context`, shell scaffolding, frame (context bar
   + tabs + footer); wire a single placeholder view.
3. Port Pipelines into a view (deepest; validates the interface).
4. Port MRs and Issues.
5. Add Commits view + `ListCommits`.
6. Add Overview dashboard.
7. Replace the Projects panel with the project-switcher overlay; wire `default_view`.
8. Config migration + docs.

Each step keeps `go build`/tests green and is drivable via tmux.

## Compatibility

v1 configs still launch: `panels` is ignored (warning), everything else defaults.
Actions, setup, glab import, auto-detect, and the job-log viewer are preserved.
