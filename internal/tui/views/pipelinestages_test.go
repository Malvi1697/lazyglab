package views

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

func TestPipelineRow_SaysHowFarItGotStageByStage(t *testing.T) {
	// A pipeline list that only says "failed" answers the wrong question.
	p := gitlab.Pipeline{ID: 1, Status: "failed", CommitTitle: "feat: promote", Ref: "main",
		CreatedAt: time.Now()}
	stages := []gitlab.Stage{
		{Name: "lint", Status: "success"},
		{Name: "build", Status: "success"},
		{Name: "test", Status: "failed"},
		{Name: "deploy", Status: "skipped"},
	}

	row := renderRow(pipelineRow(p, stages), 120)
	marks := ansi.Strip(stageMarks(stages))

	if !strings.Contains(ansi.Strip(row), marks) {
		t.Errorf("row = %q, want the stage marks %q in it", ansi.Strip(row), marks)
	}
	// One mark per stage, in order, each in its status's own colour — the marks are
	// pre-styled, so the row must not repaint them as metadata grey.
	if strings.Count(marks, " ") != len(stages)-1 {
		t.Errorf("marks = %q, want one per stage", marks)
	}
	for _, want := range []string{
		ansi.Strip(components.StatusIcon("success")),
		ansi.Strip(components.StatusIcon("failed")),
		ansi.Strip(components.StatusIcon("skipped")),
	} {
		if !strings.Contains(marks, want) {
			t.Errorf("marks = %q, want a %q in them", marks, want)
		}
	}
	if !strings.Contains(row, components.StatusIcon("failed")) {
		t.Errorf("row = %q, want the failed stage still carrying its colour", row)
	}
}

func TestPipelineRow_NoStagesMeansNoMarks(t *testing.T) {
	// The marks follow the list rather than coming with it, and an instance without
	// GraphQL never sends them.
	p := gitlab.Pipeline{ID: 1, Status: "success", CommitTitle: "feat: promote", Ref: "main"}

	row := ansi.Strip(renderRow(pipelineRow(p, nil), 120))
	if strings.Contains(row, ansi.Strip(components.StatusIcon("skipped"))) {
		t.Errorf("row = %q, want no invented marks", row)
	}
	if !strings.Contains(row, "promote") {
		t.Errorf("row = %q, want the commit title", row)
	}
}

func TestPipelinesView_StagesArriveAfterTheListAndReachTheRows(t *testing.T) {
	// The stages are a second message on purpose: the rows are worth drawing before they
	// land.
	v := NewPipelinesView(&Context{})
	v.width, v.height = 120, 20
	v.Update(PipelinesLoadedMsg{Pipelines: []gitlab.Pipeline{
		{ID: 77, Status: "failed", CommitTitle: "fix: crash", Ref: "main"},
	}})

	before := ansi.Strip(v.Body(120, 20))
	if !strings.Contains(before, "crash") {
		t.Fatalf("the list should draw before the stages arrive:\n%s", before)
	}

	v.Update(PipelineStagesLoadedMsg{Stages: map[int][]gitlab.Stage{
		77: {{Name: "lint", Status: "success"}, {Name: "test", Status: "failed"}},
	}})

	after := ansi.Strip(v.Body(120, 20))
	if !strings.Contains(after, ansi.Strip(stageMarks([]gitlab.Stage{
		{Status: "success"}, {Status: "failed"},
	}))) {
		t.Errorf("the marks never reached the row:\n%s", after)
	}
}

func TestJobsPanel_StagesReadInTheOrderThePipelineRanThem(t *testing.T) {
	// GitLab lists jobs newest first, so the panel used to open on the last stage and end
	// at the first.
	var p jobsPanel
	p.setJobs([]gitlab.Job{
		{ID: 90, Name: "deploy:prod", Stage: "deploy", Status: "canceled"},
		{ID: 80, Name: "tests:api", Stage: "test", Status: "failed"},
		{ID: 71, Name: "lint:python", Stage: "lint", Status: "success"},
		{ID: 70, Name: "lint:frontend", Stage: "lint", Status: "success"},
	})

	rows, _ := p.items()
	var order []string
	for _, row := range rows {
		if strings.HasPrefix(row, "\x00") {
			order = append(order, strings.TrimSpace(ansi.Strip(row[1:])))
		}
	}
	want := []string{"lint", "test", "deploy"}
	if len(order) != len(want) {
		t.Fatalf("stages = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Errorf("stages = %v, want %v", order, want)
			break
		}
	}
	// Within a stage the jobs keep the order they were created in.
	if p.jobs[0].Name != "lint:frontend" || p.jobs[1].Name != "lint:python" {
		t.Errorf("lint jobs = %q, %q, want frontend then python", p.jobs[0].Name, p.jobs[1].Name)
	}
}

func TestPipelines_TNamesTheStagesBehindTheMarks(t *testing.T) {
	// "How do I tell which mark is which stage?" — this is the answer, out of the stages
	// already in hand, so it costs no request and no drilling in.
	v := NewPipelinesView(&Context{})
	v.width, v.height = 120, 24
	v.Update(PipelinesLoadedMsg{Pipelines: []gitlab.Pipeline{
		{ID: 1042, Status: "canceled", CommitTitle: "fix: restore the admin screen", Ref: "main"},
	}})
	v.Update(PipelineStagesLoadedMsg{Stages: map[int][]gitlab.Stage{
		1042: {
			{Name: "lint", Status: "success", Jobs: 2},
			{Name: "build", Status: "canceled", Jobs: 2},
			{Name: "operations", Status: "canceled", Jobs: 4},
		},
	}})

	// Folded to begin with: the list is what the page is for.
	if body := plain(v.Body(120, 24)); strings.Contains(body, "operations") {
		t.Errorf("the stages box should start folded:\n%s", body)
	}
	// Every foldable box says it the same way: Show / Hide, and which box.
	if !hasHint(v.KeyHints(), "t", "Show stages") {
		t.Errorf("hints = %v, want t offering to show them", v.KeyHints())
	}

	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	body := plain(v.Body(120, 24))

	for _, want := range []string{"Stages (#1042)", "lint", "2 jobs", "operations", "4 jobs"} {
		if !strings.Contains(body, want) {
			t.Errorf("body should carry %q:\n%s", want, body)
		}
	}
	// Named in the order the pipeline ran them, which is the order of the marks on its row
	// and of the groups Enter opens.
	if at, then := strings.Index(body, "lint"), strings.Index(body, "operations"); at > then {
		t.Errorf("stages are named out of order:\n%s", body)
	}
	// And the list is still there above it.
	if !strings.Contains(body, "admin screen") {
		t.Errorf("the list should keep the rest of the page:\n%s", body)
	}
}
