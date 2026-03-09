# Auto-detect Project + Collapsible Panels Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Auto-detect the GitLab project from the current directory's git remote and implement lazygit-style collapsible sidebar panels.

**Architecture:** Two independent features. (1) New `internal/util/gitremote.go` parses git remotes, returns host+path. App layer passes detected info to TUI. TUI matches against loaded projects after `ProjectsLoadedMsg`. (2) `ComputeLayout` changes to give focused panel full height, collapsed panels get 3 lines. `renderSidePanel` renders collapsed panels as a single content line.

**Tech Stack:** Go, os/exec (git remote -v), regexp (URL parsing), Bubble Tea v2

---

### Task 1: Git remote parser — tests

**Files:**
- Create: `internal/util/gitremote_test.go`

**Step 1: Write tests for URL parsing**

```go
package util

import "testing"

func TestParseGitLabRemote_HTTPS(t *testing.T) {
	host, path := ParseGitLabRemote("https://gitlab.com/owner/project.git")
	if host != "gitlab.com" || path != "owner/project" {
		t.Errorf("got host=%q path=%q", host, path)
	}
}

func TestParseGitLabRemote_SSH(t *testing.T) {
	host, path := ParseGitLabRemote("git@gitlab.com:owner/project.git")
	if host != "gitlab.com" || path != "owner/project" {
		t.Errorf("got host=%q path=%q", host, path)
	}
}

func TestParseGitLabRemote_Subgroup(t *testing.T) {
	host, path := ParseGitLabRemote("git@gitlab.com:org/team/project.git")
	if host != "gitlab.com" || path != "org/team/project" {
		t.Errorf("got host=%q path=%q", host, path)
	}
}

func TestParseGitLabRemote_NoSuffix(t *testing.T) {
	host, path := ParseGitLabRemote("https://gitlab.com/owner/project")
	if host != "gitlab.com" || path != "owner/project" {
		t.Errorf("got host=%q path=%q", host, path)
	}
}

func TestParseGitLabRemote_CustomHost(t *testing.T) {
	host, path := ParseGitLabRemote("https://gitlab.mycompany.com/team/repo.git")
	if host != "gitlab.mycompany.com" || path != "team/repo" {
		t.Errorf("got host=%q path=%q", host, path)
	}
}

func TestParseGitLabRemote_Invalid(t *testing.T) {
	host, path := ParseGitLabRemote("not-a-url")
	if host != "" || path != "" {
		t.Errorf("expected empty, got host=%q path=%q", host, path)
	}
}

func TestParseGitRemoteOutput(t *testing.T) {
	output := `origin	git@gitlab.com:owner/project.git (fetch)
origin	git@gitlab.com:owner/project.git (push)
upstream	https://gitlab.com/upstream/project.git (fetch)
upstream	https://gitlab.com/upstream/project.git (push)
`
	remotes := ParseGitRemoteOutput(output)
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}
	if remotes[0].Name != "origin" {
		t.Errorf("expected first remote to be origin, got %q", remotes[0].Name)
	}
}

func TestParseGitRemoteOutput_PreferOrigin(t *testing.T) {
	output := `upstream	https://gitlab.com/upstream/project.git (fetch)
upstream	https://gitlab.com/upstream/project.git (push)
origin	git@gitlab.com:owner/project.git (fetch)
origin	git@gitlab.com:owner/project.git (push)
`
	remotes := ParseGitRemoteOutput(output)
	// origin should be first regardless of order in output
	if remotes[0].Name != "origin" {
		t.Errorf("expected origin first, got %q", remotes[0].Name)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/util/ -run "TestParseGit" -v`
Expected: FAIL — functions not defined

---

### Task 2: Git remote parser — implementation

**Files:**
- Create: `internal/util/gitremote.go`

**Step 1: Implement the parser**

```go
package util

import (
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// GitRemote represents a parsed git remote.
type GitRemote struct {
	Name string // e.g. "origin"
	Host string // e.g. "gitlab.com"
	Path string // e.g. "owner/project"
}

var sshRemoteRegex = regexp.MustCompile(`^[\w.-]+@([\w.-]+):(.*?)(?:\.git)?$`)

// ParseGitLabRemote extracts host and project path from a git remote URL.
// Supports HTTPS and SSH formats. Returns empty strings if unparseable.
func ParseGitLabRemote(remoteURL string) (host, path string) {
	remoteURL = strings.TrimSpace(remoteURL)

	// Try SSH format: git@host:path.git
	if matches := sshRemoteRegex.FindStringSubmatch(remoteURL); len(matches) == 3 {
		return matches[1], strings.TrimSuffix(matches[2], "/")
	}

	// Try HTTPS format
	parsed, err := url.Parse(remoteURL)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	p := strings.TrimPrefix(parsed.Path, "/")
	p = strings.TrimSuffix(p, ".git")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return "", ""
	}
	return parsed.Host, p
}

// ParseGitRemoteOutput parses the output of `git remote -v` into GitRemote structs.
// Deduplicates by name (fetch lines only) and sorts with "origin" first.
func ParseGitRemoteOutput(output string) []GitRemote {
	seen := make(map[string]bool)
	var remotes []GitRemote

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "(fetch)") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := parts[0]
		if seen[name] {
			continue
		}
		seen[name] = true

		host, path := ParseGitLabRemote(parts[1])
		if host != "" && path != "" {
			remotes = append(remotes, GitRemote{Name: name, Host: host, Path: path})
		}
	}

	// Sort: origin first, then alphabetical
	sort.SliceStable(remotes, func(i, j int) bool {
		if remotes[i].Name == "origin" {
			return true
		}
		if remotes[j].Name == "origin" {
			return false
		}
		return remotes[i].Name < remotes[j].Name
	})

	return remotes
}

// DetectGitRemotes runs `git remote -v` in the current directory and returns parsed remotes.
// Returns nil if not in a git repo or on error.
func DetectGitRemotes() []GitRemote {
	cmd := exec.Command("git", "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return ParseGitRemoteOutput(string(out))
}
```

**Step 2: Run tests to verify they pass**

Run: `go test ./internal/util/ -run "TestParseGit" -v`
Expected: PASS

**Step 3: Commit**

```bash
git add internal/util/gitremote.go internal/util/gitremote_test.go
git commit -m "feat: add git remote parser for project auto-detection"
```

---

### Task 3: Pass detected remote to TUI

**Files:**
- Modify: `internal/app/app.go:13-34` (Run function + NewApp call)
- Modify: `internal/tui/app.go:16-77` (App struct + NewApp)
- Modify: `internal/tui/messages.go` (new message type)

**Step 1: Add DetectedProject field to App struct**

In `internal/tui/app.go`, add to the App struct after `activeProject`:

```go
// Auto-detected project from git remote
detectedHost string // host from git remote, e.g. "gitlab.com"
detectedPath string // project path from git remote, e.g. "owner/project"
```

**Step 2: Update NewApp signature**

In `internal/tui/app.go`, change `NewApp` to accept detected remote info:

```go
func NewApp(clients map[string]*gitlab.Client, hostNames []string, detectedHost, detectedPath string) *App {
```

Store the values in the returned App.

**Step 3: Update app.go to detect and pass remote**

In `internal/app/app.go`, in `Run()`, after `buildClients`:

```go
// Auto-detect project from git remote
var detectedHost, detectedPath string
remotes := util.DetectGitRemotes()
for _, r := range remotes {
    if _, ok := clients[r.Host]; ok {
        detectedHost = r.Host
        detectedPath = r.Path
        break
    }
}
```

Add import for `util` package. Pass `detectedHost, detectedPath` to `tui.NewApp`.

**Step 4: Auto-select on ProjectsLoadedMsg**

In `internal/tui/app.go`, in the `ProjectsLoadedMsg` handler (lines 110-120), after setting `a.projects`, add auto-select logic:

```go
// Auto-select project from git remote detection
if a.detectedPath != "" && a.activeProject == nil {
    for _, p := range a.projects {
        if strings.EqualFold(p.NameWithNamespace, a.detectedPath) {
            a.detectedPath = "" // clear so it doesn't re-trigger
            return a, func() tea.Msg {
                return ProjectSelectedMsg{Project: p}
            }
        }
    }
}
```

**Step 5: Run the app to verify**

Run: `go build -o lazyglab . && LAZYGLAB_CONFIG=~/.config/lazyglab/config.yml ./lazyglab`
Expected: If running inside a git repo with a GitLab remote matching a configured host, the project auto-selects.

**Step 6: Commit**

```bash
git add internal/app/app.go internal/tui/app.go
git commit -m "feat: auto-detect project from git remote"
```

---

### Task 4: Collapsible panels — layout

**Files:**
- Modify: `internal/tui/layout.go:24-67` (ComputeLayout function)

**Step 1: Change ComputeLayout to collapse unfocused panels**

Replace the equal distribution logic (lines 54-62) with:

```go
// Collapsed panels get 3 lines (top border + 1 content + bottom border)
collapsedHeight := 3
numCollapsed := 3 // 3 of 4 panels are collapsed
expandedHeight := usableHeight - (collapsedHeight * numCollapsed)
if expandedHeight < 5 {
    expandedHeight = 5
}

for i := range l.PanelHeights {
    if PanelID(i) == activePanel {
        l.PanelHeights[i] = expandedHeight
    } else {
        l.PanelHeights[i] = collapsedHeight
    }
}
```

**Step 2: Run build to verify**

Run: `go build ./...`
Expected: Compiles without errors.

**Step 3: Commit**

```bash
git add internal/tui/layout.go
git commit -m "feat: collapsible panel layout — focused panel expands"
```

---

### Task 5: Collapsible panels — collapsed rendering

**Files:**
- Modify: `internal/tui/app.go:606-688` (View + renderSidePanel)

**Step 1: Add collapsed content methods**

Add to `internal/tui/app.go` after the `projectItems()` function:

```go
func (a *App) collapsedProjectLine() string {
	if a.activeProject == nil {
		return "No project selected"
	}
	branch := a.activeProject.DefaultBranch
	if a.activeBranch != nil {
		branch = a.activeBranch.Name
	}
	return a.activeProject.NameWithNamespace + " → " + branch
}

func (a *App) collapsedMRLine() string {
	idx := a.cursor[PanelMergeRequests]
	if idx >= 0 && idx < len(a.mrs) {
		mr := a.mrs[idx]
		prefix := fmt.Sprintf("!%d ", mr.IID)
		return prefix + mr.Title
	}
	if len(a.mrs) == 0 {
		return "No merge requests"
	}
	return a.mrs[0].Title
}

func (a *App) collapsedPipelineLine() string {
	idx := a.cursor[PanelPipelines]
	if idx >= 0 && idx < len(a.pipelines) {
		p := a.pipelines[idx]
		return fmt.Sprintf("#%d %s (%s)", p.ID, p.Status, p.Ref)
	}
	if len(a.pipelines) == 0 {
		return "No pipelines"
	}
	return fmt.Sprintf("#%d %s", a.pipelines[0].ID, a.pipelines[0].Status)
}

func (a *App) collapsedIssueLine() string {
	idx := a.cursor[PanelIssues]
	if idx >= 0 && idx < len(a.issues) {
		issue := a.issues[idx]
		return fmt.Sprintf("#%d %s", issue.IID, issue.Title)
	}
	if len(a.issues) == 0 {
		return "No issues"
	}
	return fmt.Sprintf("#%d %s", a.issues[0].IID, a.issues[0].Title)
}
```

**Step 2: Update View() to pass collapsed content**

In `View()` (line 617), change the sidebar rendering to conditionally pass collapsed content:

```go
sidebar := lipgloss.JoinVertical(lipgloss.Left,
    a.renderSidePanelSmart(PanelProjects, "Projects", a.projectItems(), a.collapsedProjectLine()),
    a.renderSidePanelSmart(PanelMergeRequests, "Merge Requests", a.mrItems(), a.collapsedMRLine()),
    a.renderSidePanelSmart(PanelPipelines, a.pipelinePanelTitle(), a.pipelineItems(), a.collapsedPipelineLine()),
    a.renderSidePanelSmart(PanelIssues, "Issues", a.issueItems(), a.collapsedIssueLine()),
)
```

**Step 3: Add renderSidePanelSmart**

Add a new method that delegates to renderSidePanel for expanded or renders a collapsed box:

```go
func (a *App) renderSidePanelSmart(id PanelID, title string, items []string, collapsedLine string) string {
	if a.activePanel == id {
		return a.renderSidePanel(id, title, items)
	}
	// Collapsed: 3 lines total (top border + 1 content + bottom border)
	totalWidth := a.layout.SidebarWidth
	panelHeight := a.layout.PanelHeights[id]
	titleText := fmt.Sprintf("[%d] %s", int(id)+1, title)
	line := truncate(collapsedLine, totalWidth-4)
	return renderBox(titleText, []string{line}, totalWidth, panelHeight, ColorSecondary, ColorSecondary)
}
```

**Step 4: Run and visually verify**

Run: `go build -o lazyglab . && ./lazyglab`
Expected: Only the focused panel is expanded. Others show single-line summaries. Switching panels with Tab/1-4 expands the new panel and collapses the old one.

**Step 5: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: collapsible sidebar panels with single-line summaries"
```

---

### Task 6: Final integration test and cleanup

**Files:**
- Modify: `internal/tui/layout.go` (if any edge cases found)
- Modify: `internal/tui/app.go` (if any edge cases found)

**Step 1: Run all tests**

Run: `make check`
Expected: All tests pass, no lint errors.

**Step 2: Run app and verify full flow**

Run: `go build -o lazyglab . && ./lazyglab`
Verify:
- Auto-detects project from git remote (if in a matching repo)
- Panels collapse/expand correctly on switch
- Projects panel shows `namespace/project → branch` when collapsed
- Navigation (j/k, Enter, Esc) works correctly in expanded panels
- No visual glitches at small/large terminal sizes

**Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: polish auto-detect and collapsible panel edge cases"
```
