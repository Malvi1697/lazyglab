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

## Updating

lazyglab checks for a newer release once per launch, in the background, and says
so on the tabs row when there is one (`▲ v0.5.0 available · lazyglab update`).
Nothing is downloaded until you ask:

```bash
lazyglab update
```

It downloads the build for your platform, verifies its SHA-256 against the
release's `checksums.txt`, and replaces the running binary in place. A copy
installed by a package manager (Homebrew, `.deb`/`.rpm`, Nix) is left alone —
the command tells you which upgrade to run instead. If the install directory
needs root, use `sudo lazyglab update`.

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
keys (`1`-`6`), `H`/`L` or `[`/`]`. `Tab` stays inside the active view, cycling its
own boxes. The context bar shows the active project
and branch, plus the latest status/error message.

The five views:

| # | View | What it shows |
|---|------|----------------|
| 1 | **Dashboard** | The project's front page: recent commits with their CI status. `t` brings the project's README up below them, `Tab` moves between the two. Default view. |
| 2 | **Pipelines** | Pipeline list; `Enter` drills into jobs grouped by stage, and into a job's log. |
| 3 | **Merge Requests** | Open MRs; `Enter` opens the full merge-request page in place, with approve/merge. |
| 4 | **Issues** | Open issues, with close/reopen; `Enter` opens the issue page and its discussion. |
| 5 | **Todos** | Your GitLab To-Do list, across every project: reviews asked of you, mentions, assignments, failed pipelines. `d` marks one done, `D` clears the list. |

**Todos** is the only view that ignores the selected project — it answers "what is
waiting on me" rather than "what is happening here", which is the question you have
before you have picked a project. Set `default_view: todos` to land there.

### One list, laid out the same way everywhere

Every list is the same table, because every row is the same kind of thing: what it
is called, what kind of change it is, how CI went, what it says, then who, where,
and when.

```
  !16 feat: · split registration into two tables    jiri.kucera    registered_w…  1.6. 13:04
  !12       · Test mr                               pavel.zehnula  test-MR        1.6. 13:04
   !9  wip: · deprecate User.club in favour of …    michal.dolezel user-club     27.3. 16:47
```

The numbers and the prefixes are right-aligned, so they line up and every subject
starts in the same column; the metadata stays grey and the message keeps the
default weight. The columns are measured over the whole list, so scrolling never
shifts the text sideways, and the ones at the right drop off on a narrow terminal
in the order you would give them up — the message is what matters.

The lists take the **whole width**. There is no preview panel restating the
highlighted row: `Enter` opens the real thing (the jobs, the merge-request page,
the issue and its discussion), and half the width spent on a summary was coming
out of the titles.

**`t` toggles the second box** where there is one. On the dashboard the README
**starts folded away** — the commits are what the page is opened for, and half the
rows spent on prose you have read once is half the commits you cannot see; `t`
brings it up and `Tab` then moves between the two. Under a to-do the reason starts
open, since it is about the highlighted row. The footer always says which way the
key goes.

The README is rendered as terminal text rather than converted: headings take the
accent, list bullets and quotes their marker, fenced code goes grey. It is fetched
once per project and branch — it is the one thing on the page that does not change
while you watch.

**What refreshes:** whatever is on screen. `r` and the thirty-second tick refresh
the *active* view, and within it the thing you are looking at — the open commit,
merge request or issue page, or the jobs of the pipeline you drilled into, rather
than the list behind it. Anything long-form being read (a diff, a job log, a
discussion) is left alone, because a refetch would move what you are halfway
through. Switching tabs reuses data younger than ten seconds; `r` always fetches.

Every list takes `/` to **search** it: type to narrow, `Enter` to keep the list
narrowed while the action keys work again, `Esc` to clear.

Two keys copy, everywhere: **`y` copies what you would type** — a SHA, `!42`, `#7`,
`#1234`, a job's name — and **`Y` copies the link you would send**. They act on the
box that has focus, so in the commit page's jobs box they copy the job while
everywhere else on that page they copy the commit.

The far right of the context bar says when the data was last fetched — a spinner
while a fetch is in flight, then how stale it is and how long until the automatic
refresh. `r` refreshes now.

A sixth view, **Commits** (a full-height commit list), exists but is not a default
tab: Overview already lists recent commits and `Enter` opens the commit page in
place. Add `commits` to `settings.views` to get the tab back.

`Enter` on a commit opens the **commit page in place** — no tab switching — with
the same things GitLab's commit page shows: the message, the author, the parent,
the branches and tags containing it, the merge requests it belongs to, and the
pipelines it triggered together with their jobs grouped by stage. `j`/`k` scroll it,
`Esc` goes back to the list, `R` retries the pipeline, `p` runs one on the branch
head and `y` copies the SHA.

The page lists the commit's **changed files** with their `+`/`-` counts. `Enter`
steps into them, `Enter` again reads that file's unified diff full-screen, and
`Esc` comes back. The diff is **syntax highlighted** by the file's language, with
the `+`/`-` gutter carrying whether a line was added or removed, so a diff reads
like the file it came from. `←`/`→` (or `h`/`l`) step between the commit's files,
and the title says which of how many you are on. A diff GitLab declines to send
(too large) says so rather than looking like an empty change.

`Enter` on a merge request opens the **merge-request page in place**, built from
the same boxes as the commit page: where it is going (`source → develop`), who is
reviewing it, its pipeline, **whether it can be merged** in GitLab's own words,
how many approvals it still needs and who has given theirs — then its description,
its changed files with diffs, and its pipeline's jobs. `a` approves, `m` merges
(both say what stands in the way rather than sending a request that cannot
succeed), `R` retries the pipeline, and `←`/`→` (or `h`/`l`) step between merge
requests without leaving the page.

### Discussions

The merge-request and issue pages carry the **conversation**: `Tab` reaches the
discussion box, `Enter` reads the whole thread as prose (author, when, the file
and line a review comment is on, whether it was resolved), and **`c` writes a
comment in `$EDITOR`** — the same editor git gives you, because a comment is prose
and `#` lines are stripped exactly as in a commit message. An empty comment is not
posted.

GitLab's own record ("added 3 commits", "changed the description") is hidden by
default — an issue can carry a hundred of those and no conversation at all — and
the heading says how many are being left out. `s` shows them.

`Tab` cycles the page's boxes — message, changes, jobs — skipping any that are
empty. The jobs are already listed on the page, so `Enter` **steps the focus into them**
rather than replacing the page: a cursor appears on a job while the message and
branches stay above it, and the page scrolls down to bring the jobs into view.
From there `Enter` reads that job's log — the one thing that does take the whole
body, because a log needs the room — `R` retries a job, `C` cancels one and `p`
plays a manual one. `Esc` unwinds log → jobs → page → list.

`←`/`→` (or `h`/`l`) step to the previous and next commit without leaving the page.

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
`H`/`L` and `[`/`]` step between views.

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

lazyglab stores its config where the OS keeps user config — on Linux
`~/.config/lazyglab/config.yml`, on macOS
`~/Library/Application Support/lazyglab/config.yml` — and the setup wizard prints
the path it used. Override it with the `LAZYGLAB_CONFIG` environment variable.
The file holds a token, so it must be mode 0600; lazyglab refuses to read it
otherwise. Besides the host/token entries written
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
  # Enabled tabs, in display order. Omit any to hide it.
  # Empty/omitted = dashboard, pipelines, mrs, issues, todos.
  views: [dashboard, pipelines, mrs, issues, todos, commits]
  # The view shown at launch. Empty/omitted = first enabled view.
  default_view: dashboard
  # Auto-refresh interval for the active view, in seconds. 0 disables it.
  refresh_interval: 30
```

- **views** — reorder or hide tabs. For example `[overview, pipelines]` shows
  only those two. Valid names: `dashboard` (`overview` still works), `pipelines`,
  `mrs`, `issues`, `todos`, `commits`. Unknown or duplicate names are dropped with a warning.
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
| `Tab` / `Shift+Tab` | Cycle the boxes inside the active view |
| `H` / `L` | Previous / next view |
| `[` / `]` | Previous / next view |
| `h` / `l`, `←` / `→` | Move *within* what is open (between commits, between a commit's files) |
| `t` | Fold the second box away, where the view has one |
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

### Dashboard

| Key | Action |
|-----|--------|
| `Enter` | Open the commit page in place |
| `t` | Show / fold the README (folded when the view opens) |
| `Tab` | Move between the commits and the README, once it is up |
| `o` | Open the commit in a browser |

### Todos

| Key | Action |
|-----|--------|
| `Enter` / `o` | Open it on GitLab |
| `d` / `D` | Mark the highlighted to-do / the whole list done |
| `t` | Fold the detail below the list away |

## License

[MIT](LICENSE)
