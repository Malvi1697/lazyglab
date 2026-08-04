# lazyglab

A terminal UI for GitLab, in the spirit of [lazygit](https://github.com/jesseduffield/lazygit):
merge requests, pipelines, issues and your to-do list without leaving the terminal.

```
  main  ACME / Website                                    Selected: ACME / Website  updated 4s ago
 [1] Dashboard ·  2 Pipelines ·  3 Merge Requests ·  4 Issues ·  5 Todos

── Recent Commits (50) ──────────────────────────────────────────────────────────────────────────────
▌     feat: ▲ paginate the search endpoint                              Zoë Müller     4.8. 15:28
       fix: · clamp the date window to the current week                  Ada Byron      3.8. 20:27
      test: ● fix the flaky timezone test in CI                         Zoë Müller     3.8. 17:06
     chore: · bump the client library                                   Ada Byron      2.8. 09:14

Commit page: Enter | Show readme: t | Copy SHA/link: y/Y | Search: / | View: 1-5 | Keybindings: ?
```


## Install

```bash
# Linux / macOS
curl -sL https://raw.githubusercontent.com/Malvi1697/lazyglab/master/install.sh | sh

# or with Go
go install github.com/Malvi1697/lazyglab@latest
```

Also on the [releases page](https://github.com/Malvi1697/lazyglab/releases): binaries
for Linux, macOS and Windows, plus `.deb` and `.rpm` packages. From source:
`git clone …  && make build`.

## Updating

lazyglab checks for a newer release once per launch, in the background, and says so
on the tabs row (`▲ v0.7.0 available · lazyglab update`). Nothing is downloaded until
you ask:

```bash
lazyglab update      # verifies the SHA-256 from the release's checksums.txt
```

A copy installed by a package manager is left alone; the command names the upgrade
to run instead. If the install directory needs root, use `sudo lazyglab update`.

## First run

With no config, lazyglab runs a short wizard (or offers to import an existing `glab`
CLI config). You need a [personal access token](https://docs.gitlab.com/ee/user/profile/personal_access_tokens.html)
with the `api` scope — `read_api` if you only want to look.

```
$ lazyglab
  No config found. Let's set up lazyglab.
  GitLab host [gitlab.com]:
  Personal access token: ****
  Testing connection... OK (logged in as @you)
```

Reconfigure later with `lazyglab setup`, or press `A` in the app: if GitLab ever
rejects the stored token, lazyglab opens that form itself rather than leaving a red
error in the status bar, validates the new token before writing it, and reloads
without a restart.

## Usage

```bash
lazyglab            # open the cockpit
lazyglab setup      # run the wizard again
lazyglab update     # install the newest release
lazyglab --version
```

## The cockpit

A context bar and a row of tabs stay on screen; below them is one full-screen view
at a time. Switch views with the number keys, `H`/`L` or `[`/`]`.

| # | View | What it shows |
|---|------|----------------|
| 1 | **Dashboard** | Recent commits with their CI status. `t` brings the README up below them; `Enter` opens the commit page. |
| 2 | **Pipelines** | One mark per stage, so you can see where each pipeline got to. `t` names the stages, `Enter` opens the jobs and their logs. |
| 3 | **Merge Requests** | Open MRs; `Enter` opens the page with approve/merge, the changed files and the discussion. |
| 4 | **Issues** | Open issues, with close/reopen; `Enter` opens the issue and its discussion. |
| 5 | **Todos** | Your GitLab to-do list across every project — the one view that ignores the selected project. |

A sixth view, **Commits** (a full-height commit list), is available by adding
`commits` to `settings.views`.

Four things work the same way everywhere:

- **Every list is the same table** — what it is called, what kind of change it is,
  how CI went, what it says, then who, where and when. Lists take the whole width;
  what a row cannot say is behind `Enter`.
- **`/` searches the list you are in.** Typing narrows it, `Enter` keeps it narrowed
  and hands the keys back, `Esc` clears.
- **`y` copies what you would type** (a SHA, `!42`, `#7`, a job name) and **`Y` the
  link you would send**. In the project picker they copy the SSH and HTTPS clone
  URLs.
- **`r` refreshes what is on screen**, and so does the thirty-second tick — the open
  page rather than the list behind it, and nothing at all while you are reading a
  diff or a log. It pauses when the terminal is in the background.

Pipelines and jobs are actionable from either place they appear: `R` retries, `C`
cancels, `p` runs a new pipeline (on the branch head — GitLab has no pipeline for an
arbitrary past commit) or plays a manual job. Discussions take `c` to write a comment
in `$EDITOR` and `s` to show GitLab's own bookkeeping notes, which are hidden by
default.

Star projects with `f` in the project picker (`P`) to get them at the top of the list
and in the `f` picker. The project you were last in is reopened on the next launch,
unless you are inside a git repository whose remote points at a configured host — in
that case that project wins.

Colours come from the terminal's own 16, by index: "green" is whatever green your
theme calls green, and body text sets no colour at all.

## Keybindings

`?` shows this list in the app.

| Key | Action |
|-----|--------|
| `q`, `Ctrl+c` | Quit |
| `?` | Help |
| `1`–`9`, `H`/`L`, `[`/`]` | Switch view |
| `h`/`l`, `←`/`→` | Move within what is open (between commits, between a commit's files) |
| `Tab` / `Shift+Tab` | Cycle the boxes of the active view |
| `j`/`k`, `.`/`,`, `Ctrl+d`/`Ctrl+u`, `g`/`G` | Move or scroll |
| `Enter` | Open / drill in |
| `Esc` | Back, one level at a time |
| `/` | Search the list |
| `t` | Fold the view's second box |
| `y` / `Y` | Copy the identifier / the link |
| `o` | Open in a browser |
| `r` | Refresh |
| `P` / `b` / `f` | Project picker / branch filter / favorites |
| `A` | Reconnect (change host or token) |
| `R` / `C` / `p` | Retry / cancel / run — pipelines and jobs |
| `a` / `m` | Approve / merge a merge request |
| `c` | Close an issue, or write a comment on a page with a discussion |
| `d` / `D` | Mark one to-do / all to-dos done |
| `s` | Show or hide GitLab's own notes in a discussion |

## Configuration

The config lives where the OS keeps user config — `~/.config/lazyglab/config.yml` on
Linux, `~/Library/Application Support/lazyglab/config.yml` on macOS — and the wizard
prints the path it used. `LAZYGLAB_CONFIG` overrides it. It holds a token, so it must
be mode 0600; lazyglab refuses to read it otherwise.

```yaml
default_host: gitlab.com
hosts:
  gitlab.com:
    token: "…"
    favorites: [my-group/my-project]   # managed with f, hand-editable
    last_project: my-group/my-project  # reopened next launch
settings:
  views: [dashboard, pipelines, mrs, issues, todos]  # tabs, in order
  default_view: dashboard
  refresh_interval: 30                              # seconds; 0 disables
```

- **views** — which tabs exist and in what order. Names: `dashboard`, `pipelines`,
  `mrs`, `issues`, `todos`, `commits`. Unknown or duplicate names are dropped with a
  warning.
- **default_view** — the view at launch; falls back to the first enabled one.
- **refresh_interval** — 1–4 are clamped to 5s, `0` disables, omitted means 30s.

## Development

```bash
make build     # or: go build -o lazyglab .
make test
make check     # vet + golangci-lint + tests, what the pre-commit hook runs
make install   # into ~/go/bin
```

[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) is the map of the code: what each
package owns, the shared list layout, the measured request budget and the rules that
keep it there, and the GitLab API quirks each of which cost a bug.

## License

[MIT](LICENSE)
