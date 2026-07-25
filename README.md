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

To reconfigure later: `lazyglab setup`, or press `A` inside the app.

### When a token stops working

If GitLab rejects the stored token — expired, revoked, or missing the `api`
scope — lazyglab opens a **Reconnect** overlay instead of leaving a red error in
the status bar. The host is prefilled and editable, with the token field below
it:

```
╭─Reconnect to GitLab──────────────────────────────────────╮
│ Authentication failed                                    │
│ invalid_token, Token was revoked.                        │
│                                                          │
│ GitLab host                                              │
│   gitlab.example.com                                     │
│                                                          │
│ Personal access token (scope: api)                        │
│ › ••••••••••••                                           │
│                                                          │
│ Create one at https://gitlab.example.com/-/user_setti...  │
│                                                          │
│ Tab: next field  Enter: save  Esc: cancel                │
╰──────────────────────────────────────────────────────────╯
```

The new token is validated before anything is written; on success it is saved to
the config (other hosts and all settings are left untouched) and the app reloads
with it, no restart needed. `Esc` dismisses the overlay — it will not pop up
again on its own until you press `A`.

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
| 1 | **Overview** | Dashboard: a navigable recent-commits list (with CI status) plus summaries of pipelines, merge requests, and issues. Default view. |
| 2 | **Pipelines** | Pipeline list; `Enter` drills into jobs grouped by stage, and into a job's log. |
| 3 | **Merge Requests** | Open MRs, with approve/merge actions. |
| 4 | **Issues** | Open issues, with close/reopen. |

A fifth view, **Commits** (a full-height commit list), exists but is not a default
tab: Overview already lists recent commits and `Enter` opens the commit page in
place. Add `commits` to `settings.views` to get the tab back.

`Enter` on a commit opens the **commit page in place** — no tab switching — with
the same things GitLab's commit page shows: the message, the author, the parent,
the branches and tags containing it, the merge requests it belongs to, and the
pipelines it triggered together with their jobs grouped by stage. `j`/`k` scroll it,
`Esc` goes back to the list, `R` retries the pipeline, `p` runs one on the branch
head and `y` copies the SHA.

The jobs are already listed on the page, so `Enter` **steps the focus into them**
rather than replacing the page: a cursor appears on a job while the message and
branches stay above it, and the page scrolls down to bring the jobs into view.
From there `Enter` reads that job's log — the one thing that does take the whole
body, because a log needs the room — `R` retries a job, `C` cancels one and `p`
plays a manual one. `Esc` unwinds log → jobs → page → list.

`←`/`→` (or `H`/`L`) step to the previous and next commit without leaving the page.

A commit that never triggered CI simply says so — the detail is still useful, and
nothing jumps you elsewhere.

Pipelines that **passed with warnings** (a success whose allowed-to-fail job
failed) show an orange `!` rather than a green check, matching what GitLab shows.
Only GitLab's single-pipeline endpoint reports this, so it costs one request per
successful pipeline the first time it is seen; the verdict is then cached for the
session, since a finished pipeline cannot change its mind.

Note that `p` in the detail runs a pipeline **on the branch head, not on the
commit**: GitLab creates pipelines for a ref (branch or tag), never for an
arbitrary past commit, so that is the closest thing that exists. The confirmation
prompt names the branch it will build.

Press `P` to switch projects and `b` to switch branches from any view — both
open as overlays on top of the current view. `r` refreshes the active view.
`h`/`l` step between views, like `Shift+Tab`/`Tab`.

Both the project and branch pickers support incremental search: press `/` and
type to narrow the list (matching the display name or the `group/project` path);
the title shows `matched/total`. While you type, characters are text rather than
commands — `f` is a letter, not "star" — so `Enter` **applies** the search: the
list stays narrowed and every normal key works again, letting you search for a
project and then star it. `Enter` again opens the highlighted entry, `/` resumes
editing the query, `Esc` clears it, and a second `Esc` closes the picker. Arrows
and `Ctrl+d`/`Ctrl+u` navigate throughout.

### Favorites

Star the projects you work on daily so they are one keystroke away instead of
somewhere in a list of hundreds:

- In the project switcher (`P`), highlight a project and press `f`. Starred
  projects are marked `★`, sorted to the top of the list and separated from the
  rest by a divider.
- Press `f` from any view to open the **Favorites** picker, a short list of just
  the starred projects: `Enter` opens one, `f` unstars it.

Favorites are stored per host in the config (`hosts.<host>.favorites`) as
`group/project` paths, so they survive restarts and can be edited by hand. A
favorite is fetched directly by path when it is not among the loaded projects, so
it stays reachable regardless.

### Resuming where you left off

The project you were last on is remembered per host (`hosts.<host>.last_project`)
and reopened on the next launch. Running lazyglab inside a git repository whose
remote points at a configured GitLab host overrides it — being in that repo is the
clearer signal about what you want to look at.

## Colors

lazyglab draws with the terminal's own 16 colors, referenced by index rather than
as fixed values: "green" is whatever green your theme calls green, and body text
sets no color at all, so it inherits your foreground. Change your terminal theme
and lazyglab follows it — nothing here overrides your palette.

## Configuration

lazyglab stores its config at `~/.config/lazyglab/config.yml` (override with the
`LAZYGLAB_CONFIG` environment variable). Besides the host/token entries written
by the setup wizard, an optional `settings:` block tunes the UI:

```yaml
default_host: gitlab.com
hosts:
  gitlab.com:
    token: "…"
    # Starred projects for this host, shown in the favorites picker (f).
    # Managed from the UI, but hand-editable.
    favorites:
      - my-group/my-project
      - other-group/service
    # Reopened on the next launch. Written when you switch projects.
    last_project: my-group/my-project
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
| `h` / `l` | Previous / next view |
| `[` / `]` | Previous / next view |
| `P` | Project switcher (`f` stars the highlighted project) |
| `f` | Favorites picker (`f` again unstars) |
| `/` | Search inside the project / branch picker |
| `b` | Branch switcher |
| `A` | Reconnect (change host / replace token) |
| `r` | Refresh active view |
| `j` / `k`, `↓` / `↑` | Down / up |
| `.` / `,` | Page down / up |
| `Ctrl+d` / `Ctrl+u` | Half page down / up |
| `>` / `<`, `G` / `g`, `End` / `Home` | Jump to bottom / top |
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
