package tui

import (
	"fmt"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// --- truncate tests ---

func TestTruncate_EmptyString(t *testing.T) {
	got := truncate("", 10)
	if got != "" {
		t.Errorf("truncate(\"\", 10) = %q, want \"\"", got)
	}
}

func TestTruncate_ShortString(t *testing.T) {
	got := truncate("hello", 10)
	if got != "hello" {
		t.Errorf("truncate(\"hello\", 10) = %q, want \"hello\"", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	got := truncate("hello", 5)
	if got != "hello" {
		t.Errorf("truncate(\"hello\", 5) = %q, want \"hello\"", got)
	}
}

func TestTruncate_NeedsTruncation(t *testing.T) {
	got := truncate("hello world", 8)
	// maxLen=8, so s[:5] + "..." = "hello..."
	want := "hello..."
	if got != want {
		t.Errorf("truncate(\"hello world\", 8) = %q, want %q", got, want)
	}
}

func TestTruncate_MaxLenZero(t *testing.T) {
	got := truncate("hello", 0)
	if got != "" {
		t.Errorf("truncate(\"hello\", 0) = %q, want \"\"", got)
	}
}

func TestTruncate_MaxLenNegative(t *testing.T) {
	got := truncate("hello", -1)
	if got != "" {
		t.Errorf("truncate(\"hello\", -1) = %q, want \"\"", got)
	}
}

func TestTruncate_MaxLenOne(t *testing.T) {
	// maxLen=1, which is <= 3, so just s[:1]
	got := truncate("hello", 1)
	if got != "h" {
		t.Errorf("truncate(\"hello\", 1) = %q, want \"h\"", got)
	}
}

func TestTruncate_MaxLenTwo(t *testing.T) {
	got := truncate("hello", 2)
	if got != "he" {
		t.Errorf("truncate(\"hello\", 2) = %q, want \"he\"", got)
	}
}

func TestTruncate_MaxLenThree(t *testing.T) {
	got := truncate("hello", 3)
	if got != "hel" {
		t.Errorf("truncate(\"hello\", 3) = %q, want \"hel\"", got)
	}
}

func TestTruncate_MaxLenFour(t *testing.T) {
	// maxLen=4, > 3, so s[:1] + "..." = "h..."
	got := truncate("hello", 4)
	if got != "h..." {
		t.Errorf("truncate(\"hello\", 4) = %q, want \"h...\"", got)
	}
}

// --- collapsedProjectLine tests ---

func TestCollapsedProjectLine_NoProject(t *testing.T) {
	a := &App{}
	got := a.collapsedProjectLine()
	if got != "No project selected" {
		t.Errorf("collapsedProjectLine() = %q, want \"No project selected\"", got)
	}
}

func TestCollapsedProjectLine_WithProject(t *testing.T) {
	a := &App{
		activeProject: &gitlab.Project{
			NameWithNamespace: "group / myproject",
			DefaultBranch:     "main",
		},
	}
	got := a.collapsedProjectLine()
	want := "group / myproject → main"
	if got != want {
		t.Errorf("collapsedProjectLine() = %q, want %q", got, want)
	}
}

func TestCollapsedProjectLine_WithActiveBranch(t *testing.T) {
	a := &App{
		activeProject: &gitlab.Project{
			NameWithNamespace: "group / myproject",
			DefaultBranch:     "main",
		},
		activeBranch: &gitlab.Branch{
			Name: "feature/test",
		},
	}
	got := a.collapsedProjectLine()
	want := "group / myproject → feature/test"
	if got != want {
		t.Errorf("collapsedProjectLine() = %q, want %q", got, want)
	}
}

// --- collapsedMRLine tests ---

func TestCollapsedMRLine_Empty(t *testing.T) {
	a := &App{}
	got := a.collapsedMRLine()
	if got != "No merge requests" {
		t.Errorf("collapsedMRLine() = %q, want \"No merge requests\"", got)
	}
}

func TestCollapsedMRLine_WithMRs(t *testing.T) {
	a := &App{
		mrs: []gitlab.MergeRequest{
			{IID: 123, Title: "Fix authentication bug"},
			{IID: 124, Title: "Add tests"},
		},
	}
	got := a.collapsedMRLine()
	want := "!123 Fix authentication bug"
	if got != want {
		t.Errorf("collapsedMRLine() = %q, want %q", got, want)
	}
}

func TestCollapsedMRLine_CursorOnSecondMR(t *testing.T) {
	a := &App{
		mrs: []gitlab.MergeRequest{
			{IID: 123, Title: "Fix authentication bug"},
			{IID: 124, Title: "Add tests"},
		},
	}
	a.cursor[PanelMergeRequests] = 1
	got := a.collapsedMRLine()
	want := "!124 Add tests"
	if got != want {
		t.Errorf("collapsedMRLine() = %q, want %q", got, want)
	}
}

func TestCollapsedMRLine_CursorOutOfBounds(t *testing.T) {
	a := &App{
		mrs: []gitlab.MergeRequest{
			{IID: 123, Title: "Fix authentication bug"},
		},
	}
	a.cursor[PanelMergeRequests] = 5 // out of bounds
	got := a.collapsedMRLine()
	// Falls through to the fallback: first MR
	want := "!123 Fix authentication bug"
	if got != want {
		t.Errorf("collapsedMRLine() = %q, want %q", got, want)
	}
}

// --- collapsedPipelineLine tests ---

func TestCollapsedPipelineLine_Empty(t *testing.T) {
	a := &App{}
	got := a.collapsedPipelineLine()
	if got != "No pipelines" {
		t.Errorf("collapsedPipelineLine() = %q, want \"No pipelines\"", got)
	}
}

func TestCollapsedPipelineLine_WithPipelines(t *testing.T) {
	a := &App{
		pipelines: []gitlab.Pipeline{
			{ID: 456, Status: "success", Ref: "main"},
			{ID: 457, Status: "running", Ref: "develop"},
		},
	}
	got := a.collapsedPipelineLine()
	want := fmt.Sprintf("#456 %s success (main)", PipelineStatusIcon("success"))
	if got != want {
		t.Errorf("collapsedPipelineLine() = %q, want %q", got, want)
	}
}

func TestCollapsedPipelineLine_FailedPipeline(t *testing.T) {
	a := &App{
		pipelines: []gitlab.Pipeline{
			{ID: 100, Status: "failed", Ref: "feature/broken"},
		},
	}
	got := a.collapsedPipelineLine()
	want := fmt.Sprintf("#100 %s failed (feature/broken)", PipelineStatusIcon("failed"))
	if got != want {
		t.Errorf("collapsedPipelineLine() = %q, want %q", got, want)
	}
}

func TestCollapsedPipelineLine_CursorOnSecond(t *testing.T) {
	a := &App{
		pipelines: []gitlab.Pipeline{
			{ID: 456, Status: "success", Ref: "main"},
			{ID: 457, Status: "running", Ref: "develop"},
		},
	}
	a.cursor[PanelPipelines] = 1
	got := a.collapsedPipelineLine()
	want := fmt.Sprintf("#457 %s running (develop)", PipelineStatusIcon("running"))
	if got != want {
		t.Errorf("collapsedPipelineLine() = %q, want %q", got, want)
	}
}

func TestCollapsedPipelineLine_CursorOutOfBounds(t *testing.T) {
	a := &App{
		pipelines: []gitlab.Pipeline{
			{ID: 456, Status: "success", Ref: "main"},
		},
	}
	a.cursor[PanelPipelines] = 10
	got := a.collapsedPipelineLine()
	want := fmt.Sprintf("#456 %s success", PipelineStatusIcon("success"))
	if got != want {
		t.Errorf("collapsedPipelineLine() = %q, want %q", got, want)
	}
}

// --- collapsedIssueLine tests ---

func TestCollapsedIssueLine_Empty(t *testing.T) {
	a := &App{}
	got := a.collapsedIssueLine()
	if got != "No issues" {
		t.Errorf("collapsedIssueLine() = %q, want \"No issues\"", got)
	}
}

func TestCollapsedIssueLine_WithIssues(t *testing.T) {
	a := &App{
		issues: []gitlab.Issue{
			{IID: 89, Title: "Bug in login page"},
			{IID: 90, Title: "Feature request: dark mode"},
		},
	}
	got := a.collapsedIssueLine()
	want := "#89 Bug in login page"
	if got != want {
		t.Errorf("collapsedIssueLine() = %q, want %q", got, want)
	}
}

func TestCollapsedIssueLine_CursorOnSecond(t *testing.T) {
	a := &App{
		issues: []gitlab.Issue{
			{IID: 89, Title: "Bug in login page"},
			{IID: 90, Title: "Feature request: dark mode"},
		},
	}
	a.cursor[PanelIssues] = 1
	got := a.collapsedIssueLine()
	want := "#90 Feature request: dark mode"
	if got != want {
		t.Errorf("collapsedIssueLine() = %q, want %q", got, want)
	}
}

func TestCollapsedIssueLine_CursorOutOfBounds(t *testing.T) {
	a := &App{
		issues: []gitlab.Issue{
			{IID: 89, Title: "Bug in login page"},
		},
	}
	a.cursor[PanelIssues] = 99
	got := a.collapsedIssueLine()
	// Falls back to first issue
	want := "#89 Bug in login page"
	if got != want {
		t.Errorf("collapsedIssueLine() = %q, want %q", got, want)
	}
}
