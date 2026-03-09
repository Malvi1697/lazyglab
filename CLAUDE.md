# Lazyglab — Terminal UI for GitLab

A lazygit-inspired TUI for GitLab, wrapping the GitLab API with a keyboard-driven interface.

## Tech Stack

- **Language:** Go 1.26+
- **TUI Framework:** Bubble Tea v2 (charm.land/bubbletea/v2)
- **Styling:** Lipgloss v2 (charm.land/lipgloss/v2)
- **Components:** Bubbles v2 (charm.land/bubbles/v2) — not yet used
- **GitLab API:** gitlab.com/gitlab-org/api/client-go v1.46.0
- **Config parsing:** gopkg.in/yaml.v3 (reads glab CLI config)

## Architecture

### Project Structure

```
lazyglab/
├── main.go                      # Entry point, CLI args, app bootstrap
├── internal/
│   ├── app/
│   │   ├── app.go               # Application lifecycle, initialization
│   │   └── config.go            # Config loading (reads glab config + own config)
│   ├── gitlab/                  # GitLab API abstraction layer
│   │   ├── client.go            # Client wrapper, auth, host management
│   │   ├── mergerequests.go     # MR API operations
│   │   ├── pipelines.go         # Pipeline/job API operations
│   │   ├── issues.go            # Issue API operations
│   │   ├── projects.go          # Project listing/selection
│   │   └── types.go             # Domain models (decoupled from API structs)
│   ├── tui/                     # All Bubble Tea UI code
│   │   ├── app.go               # Root model, message router, layout compositor
│   │   ├── keys.go              # Keybinding definitions
│   │   ├── styles.go            # Lipgloss styles (colors, borders)
│   │   ├── messages.go          # Custom message types
│   │   ├── layout.go            # Panel layout computation
│   │   ├── components/          # Reusable UI building blocks
│   │   │   ├── listpanel.go     # Generic scrollable list panel
│   │   │   ├── detailpanel.go   # Right-side detail viewport
│   │   │   ├── statusbar.go     # Bottom status bar
│   │   │   ├── searchbar.go     # Filter/search input overlay
│   │   │   ├── confirmation.go  # Yes/No confirmation dialog
│   │   │   └── helpoverlay.go   # Help popup (? key)
│   │   ├── panels/              # Domain-specific panel models
│   │   │   ├── projects.go      # Project selector
│   │   │   ├── mergerequests.go # MR list + detail
│   │   │   ├── pipelines.go     # Pipeline list + stages/jobs
│   │   │   └── issues.go        # Issue list + detail
│   │   └── context/             # Navigation context management
│   │       ├── manager.go       # Context stack (push/pop, focus)
│   │       └── context.go       # Context interface
│   └── util/                    # Shared utilities
│       ├── truncate.go          # String truncation
│       ├── timeago.go           # Relative time formatting
│       └── color.go             # Status-to-color mapping
├── config/
│   └── default.go               # Default configuration values
└── testdata/                    # Mock API responses for tests
```

### Key Design Decisions

1. **Direct GitLab API** — Uses `client-go` SDK, not `glab` CLI subprocess calls. Auth tokens read from glab's config (`~/.config/glab-cli/config.yml` or `~/Library/Application Support/glab-cli/config.yml`). Hosts with empty tokens are skipped.

2. **Root Model as Message Router** — `tui/app.go` owns all panel sub-models and dispatches messages. Global keys handled at root, panel-specific keys forwarded to active panel.

3. **Context Stack Navigation** — LIFO stack: `Enter` pushes detail context, `Esc` pops back. `Tab`/number keys switch side panels.

4. **Async Data Loading** — All API calls via `tea.Cmd` (non-blocking). Panels show spinner while loading. `tea.Batch()` for parallel loads.

5. **Domain Type Isolation** — TUI layer never sees `client-go` API structs directly. `internal/gitlab/` translates to domain types in `types.go`.

### Panel Layout

```
+------------------+--------------------------------------+
| [1] Projects     |                                      |
|   project-a      |        Main Content Panel            |
|   project-b  *   |                                      |
|------------------|   (MR details, pipeline stages,      |
| [2] MRs          |    issue body, diff view, etc.)      |
|   !123 Fix auth  |                                      |
|   !124 Add tests |                                      |
|------------------|                                      |
| [3] Pipelines    |                                      |
|   #456 passed    |                                      |
|   #457 running   |                                      |
|------------------|                                      |
| [4] Issues       |                                      |
|   #89 Bug in..   |                                      |
|   #90 Feature..  |                                      |
+------------------+--------------------------------------+
| Status bar: host | project | branch                     |
+------------------------------------------------------------+
```

### Keybindings

**Global:** `q` quit, `?` help, `1-4` panel switch, `Tab`/`S-Tab` next/prev panel, `/` search, `r` refresh
**Lists:** `j`/`k` up/down, `g`/`G` top/bottom, `Enter` detail view, `Esc` back
**MRs:** `a` approve, `m` merge, `c` comment, `o` open in browser
**Pipelines:** `R` retry, `C` cancel
**Issues:** `c` close/reopen, `o` open in browser

## Build & Run

```bash
go build -o lazyglab .    # Build
go run .                  # Run directly
make build                # Build via Makefile
make run                  # Run via Makefile
```

## Testing

```bash
go test ./...             # Run all tests
```

## Configuration

Lazyglab reads auth from glab's config file. No additional configuration needed if glab is already set up. Own config at `~/.config/lazyglab/config.yml` (future).
