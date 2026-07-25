package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

var enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}

func TestCommitsView_EnterAsksForThePipeline(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", Title: "first"}, {ShortID: "bbb2222", Title: "second"}}
	v.cursor = 1

	cmd := v.Update(enterKey)
	if cmd == nil {
		t.Fatal("Enter on a commit should produce a command")
	}
	msg, ok := cmd().(ShowCommitPipelineMsg)
	if !ok {
		t.Fatalf("expected ShowCommitPipelineMsg, got %T", cmd())
	}
	if msg.ShortSHA != "bbb2222" {
		t.Errorf("ShortSHA = %q, want the selected commit", msg.ShortSHA)
	}
}

func TestOverviewView_EnterAsksForThePipeline(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "ccc3333"}}

	cmd := v.Update(enterKey)
	if cmd == nil {
		t.Fatal("Enter on a commit should produce a command")
	}
	if msg := cmd().(ShowCommitPipelineMsg); msg.ShortSHA != "ccc3333" {
		t.Errorf("ShortSHA = %q, want ccc3333", msg.ShortSHA)
	}
}

func TestCommitsView_EnterWithNoCommitsDoesNothing(t *testing.T) {
	v := NewCommitsView(&Context{})
	if cmd := v.Update(enterKey); cmd != nil {
		t.Error("Enter with an empty list must do nothing")
	}
}

func TestPipelinesView_SelectsPipelineForCommit(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20
	v.pipelines = []gitlab.Pipeline{
		{ID: 1, SHA: "1111111111111111111111111111111111111111"},
		{ID: 2, SHA: "bbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		{ID: 3, SHA: "3333333333333333333333333333333333333333"},
	}

	// Commit SHAs are short, pipeline SHAs full: the match is by prefix.
	v.Update(ShowCommitPipelineMsg{ShortSHA: "bbb2222"})
	if v.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (the pipeline for that commit)", v.cursor)
	}
	if v.pendingSHA != "" {
		t.Error("a resolved request should not stay pending")
	}
}

func TestPipelinesView_WaitsForTheListToArrive(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20

	// Arriving before pipelines are loaded: the request has to survive the load.
	v.Update(ShowCommitPipelineMsg{ShortSHA: "bbb2222"})
	if v.pendingSHA != "bbb2222" {
		t.Fatalf("pendingSHA = %q, want it kept until the list arrives", v.pendingSHA)
	}

	v.Update(PipelinesLoadedMsg{Pipelines: []gitlab.Pipeline{
		{ID: 9, SHA: "9999999999999999999999999999999999999999"},
		{ID: 2, SHA: "bbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}})
	if v.cursor != 1 {
		t.Errorf("cursor = %d, want 1 after the load", v.cursor)
	}
	if v.pendingSHA != "" {
		t.Error("the request should be cleared once satisfied")
	}
}

func TestPipelinesView_ReportsCommitWithoutPipeline(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20

	v.Update(ShowCommitPipelineMsg{ShortSHA: "deadbee"})
	v.Update(PipelinesLoadedMsg{Pipelines: []gitlab.Pipeline{
		{ID: 1, SHA: "1111111111111111111111111111111111111111"},
	}})

	if v.pendingSHA != "" {
		t.Error("an unsatisfiable request must not linger")
	}
	if !strings.Contains(v.status, "deadbee") {
		t.Errorf("status = %q, want it to name the commit with no pipeline", v.status)
	}
}

func TestPipelinesView_JumpLeavesJobDrilldown(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20
	v.pipelines = []gitlab.Pipeline{{ID: 2, SHA: "bbb2222bbbb"}}
	v.viewingJobs = true
	v.jobs = []gitlab.Job{{ID: 5, Name: "build"}}
	v.jobTrace = "some log"

	v.Update(ShowCommitPipelineMsg{ShortSHA: "bbb2222"})

	if v.viewingJobs || v.jobTrace != "" || v.jobs != nil {
		t.Error("jumping to a commit's pipeline should leave the job/log drill-down")
	}
}
