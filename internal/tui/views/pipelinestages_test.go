package views

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

func TestPipelineRow_SaysHowFarItGotStageByStage(t *testing.T) {
	// A pipeline list that only says "failed" answers the wrong question. Where it
	// stopped is the thing you open the view for, and GitLab's own list shows it as a
	// row of marks per stage.
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
	// GraphQL never sends them. Either way the row is a row, not a row of
	// placeholders pretending the pipeline has no stages.
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
	// The stages are a second message on purpose: the rows are worth drawing before
	// they land.
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
