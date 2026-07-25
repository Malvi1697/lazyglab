# lazyglab

A terminal UI for GitLab, inspired by [lazygit](https://github.com/jesseduffield/lazygit).

Manage merge requests, pipelines, and issues without leaving your terminal.

## Features

- A cockpit layout: a context bar (active project/branch) and tabs on top,
  one full-screen view below, switched with number keys or Tab
- Overview dashboard summarizing recent commits, pipelines, MRs, and issues
  at a glance
- Browse and switch between GitLab projects
- View, approve, and merge MRs
- Monitor pipelines, view jobs grouped by stage, and read job logs
- Run, retry, and cancel pipelines and individual jobs
- Play manual jobs directly from the TUI
- Filter pipelines by branch
- Browse and manage issues (close/reopen)
- Browse recent commits, with CI status mapped in from pipelines
- Vim-style keyboard navigation (j/k/h/l/g/G/Ctrl+d/Ctrl+u)
- Context-sensitive keybinding hints at the bottom
- Interactive first-run setup wizard — no external tools required
- Can import existing `glab` CLI config automatically

## Install

### Quick install (Linux / macOS)

```bash
curl -sL https://raw.githubusercontent.com/Malvi1697/lazyglab/master/install.sh | sh
```

### Go install

```bash
go install github.com/Malvi1697/lazyglab@latest
```

### Download binary

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/Malvi1697/lazyglab/releases) page.

### Debian / Ubuntu (.deb)

```bash
# Download from the latest release
sudo dpkg -i lazyglab_*_linux_amd64.deb
```

### RHEL / Fedora (.rpm)

```bash
# Download from the latest release
sudo rpm -i lazyglab_*_linux_amd64.rpm
```

### Build from source

```bash
git clone https://github.com/Malvi1697/lazyglab.git
cd lazyglab
make build
```

## First Run

On first launch, lazyglab will:

1. Check for an existing `glab` CLI config and offer to import it
2. If no glab config is found, run an interactive setup wizard:

```
$ lazyglab

  No config found. Let's set up lazyglab.

  GitLab host [gitlab.com]:
  Personal access token: ****

  Testing connection... OK (logged in as @you)
  Config saved to ~/.config/lazyglab/config.yml
```

You'll need a [Personal Access Token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html) with `api` scope (or `read_api` for read-only).

To reconfigure later: `lazyglab setup`

## Usage

```bash
lazyglab            # Launch the TUI
lazyglab setup      # Re-run setup wizard
lazyglab --version  # Show version
```

## The cockpit

lazyglab is a cockpit: a context bar and a row of tabs stay on screen, and the
space below shows one full-screen view at a time. Switch views with the number
keys (`1`-`5`) or `Tab`/`Shift+Tab`. The context bar shows the active project
and branch, plus the latest status/error message.

The five views:

| # | View | What it shows |
|---|------|----------------|
| 1 | **Overview** | Dashboard: recent commits (with CI status) plus summaries of pipelines, merge requests, and issues. Read-only, default view. |
| 2 | **Pipelines** | Pipeline list; `Enter` drills into jobs grouped by stage, and into a job's log. |
| 3 | **Merge Requests** | Open MRs, with approve/merge actions. |
| 4 | **Issues** | Open issues, with close/reopen. |
| 5 | **Commits** | Recent commits on the active branch. |

Press `p` to switch projects and `b` to switch branches from any view — both
open as overlays on top of the current view. `r` refreshes the active view.

## Configuration

lazyglab stores its config at `~/.config/lazyglab/config.yml` (override with the
`LAZYGLAB_CONFIG` environment variable). Besides the host/token entries written
by the setup wizard, an optional `settings:` block tunes the UI:

```yaml
default_host: gitlab.com
hosts:
  gitlab.com:
    token: "…"
settings:
  # Enabled tabs, in display order. Omit any to hide it. Empty/omitted = all five.
  views: [overview, pipelines, mrs, issues, commits]
  # The view shown at launch. Empty/omitted = first enabled view.
  default_view: overview
  # Auto-refresh interval for the active view, in seconds. 0 disables it.
  refresh_interval: 30
```

- **views** — reorder or hide tabs. For example `[overview, pipelines]` shows
  only those two. Valid names: `overview`, `pipelines`, `mrs`, `issues`,
  `commits`. Unknown or duplicate names are dropped with a warning.
- **default_view** — which view is active on launch, by name. Falls back to
  the first enabled view if empty, unknown, or not in `views`.
- **refresh_interval** — the active view reloads on this interval (paused while
  an overlay — help/project/branch/confirm — is open). Values 1–4 are clamped
  to 5s; `0` disables auto-refresh. Omitted → 30s.

With no `settings:` block, all five views are enabled in the order above,
Overview is shown first, and auto-refresh runs every 30s.

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `q` | Quit |
| `?` | Help overlay |
| `1-5` | Switch view |
| `Tab` / `Shift+Tab` | Next/previous view |
| `p` | Project switcher |
| `b` | Branch switcher |
| `r` | Refresh active view |
| `j/k` | Navigate down/up |
| `g/G` | Go to top/bottom |
| `Ctrl+d/u` | Half page down/up |
| `Esc` | Close overlay / go back |

### Pipelines

| Key | Action |
|-----|--------|
| `Enter` | View jobs (then a job's log) |
| `R` | Retry pipeline / job |
| `C` | Cancel pipeline / job |
| `p` | Run new pipeline / play manual job |
| `o` | Open in browser |
| `Esc` | Back (job log → jobs → pipeline list) |

### Merge Requests

| Key | Action |
|-----|--------|
| `a` | Approve MR |
| `m` | Merge MR |
| `o` | Open in browser |

### Issues

| Key | Action |
|-----|--------|
| `c` | Close/reopen issue |
| `o` | Open in browser |

### Commits

| Key | Action |
|-----|--------|
| `o` | Open commit in browser |

Overview is read-only and has no view-specific keys.

## License

[MIT](LICENSE)
