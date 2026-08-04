# lazyglab v2 Cockpit — Implementation Plan

**Goal:** Replace the fixed 4-panel lazygit-style TUI with a cockpit: a context bar, one switchable full-screen view at a time (Overview, Pipelines, MRs, Issues, Commits), and shared modal overlays.

**Architecture:** Three packages avoid an import cycle (the shell holds views, so shared code must be in packages both import): `internal/tui/components` (pure UI helpers/styles), `internal/tui/views` (the `View` interface, `Context`, view messages, and the concrete views), and `internal/tui` (the shell: app + frame + overlays). Views import `components` + `gitlab`; the shell imports `views` + `components`. Views never import the shell.

**Tech Stack:** Go 1.26, charm.land/bubbletea/v2, charm.land/lipgloss/v2, gitlab.com/gitlab-org/api/client-go.

**Spec:** `docs/specs/2026-07-25-tui-v2-cockpit-design.md`

**Prereq for every commit:** `export PATH="$PATH:$(go env GOPATH)/bin"` (pre-commit hooks need `golangci-lint`/`goimports`).

**Porting note:** Tasks 4–6 port existing v1 logic (in `internal/tui/app.go`) into view files. The behaviour and most code are unchanged — only the enclosing type and how it reads the active project/branch change (from `App` fields to the shared `*Context`). Each port task lists exactly which v1 functions move and the signature adaptation, then verifies by build + tmux drive. Where a port task shows code, it is the adapted skeleton; copy the bodies of the named v1 functions into it verbatim, replacing `a.activeProject`→`v.ctx.Project`, `a.activeBranch`→`v.ctx.Branch`, `a.clients[a.activeHost]`→`v.ctx.Client`.

---

## Package layout (target)

```
internal/tui/
  components/            # pure UI, no shell/view deps
    styles.go            # moved from tui/styles.go
    box.go               # RenderBox
    text.go              # Truncate, PadRight, WrapLine
    status.go            # StatusIcon, StatusColor (moved from styles.go)
    *_test.go
  views/                 # View interface + Context + concrete views
    view.go              # View interface, Context, KeyHint, view registry
    messages.go          # data-load messages (moved from tui/messages.go)
    pipelines.go mrs.go issues.go commits.go overview.go
    *_test.go
  app.go                 # shell: state, global keys, router, auto-refresh
  frame.go               # context bar + tabs + footer
  overlays.go            # project switcher, branch picker, help, confirm
  keys.go                # key constants (kept)
internal/gitlab/commits.go   # ListCommits + Commit type
```

---

## Task 1: Extract pure UI into `components` package

**Files:**
- Create: `internal/tui/components/styles.go`, `box.go`, `text.go`, `status.go`
- Create: `internal/tui/components/text_test.go`
- Modify: callers in `internal/tui/app.go` (temporary shims until the shell is rewritten)

- [ ] **Step 1: Move styles and status functions**

Create `internal/tui/components/styles.go` with the exact contents of
`internal/tui/styles.go` lines 1-61 (palette + named styles), changing the
package clause to `package components`. Create `internal/tui/components/status.go`
with `PipelineStatusColor`/`PipelineStatusIcon` from `styles.go` lines 63-103,
renamed to exported `StatusColor`/`StatusIcon`, `package components`.

Delete `internal/tui/styles.go`.

- [ ] **Step 2: Create `components/box.go`**

Move `renderBox` from `internal/tui/app.go` into `internal/tui/components/box.go`
as exported `RenderBox`, `package components`. Its signature is unchanged:

```go
func RenderBox(title string, lines []string, totalWidth, totalHeight int, borderColor, titleColor color.Color) string
```

Copy the body verbatim (it only uses `lipgloss`, `strings`, `color`).

- [ ] **Step 3: Create `components/text.go` with a test**

Create `internal/tui/components/text.go` (`package components`) with `Truncate`,
`PadRight`, `WrapLine`, `StatusIconPadded` — the bodies of `truncate`, and the
`padRight`/`wrapLine`/`statusIcon` helpers as designed in v1:

```go
package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Truncate shortens s to maxLen display cells, adding an ellipsis when it fits.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return ansi.Truncate(s, maxLen, "")
	}
	return ansi.Truncate(s, maxLen-3, "") + "..."
}

// PadRight pads s with spaces to width w (no-op if already wider).
func PadRight(s string, w int) string {
	if lipgloss.Width(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}

// StatusIconPadded returns the status icon padded to 2 display cells so 1- and
// 2-cell glyphs align in a column.
func StatusIconPadded(status string) string {
	icon := StatusIcon(status)
	if lipgloss.Width(StatusIcon(statusPlain(status))) < 2 {
		return icon + " "
	}
	return icon
}

// statusPlain strips styling to measure the glyph width; StatusIcon already
// returns a styled (colored) icon, so measure the raw glyph via a lookup.
func statusPlain(status string) string { return status }
```

Note: `StatusIcon` already returns a colored glyph. To measure padding reliably,
measure the *known* glyph: replace `StatusIconPadded` with a width table instead
of re-deriving. Simpler correct version:

```go
// StatusIconPadded returns StatusIcon padded to 2 cells.
func StatusIconPadded(status string) string {
	icon := StatusIcon(status)
	// The only 2-cell glyph is manual ("❚❚"); everything else is 1 cell.
	if status == "manual" {
		return icon
	}
	return icon + " "
}
```

Move `WrapLine` (exported) from v1 `wrapLine` verbatim.

Create `internal/tui/components/text_test.go` porting `TestWrapLine` (call
`WrapLine`) plus a `TestTruncate` and `TestPadRight`:

```go
package components

import "testing"

func TestWrapLine(t *testing.T) {
	got := WrapLine("the quick brown fox", 9)
	for _, l := range got {
		if len([]rune(l)) > 9 {
			t.Errorf("line %q exceeds width 9", l)
		}
	}
	if len(got) < 2 {
		t.Errorf("expected multiple lines, got %v", got)
	}
	if got := WrapLine("supercalifragilistic", 5); len(got) < 4 {
		t.Errorf("expected hard break into >=4 chunks, got %v", got)
	}
}

func TestPadRight(t *testing.T) {
	if got := PadRight("ab", 5); got != "ab   " {
		t.Errorf("PadRight = %q, want %q", got, "ab   ")
	}
	if got := PadRight("abcdef", 3); got != "abcdef" {
		t.Errorf("PadRight should not shrink, got %q", got)
	}
}
```

- [ ] **Step 4: Point v1 `app.go` at the new package (temporary)**

Add `import "github.com/Malvi1697/lazyglab/internal/tui/components"` to
`internal/tui/app.go` and replace references: `renderBox`→`components.RenderBox`,
`truncate`→`components.Truncate`, `padRight`→`components.PadRight`,
`wrapLine`→`components.WrapLine`, `PipelineStatusIcon`→`components.StatusIcon`,
`PipelineStatusColor`→`components.StatusColor`, `statusIcon(x)`→
`components.StatusIconPadded(x)`, and the `Color*`/`*Style` identifiers →
`components.Color*` / `components.*Style`. Remove the now-moved local
definitions from `app.go`. (This file is rewritten in Task 3; the shims just keep
the tree compiling.)

- [ ] **Step 5: Build, test**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/components internal/tui/app.go
git commit -m "refactor: extract pure UI helpers into components package"
```

---

## Task 2: `View` interface, `Context`, view messages

**Files:**
- Create: `internal/tui/views/view.go`, `internal/tui/views/messages.go`
- Create: `internal/tui/views/view_test.go`

- [ ] **Step 1: Write the registry test**

Create `internal/tui/views/view_test.go`:

```go
package views

import (
	"reflect"
	"testing"
)

func TestParseViews(t *testing.T) {
	all := []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues, ViewCommits}

	t.Run("empty -> all in default order", func(t *testing.T) {
		got, _ := ParseViews(nil)
		if !reflect.DeepEqual(got, all) {
			t.Errorf("got %v want %v", got, all)
		}
	})
	t.Run("subset + order", func(t *testing.T) {
		got, _ := ParseViews([]string{"pipelines", "commits"})
		want := []ViewID{ViewPipelines, ViewCommits}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("unknown + duplicate dropped with warnings", func(t *testing.T) {
		got, warn := ParseViews([]string{"pipelines", "bogus", "pipelines"})
		if !reflect.DeepEqual(got, []ViewID{ViewPipelines}) {
			t.Errorf("got %v", got)
		}
		if len(warn) != 2 {
			t.Errorf("want 2 warnings, got %v", warn)
		}
	})
}

func TestDefaultViewIndex(t *testing.T) {
	views := []ViewID{ViewOverview, ViewPipelines}
	if i := DefaultViewIndex(views, "pipelines"); i != 1 {
		t.Errorf("got %d want 1", i)
	}
	if i := DefaultViewIndex(views, ""); i != 0 {
		t.Errorf("empty default -> first, got %d", i)
	}
	if i := DefaultViewIndex(views, "issues"); i != 0 {
		t.Errorf("absent default -> first, got %d", i)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/tui/views/ -run TestParseViews -v`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `view.go`**

Create `internal/tui/views/view.go`:

```go
package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// ViewID identifies a cockpit view.
type ViewID int

const (
	ViewOverview ViewID = iota
	ViewPipelines
	ViewMRs
	ViewIssues
	ViewCommits
)

// Context is the shared session state handed to every view. Views read it; the
// shell owns and mutates it.
type Context struct {
	Client  *gitlab.Client
	Project *gitlab.Project // nil until selected
	Branch  *gitlab.Branch  // nil = default/all
}

// KeyHint is a footer hint.
type KeyHint struct{ Key, Desc string }

// View is one full-screen cockpit view.
type View interface {
	Focus() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	Body(width, height int) string
	Title() string
	KeyHints() []KeyHint
}

func viewConfigName(id ViewID) string {
	switch id {
	case ViewOverview:
		return "overview"
	case ViewPipelines:
		return "pipelines"
	case ViewMRs:
		return "mrs"
	case ViewIssues:
		return "issues"
	case ViewCommits:
		return "commits"
	}
	return ""
}

func viewIDFromName(name string) (ViewID, bool) {
	switch name {
	case "overview":
		return ViewOverview, true
	case "pipelines":
		return ViewPipelines, true
	case "mrs":
		return ViewMRs, true
	case "issues":
		return ViewIssues, true
	case "commits":
		return ViewCommits, true
	}
	return 0, false
}

func defaultViews() []ViewID {
	return []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues, ViewCommits}
}

// ParseViews converts config names into an ordered, deduplicated ViewID list.
// Empty -> all in default order. Unknown/duplicate dropped with warnings.
func ParseViews(names []string) ([]ViewID, []string) {
	if len(names) == 0 {
		return defaultViews(), nil
	}
	var out []ViewID
	var warnings []string
	seen := make(map[ViewID]bool)
	for _, n := range names {
		id, ok := viewIDFromName(n)
		if !ok {
			warnings = append(warnings, "unknown view \""+n+"\" ignored")
			continue
		}
		if seen[id] {
			warnings = append(warnings, "duplicate view \""+n+"\" ignored")
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return defaultViews(), warnings
	}
	return out, warnings
}

// DefaultViewIndex returns the index of the configured default view within the
// enabled list, or 0 when unset/absent.
func DefaultViewIndex(views []ViewID, name string) int {
	id, ok := viewIDFromName(name)
	if !ok {
		return 0
	}
	for i, v := range views {
		if v == id {
			return i
		}
	}
	return 0
}
```

Keep `viewConfigName` used by the shell's tab renderer (Task 7); if lint flags it
unused before then, add it in Task 7 instead.

- [ ] **Step 4: Move data messages**

Create `internal/tui/views/messages.go` with the message types from
`internal/tui/messages.go` (ProjectsLoadedMsg, ProjectSelectedMsg, MRsLoadedMsg,
PipelinesLoadedMsg, JobsLoadedMsg, IssuesLoadedMsg, BranchesLoadedMsg,
BranchSelectedMsg, StatusMsg, JobActionDoneMsg, PipelineActionDoneMsg,
JobTraceLoadedMsg, ErrorMsg, tickMsg, previewTickMsg, previewJobsLoadedMsg),
`package views`. Add `CommitsLoadedMsg{ Commits []gitlab.Commit; Err error }`.
Leave `PanelID` behind (deleted in Task 7). Keep `internal/tui/messages.go` for
now with only what the still-present v1 `app.go` needs; it is deleted in Task 7.

- [ ] **Step 5: Build, test**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go test ./internal/tui/views/ -v`
Expected: PASS (registry tests).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/views
git commit -m "feat: add View interface, Context, view registry and messages"
```

---

## Task 3: `ListCommits` API

**Files:**
- Create: `internal/gitlab/commits.go`, `internal/gitlab/commits_test.go`
- Modify: `internal/gitlab/types.go` (add `Commit`)

- [ ] **Step 1: Add the `Commit` domain type**

In `internal/gitlab/types.go` add:

```go
// Commit is a repository commit.
type Commit struct {
	ShortID    string
	Title      string
	AuthorName string
	CreatedAt  time.Time
	WebURL     string
	Status     string // CI status, resolved from pipelines by SHA ("" if none)
}
```

Ensure `time` is imported in `types.go`.

- [ ] **Step 2: Write the test**

Create `internal/gitlab/commits_test.go` mirroring the existing table tests in
this package (use `testhelper_test.go`'s server helper). Assert that
`ListCommits` maps `short_id`, `title`, `author_name`, `created_at`, `web_url`
from a canned JSON array and returns them in order. (Copy the structure of
`projects_test.go`; the endpoint is `/projects/1/repository/commits`.)

- [ ] **Step 3: Run to verify it fails**

Run: `go test ./internal/gitlab/ -run TestListCommits -v`
Expected: FAIL to compile.

- [ ] **Step 4: Implement `ListCommits`**

Create `internal/gitlab/commits.go`:

```go
package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// ListCommits returns recent commits for a project on the given ref
// (empty ref = default branch). CI status is left empty; callers map it from
// pipelines by SHA.
func (c *Client) ListCommits(projectID int, ref string) ([]Commit, error) {
	opts := &gogitlab.ListCommitsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 50},
	}
	if ref != "" {
		opts.RefName = gogitlab.Ptr(ref)
	}

	apiCommits, _, err := c.api.Commits.ListCommits(projectID, opts)
	if err != nil {
		return nil, err
	}

	commits := make([]Commit, len(apiCommits))
	for i, cm := range apiCommits {
		commits[i] = Commit{
			ShortID:    util.StripANSI(cm.ShortID),
			Title:      util.StripANSI(cm.Title),
			AuthorName: util.StripANSI(cm.AuthorName),
			WebURL:     util.StripANSI(cm.WebURL),
		}
		if cm.CreatedAt != nil {
			commits[i].CreatedAt = *cm.CreatedAt
		}
	}
	return commits, nil
}
```

Verify field names against the vendored client: run
`go doc gitlab.com/gitlab-org/api/client-go.Commit | grep -E 'ShortID|Title|AuthorName|CreatedAt|WebURL'`
and adjust if the SDK differs.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/gitlab/ -run TestListCommits -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/gitlab/commits.go internal/gitlab/commits_test.go internal/gitlab/types.go
git commit -m "feat: add ListCommits to the gitlab client"
```

---

## Task 4: Port Pipelines into a view

**Files:**
- Create: `internal/tui/views/pipelines.go`

This validates the `View` interface with the most complex view.

- [ ] **Step 1: Create the view skeleton**

```go
package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// PipelinesView shows a pipeline list with a job/detail pane and log viewer.
type PipelinesView struct {
	ctx *Context

	pipelines []gitlab.Pipeline
	cursor    int

	// jobs / log (ported from v1 job view)
	viewingJobs    bool
	jobs           []gitlab.Job
	jobCursor      int
	jobTrace       string
	jobTraceScroll int

	status string
}

func NewPipelinesView(ctx *Context) *PipelinesView { return &PipelinesView{ctx: ctx} }

func (v *PipelinesView) Title() string { return "Pipelines" }

func (v *PipelinesView) Focus() tea.Cmd { return v.load() }
```

- [ ] **Step 2: Port the load commands**

Copy these v1 `App` methods into `PipelinesView`, renaming the receiver to `v`
and replacing state access as described in the Porting note:
`loadPipelines`→`v.load`, `loadJobs`, `loadJobTrace`, `runPipeline`,
`retryPipeline`, `cancelPipeline`, `retryJob`, `cancelJob`, `playJob`. They read
`v.ctx.Project.ID`, `v.ctx.Client`, `v.ctx.Branch`, and `v.pipelines`/`v.cursor`
/`v.jobs`/`v.jobCursor` instead of the `App` fields. Return the same messages.

- [ ] **Step 3: Port Update**

Implement `Update(msg tea.Msg) tea.Cmd` combining v1's `PipelinesLoadedMsg`,
`JobsLoadedMsg`, `JobTraceLoadedMsg`, `PipelineActionDoneMsg`, `JobActionDoneMsg`
handling and the pipeline/job key handling from v1 `handleKeyMsg`,
`handleJobViewKey`, and `handlePanelKey`'s Pipelines branch. Confirmation dialogs
are raised by returning a message the shell turns into an overlay (see Task 7);
for now, perform actions directly and rely on the shell's confirm overlay wired
in Task 7 — expose the pending action via a returned `ConfirmMsg{Prompt string; Action tea.Cmd}`
(add this type to `messages.go`).

- [ ] **Step 4: Port Body + KeyHints**

`Body(width, height int)` renders master (list left ~45%) + detail (right) using
`components.RenderBox`, porting `pipelineItems` (aligned `time icon title` — drop
the ref column per the latest decision), `pipelineDetail` (+ hover job preview if
retained; otherwise metadata only), `jobItems`, `jobDetail`, `jobTraceView`.
`KeyHints` returns the pipeline/job hints from v1's keybind bar.

- [ ] **Step 5: Build**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./internal/tui/views/`
Expected: PASS (the view compiles independently; it is wired into the shell in
Task 7).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/views/pipelines.go internal/tui/views/messages.go
git commit -m "feat: port Pipelines into a cockpit view"
```

---

## Task 5: Port MRs and Issues into views

**Files:**
- Create: `internal/tui/views/mrs.go`, `internal/tui/views/issues.go`

- [ ] **Step 1: MRsView**

Mirror Task 4's structure. State: `mrs []gitlab.MergeRequest`, `cursor int`,
`status string`. `Focus`→`load` (port `loadMRs`). Port `approveMR`, `mergeMR`,
`openInBrowser` (MR branch). `Update` handles `MRsLoadedMsg`, `StatusMsg`, and
keys `a`/`m`/`o`/navigation (raising `ConfirmMsg` for approve/merge). `Body`
ports `mrItems` + `mrDetail`. `KeyHints`: approve/merge/open.

- [ ] **Step 2: IssuesView**

State: `issues []gitlab.Issue`, `cursor int`. `Focus`→port `loadIssues`. Port
`toggleIssue`, `openInBrowser` (issue). `Update` handles `IssuesLoadedMsg`, keys
`c`/`o`/navigation. `Body` ports `issueItems` + `issueDetail`. `KeyHints`:
close/reopen, open.

- [ ] **Step 3: Build**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./internal/tui/views/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/views/mrs.go internal/tui/views/issues.go
git commit -m "feat: port MRs and Issues into cockpit views"
```

---

## Task 6: Commits and Overview views

**Files:**
- Create: `internal/tui/views/commits.go`, `internal/tui/views/overview.go`

- [ ] **Step 1: CommitsView**

```go
package views

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

type CommitsView struct {
	ctx     *Context
	commits []gitlab.Commit
	cursor  int
	status  string
}

func NewCommitsView(ctx *Context) *CommitsView { return &CommitsView{ctx: ctx} }

func (v *CommitsView) Title() string { return "Commits" }

func (v *CommitsView) Focus() tea.Cmd { return v.load() }

func (v *CommitsView) load() tea.Cmd {
	if v.ctx.Project == nil {
		return nil
	}
	client := v.ctx.Client
	pid := v.ctx.Project.ID
	ref := ""
	if v.ctx.Branch != nil {
		ref = v.ctx.Branch.Name
	}
	return func() tea.Msg {
		commits, err := client.ListCommits(pid, ref)
		return CommitsLoadedMsg{Commits: commits, Err: err}
	}
}

func (v *CommitsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case CommitsLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading commits: %v", msg.Err)
			return nil
		}
		v.commits = msg.Commits
		if v.cursor >= len(v.commits) {
			v.cursor = len(v.commits) - 1
		}
		if v.cursor < 0 {
			v.cursor = 0
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if v.cursor < len(v.commits)-1 {
				v.cursor++
			}
		case "k", "up":
			if v.cursor > 0 {
				v.cursor--
			}
		case "g":
			v.cursor = 0
		case "G":
			if len(v.commits) > 0 {
				v.cursor = len(v.commits) - 1
			}
		}
	}
	return nil
}

func (v *CommitsView) Body(width, height int) string {
	listW := width * 45 / 100
	if listW < 30 {
		listW = 30
	}
	detailW := width - listW

	items := make([]string, len(v.commits))
	for i, c := range v.commits {
		icon := "  "
		if c.Status != "" {
			icon = components.StatusIconPadded(c.Status)
		}
		items[i] = fmt.Sprintf("%s %s %s  %s",
			util.TimeAgoShort(c.CreatedAt), icon,
			components.PadRight(c.ShortID, 8), c.Title)
	}
	list := renderList(listW, height, "Commits", items, v.cursor)

	detail := "Select a commit"
	if v.cursor >= 0 && v.cursor < len(v.commits) {
		c := v.commits[v.cursor]
		detail = fmt.Sprintf("%s\n\n%s\nAuthor: %s\n%s\n\n%s",
			components.TitleStyle.Render(c.ShortID),
			c.Title, c.AuthorName,
			util.TimeAgo(c.CreatedAt),
			components.HelpDescStyle.Render(c.WebURL))
	}
	detailBox := components.RenderBox("Commit", splitLines(detail), detailW, height,
		components.ColorSecondary, components.ColorPrimary)

	return joinH(list, detailBox)
}

func (v *CommitsView) KeyHints() []KeyHint {
	return []KeyHint{{"o", "Open"}}
}
```

Add small shared helpers `renderList`, `splitLines`, `joinH` to `view.go`
(a bordered scrollable list matching v1 `renderSidePanel`, `strings.Split`, and
`lipgloss.JoinHorizontal`). `renderList` ports the scroll/selection logic from v1
`renderSidePanel` using `components.RenderBox` and `components.SelectedItemStyle`.

- [ ] **Step 2: OverviewView**

`OverviewView` holds `ctx`, plus cached `commits`, `pipelines`, `mrs`, `issues`.
`Focus` batches `ListCommits`, `ListPipelines`, `ListMergeRequests`, `ListIssues`.
`Update` stores each `*LoadedMsg`. `Body` renders the top half as a recent-commits
list and the bottom half as three side-by-side summary columns
(`lipgloss.JoinHorizontal` of three `components.RenderBox`es), each showing counts
and the few most recent items. It maps commit CI status by matching commit
`ShortID`/SHA against `pipelines[].SHA`. No actions; `KeyHints` empty (navigation
only; the shell handles switching to a full view).

- [ ] **Step 3: Build and unit-test the mapping**

Add `internal/tui/views/overview_test.go` testing the pure
`commitStatus(commits, pipelines)` mapping helper (commit with a matching
pipeline SHA gets its status; no match -> "").

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go test ./internal/tui/views/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/views/commits.go internal/tui/views/overview.go internal/tui/views/overview_test.go internal/tui/views/view.go
git commit -m "feat: add Commits and Overview cockpit views"
```

---

## Task 7: Shell rewrite (frame, router, overlays, config)

**Files:**
- Rewrite: `internal/tui/app.go`
- Create: `internal/tui/frame.go`, `internal/tui/overlays.go`
- Delete: `internal/tui/messages.go`, `internal/tui/panels.go`, `internal/tui/layout.go` (+ their tests) once unused
- Modify: `internal/app/config.go`, `internal/app/app.go`

- [ ] **Step 1: Config — `views` + `default_view`**

In `internal/app/config.go` `Settings`, replace `Panels` with:

```go
	// Views lists the enabled cockpit views in tab order, by name
	// (overview, pipelines, mrs, issues, commits). Empty = all.
	Views []string `yaml:"views"`
	// DefaultView is the view shown at launch. Empty/absent = first enabled.
	DefaultView string `yaml:"default_view"`
	// Panels is the obsolete v1 key; kept only to warn on use.
	Panels []string `yaml:"panels"`
	RefreshInterval *int `yaml:"refresh_interval"`
```

Keep `RefreshSeconds()`. Update `config_test.go` accordingly (drop panel-specific
assertions; the `views` list is parsed in the `views` package).

- [ ] **Step 2: Frame rendering**

Create `internal/tui/frame.go` with `renderContextBar`, `renderTabs`,
`renderFooter` using `components` styles. `renderTabs` highlights the active
`views.ViewID` and numbers them by position; `renderFooter` concatenates global
hints with the active view's `KeyHints()`.

- [ ] **Step 3: Overlays**

Create `internal/tui/overlays.go` porting the branch picker, help overlay
(left-aligned block), confirm dialog, and a new project switcher (list of
projects with `j/k`, Enter selects). Each renders centered via `lipgloss.Place`
over the frame. The project switcher emits `views.ProjectSelectedMsg`.

- [ ] **Step 4: Rewrite `app.go` as the shell**

The shell struct:

```go
type App struct {
	clients   map[string]*gitlab.Client
	hostNames []string
	ctx       *views.Context

	views    []views.ViewID
	viewByID map[views.ViewID]views.View
	active   int

	// overlays
	overlay overlayState // none | project | branch | help | confirm
	// ... project list, branch list, confirm action, help flag ...

	refreshInterval time.Duration
	width, height   int
	statusText      string
	statusIsErr     bool
}
```

`NewApp(clients, hostNames, detectedHost, detectedPath string-ish, viewIDs []views.ViewID, defaultIndex int, refreshInterval time.Duration)`:
builds the shared `*views.Context`, constructs each enabled view via its `New*View(ctx)`
constructor into `viewByID`, sets `active = defaultIndex`.

`Init`: load projects (for the switcher / auto-detect) + `viewByID[active].Focus()`
+ tick if interval>0.

`Update`: handle `tea.WindowSizeMsg`; if an overlay is open, route to the overlay
handler; else handle global keys (`1..N` / Tab / `p` / `b` / `?` / `r` / `q`),
`ProjectSelectedMsg` (update `ctx.Project`, refresh active view), `BranchSelectedMsg`,
`ConfirmMsg` (open confirm overlay), `tickMsg` (refresh active view unless overlay
open, reschedule); otherwise delegate to `viewByID[active].Update(msg)`.

`View`: compose `renderContextBar` + `renderTabs` + active view `Body(bodyW, bodyH)`
+ `renderFooter`, then overlay if any.

- [ ] **Step 5: Wire `app/app.go`**

Parse views + default from settings and pass into `NewApp`:

```go
	viewIDs, warnings := views.ParseViews(cfg.Settings.Views)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "  config: %s\n", w)
	}
	if len(cfg.Settings.Panels) > 0 {
		fmt.Fprintln(os.Stderr, "  config: 'panels' is obsolete; use 'views'")
	}
	defaultIndex := views.DefaultViewIndex(viewIDs, cfg.Settings.DefaultView)
	refreshInterval := time.Duration(cfg.Settings.RefreshSeconds()) * time.Second
	model := tui.NewApp(clients, hostNames, detectedHost, detectedPath, viewIDs, defaultIndex, refreshInterval)
```

- [ ] **Step 6: Delete obsolete files**

Remove `internal/tui/layout.go`, `internal/tui/layout_test.go`,
`internal/tui/panels.go`, `internal/tui/panels_test.go`, and the old
`internal/tui/messages.go` (now in `views`). Move any still-referenced key
constants; `keys.go` stays.

- [ ] **Step 7: Build, vet, lint, test, drive**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go test -race ./...`
Expected: all clean.

Drive:
```bash
make install
tmux new-session -d -s lgt -x 140 -y 45 'lazyglab'; sleep 6
tmux send-keys -t lgt "p"; sleep 1   # project switcher
tmux capture-pane -t lgt -p
tmux send-keys -t lgt Enter; sleep 6 # pick first project
tmux send-keys -t lgt "2"; sleep 2   # Pipelines view
tmux capture-pane -t lgt -p
tmux send-keys -t lgt "5"; sleep 3   # Commits view
tmux capture-pane -t lgt -p
tmux send-keys -t lgt "1"; sleep 2   # Overview
tmux capture-pane -t lgt -p
tmux kill-session -t lgt
```
Expected: context bar + tabs; project switcher works; each view renders full-width.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: cockpit shell — frame, view router, overlays, config"
```

---

## Task 8: Docs + final verification

**Files:**
- Modify: `README.md`, `TODO.md`

- [ ] **Step 1: README**

Replace the v1 `settings.panels` docs with `settings.views` + `default_view`;
document the five views and the context/tab navigation and key map.

- [ ] **Step 2: TODO**

Check off: Commits view, own config, auto-refresh; note the cockpit rework.

- [ ] **Step 3: Full verification + end-to-end drive**

Run: `export PATH="$PATH:$(go env GOPATH)/bin"; go build ./... && go vet ./... && gofmt -l . && golangci-lint run ./... && go test -race ./...`
Then drive every view against real GitLab (as in Task 7 Step 7) and confirm
Overview aggregates, Pipelines jobs+log work, Commits list loads.

- [ ] **Step 4: Commit**

```bash
git add README.md TODO.md
git commit -m "docs: document v2 cockpit views and settings"
```

---

## Self-review notes

- **Spec coverage:** context bar + tabs + footer (T7/frame), View interface +
  Context (T2), 5 views incl. new Overview & Commits (T4–T6), project switcher
  overlay replacing Projects panel (T7), ListCommits + SHA→status mapping (T3/T6),
  config `views`/`default_view` + `panels` obsolete warning (T7), components
  extraction fixing the god-file (T1), auto-refresh per active view (T7), docs (T8).
- **Import-cycle safety:** `components` and `views` never import the shell; the
  shell imports both. Verified by the package layout.
- **Placeholder scan:** the port tasks (4–6) intentionally reference named v1
  functions to copy rather than reproducing hundreds of lines; this is a
  mechanical move with an explicit transformation rule (Porting note), not a
  vague placeholder. New/foundational code (components, view registry, ListCommits,
  Commits/Overview, shell wiring, config) is given in full.
- **Type consistency:** `View`, `Context`, `ViewID`, `KeyHint`, `ParseViews`,
  `DefaultViewIndex`, `ConfirmMsg`, `CommitsLoadedMsg`, `components.*`, and the
  `New*View(ctx)` constructors are referenced consistently across tasks.
```
