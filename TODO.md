# Lazyglab — TODO

## Phase 1: MVP

### Foundation
- [x] Project scaffold (directory structure, go.mod)
- [x] Documentation (CLAUDE.md, README.md, TODO.md)
- [x] Makefile (build, run, test, install targets)
- [x] main.go entry point

### Config & Auth (Step 2)
- [x] Read glab config file (YAML parsing)
- [x] Support macOS path (`~/Library/Application Support/glab-cli/config.yml`)
- [x] Support Linux path (`~/.config/glab-cli/config.yml`)
- [x] Multi-host support (multiple GitLab instances)
- [x] Skip hosts with empty tokens
- [ ] Fallback: prompt for token if glab not configured

### GitLab Client (Step 3)
- [x] Client wrapper with authentication (`internal/gitlab/client.go`)
- [x] Domain types decoupled from API structs (`internal/gitlab/types.go`)
- [x] Project listing (`internal/gitlab/projects.go`)
- [x] MR operations: list, get detail, approve, merge (`internal/gitlab/mergerequests.go`)
- [x] Pipeline operations: list, get jobs, retry, cancel (`internal/gitlab/pipelines.go`)
- [x] Issue operations: list, get detail, close/reopen (`internal/gitlab/issues.go`)
- [x] Branch listing (`internal/gitlab/branches.go`)
- [x] Pipeline filtering by branch ref

### Core TUI (Step 4)
- [x] Root model with layout computation (`internal/tui/app.go`)
- [x] Lipgloss styles and color palette (`internal/tui/styles.go`)
- [x] Keybinding definitions (`internal/tui/keys.go`)
- [x] Custom message types (`internal/tui/messages.go`)
- [x] Panel switching (Tab, number keys)
- [x] Scrolling in sidebar panels
- [ ] Generic list panel component (`internal/tui/components/listpanel.go`)
- [ ] Detail viewport component (`internal/tui/components/detailpanel.go`)
- [ ] Status bar component (`internal/tui/components/statusbar.go`)

### Domain Panels (Steps 5-7)
- [x] Projects panel — list, select, set as active
- [x] MRs panel — list open MRs, view detail, approve, merge
- [x] Pipelines panel — list pipelines, view stages/jobs, retry, cancel
- [x] Pipeline jobs detail — grouped by stage, colored status, duration
- [x] Branch selector — `b` key opens picker, filters pipelines by branch
- [x] Issues panel — list issues, view detail, close/reopen

### Polish (Step 8)
- [x] Help overlay (`?` key)
- [ ] Search/filter overlay (`/` key)
- [ ] Confirmation dialogs for destructive actions (merge, close)
- [x] Error display in status bar
- [ ] Loading spinners during API calls

## Phase 2: Post-MVP

### Enhanced Features
- [ ] MR diff viewer (syntax-highlighted)
- [ ] Pipeline job log streaming (trace output)
- [ ] Create MR wizard
- [ ] Create Issue wizard
- [ ] MR review workflow (line-level comments)
- [ ] Notifications / Todos panel (5th panel)
- [ ] Auto-detect project from `.git/config` remote URL
- [ ] Auto-refresh on timer (configurable interval)

### Configuration
- [ ] Own config file (`~/.config/lazyglab/config.yml`)
- [ ] Custom keybinding overrides
- [ ] Theme/color customization
- [ ] Default filters (e.g., only show MRs assigned to me)

### Distribution
- [ ] goreleaser config
- [ ] Homebrew formula
- [ ] AUR package
- [ ] GitHub Actions CI/CD

## Known Challenges

- **Terminal size:** Panels must handle very small terminals gracefully
- **API rate limits:** Implement backoff, show rate limit status
- **Large lists:** Lazy loading with pagination (load more on scroll)
- **Real-time updates:** Poll pipeline status every 10-30s via `tea.Tick`
- **Layout:** Bubble Tea has no built-in grid layout — manual computation with Lipgloss `Join*` functions
