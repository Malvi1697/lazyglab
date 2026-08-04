# Architecture

A map of the code and the handful of constraints that shaped it. Read this before
changing behaviour; the code says *what*, this says *why*.

## Layout

```
main.go                    flags and subcommands only
internal/app               config, auth, the setup wizard, self-update
internal/tui               the shell: global keys, overlays, refresh, status bar
internal/tui/views         one file per view, plus the pieces they share
internal/tui/components    pure UI helpers: panels, rows, navigation, palette
internal/gitlab            API wrapper and domain types, one file per topic
internal/util              time formatting, ANSI stripping, git remote parsing
```

**`tui` must not import `app`** — it would cycle. Anything the shell needs written
to disk arrives as a function in `tui.Options` (`ReconfigureFunc`,
`SaveFavoritesFunc`, `SaveLastProjectFunc`, `CheckUpdateFunc`). For the same reason
the key strings the views need are duplicated as constants in `views/pipelines.go`;
keep the two lists in step.

Bubble Tea v2 and Lipgloss v2 live under `charm.land/...`, not
`github.com/charmbracelet/...`. `View()` returns `tea.View`; `AltScreen` and
`ReportFocus` are fields on it.

## The shared pieces

| Type | File | What it owns |
|---|---|---|
| `rowList[T]` | `views/rowlist.go` | every list's rows, cursor, scroll offset, search, and its rendering |
| `listRow` + `measureColumns` + `renderListRow` | `views/view.go` | the one row layout every list uses |
| `foldBox` + `splitBody` | `views/foldbox.go` | the second box a view can fold away with `t` |
| `jobsPanel` | `views/jobspanel.go` | a pipeline's jobs, wherever they are shown |
| `commitDetail`, `mrDetail`, `issueDetail` | `views/*detail.go` | the pages `Enter` opens |

A new view embeds `rowList[T]`, sets `match` in its constructor, and implements
`Focus`/`Update`/`handleKey`/`Body`/`KeyHints`. Register it in `views.ViewID`,
`viewIDFromName`, `defaultViews`, `NewApp`'s switch, the README table and the help
overlay.

### One row layout

```
 !42   feat: ● capacity-aware promotion    ● ● ● ▶   jiri.kucera   promo-cap   28.7. 20:28
        fix: ▲ count only free wild cards            Jan Všetíček              27.7. 15:35
```

`ref`, `kind`, CI mark, `subject`, `marks`, `author`, `extra`, `stamp`. A list fills
the fields it has; an empty field costs no column. The rules, and why:

- Columns are measured over the **whole list**, not the visible window, or scrolling
  shifts the text sideways.
- The left-hand columns are right-aligned, so numbers and colons line up and every
  subject starts in the same place. The subject takes what is left, which puts the
  right-hand columns against the right edge.
- The right-hand columns drop off widest-last when the row is narrow. The message is
  what matters; a branch cut to eight characters says nothing.
- `marks` arrives pre-styled (a pipeline's stages, in the status colours), so it is
  measured in display columns and never truncated or repainted.
- **A list takes the whole width.** No preview panel beside it restating the
  highlighted row — that width comes out of the titles. What the row cannot say
  belongs behind `Enter`; a view with nothing behind `Enter` (Todos) gets a box
  *below*, folded with `t`.

## The request budget

A cockpit that refreshes every thirty seconds is only welcome on someone else's
GitLab if a refresh is cheap. Both budgets are measured, not estimated:

```bash
go test ./internal/tui/views/ -run TestAPICost -v      # requests per user action
go test ./internal/tui/views/ -run XXX -bench . -benchmem   # cost of one frame
```

Where it stands (three-pipeline fixture):

| Action | Requests |
|---|---|
| Any single list (MRs, issues, todos) | 1 |
| Dashboard refresh | 3 (commits, pipelines, README) |
| Pipelines refresh | 1, plus 1 per 20 rows for the stage marks — 0 once cached |
| Commit page, opened | 6 |
| Commit page, stepped back to | 0 |
| Tab switch within 10s | 0 |
| Idle, terminal unfocused | 0 |
| Update check | 1 to GitHub, once per launch |

The rules that keep it there:

- **Never fetch per row.** A list endpoint missing a field is one bulk lookup, not
  one request per row (`fillCommitTitles` resolves a page of SHAs in one call).
- **When REST has no bulk endpoint, GraphQL is the bulk endpoint.** Pipeline stages
  are the only such case (`gitlab/graphql.go`); REST plus client-go is the contract
  everywhere else.
- **Cache what cannot change, forever.** A commit's title, a finished pipeline's
  verdict and its stages are immutable, so they cost one request per process
  (`titleCache`, `warningCache`, `stageCache`).
- **Cache rendered text per (content, width).** A diff, a job log and the job rows
  are derived once and reused until their source changes. Deriving a
  four-thousand-line log per frame cost 1.86 ms and 884 KB of garbage per keypress.
- **Only render what is on screen.** `renderRowsBox` builds rows on demand; styling
  a row is the most expensive thing a frame does.
- **A held key must not fan out.** Stepping between commits waits `stepSettleDelay`
  (120 ms) and drops the fetch if you moved on.
- **Poll nothing for a window nobody is watching.** `tea.View.ReportFocus` plus
  `tea.BlurMsg`/`FocusMsg` pause the refresh tick and the clock.
- **Startup asks for what it will open**: the project from the git remote or the
  previous run resolves by path (one request); the project list waits for the picker.
- Do not micro-optimise past this point. ~0.6 ms per frame is dominated by
  `lipgloss.Style.Render`; beating it costs colour-profile correctness.

## What the GitLab API taught us

Each of these is pinned by a test, because each of them cost a bug:

- **`CiStage.status` lies.** A stage whose every job was canceled reports `success`
  (its own `detailedStatus` says `status_warning`). Stage marks are derived from the
  jobs by `StageStatus`, so a row and its drill-down cannot disagree.
- **GraphQL refuses more than 20 pipelines per `ids:` filter**, so a page of thirty
  is two requests.
- **GraphQL answers HTTP 200 with an `errors` array**, so a failed query looks like
  success until you check.
- **The MR list endpoint carries no pipeline**; only `GetMergeRequest` does.
- **Job lists come back newest-first**, which is the reverse of stage order —
  `inStageOrder` fixes it so the third stage mark is the third group.
- Every string from the API goes through `util.StripANSI`: a project name is
  untrusted text that could otherwise paint the screen.
- IDs are `int64` in client-go and `int` in our types; cast at the boundary.

## Keys

- `H`/`L` (and `[`/`]`, and the numbers) switch views — the big tabs.
- `h`/`l` (and `←`/`→`) move **within** what is open.
- `Tab` cycles the boxes of the focused view; it never switches views.
- `j`/`k` and lazygit's `,`/`.`, `<`/`>`, `g`/`G`, `Ctrl+d`/`Ctrl+u`, Home/End are
  all `components.NavFor` — never re-implement list navigation.
- `/` searches the list you are in; `Enter` keeps it narrowed, `Esc` clears before
  it means back.
- `y` copies what you would type, `Y` the link you would send. In the project picker
  the same split copies the SSH and HTTPS clone URLs.
- `t` folds the view's second box.

**A new key is three edits:** the handler, the view's `KeyHints()`, and
`helpEntries()` in `overlays.go`. `legend_test.go` pins the parts that matter. A
hint must not promise what the key cannot do — offer `j/k Scroll` only when there is
something to scroll.

## Style

- ANSI palette **by index only** (`components/styles.go`), so the UI sits inside the
  user's terminal theme. Body text sets no colour.
- The accent marks focus and nothing else; status colours are the only other
  saturated things on screen.
- Where focus covers a whole row or box, the accent is a *background*: the selected
  row is a band, the active tab a chip, the focused box's heading and rule accented.
- **Colour is never the only signal.** The active tab keeps its `[2]` brackets and
  the selected row its `▌`, so NO_COLOR still says where you are.
- Four weights of text: default for content, light grey for legends, grey for
  metadata, dimmed grey for structure.

## Releases and the updater

Tagging on master (`git tag v0.X.0 && git push origin v0.X.0`) triggers GoReleaser.

`lazyglab update` downloads a release asset **by name**, so `assetName` in
`internal/app/update.go` and `name_template` in `.goreleaser.yaml` are one contract:
change one and the update breaks on every platform at once, for people who already
installed. `checksums.txt` is not optional — without it the updater refuses rather
than run an unverified binary. It never overwrites a copy a package manager owns
(`managedBy`), and the swap is write-beside-then-rename, so a failure leaves the
working binary in place.

Verify a change to it against the real releases:

```bash
go build -ldflags "-X main.version=0.3.0" -o /tmp/lazyglab .
/tmp/lazyglab update
```

## Tests

- Name the behaviour, not the function: `TestSearch_EscClearsBeforeItMeansBack`.
- Assert what a user sees: `plain(s)` strips ANSI, so a failure prints the row as it
  appears on screen.
- Never pin behaviour to the clock or the calendar. `App.now` exists so tests can
  fix the time.
- The pre-commit hook runs build, vet, gofmt, golangci-lint and tests; it takes
  minutes, so it is slow rather than hung. `make check` is the same set.
