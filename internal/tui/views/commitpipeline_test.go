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

func TestCommitRows_ShowAuthorNotHash(t *testing.T) {
	// The hash column was replaced by the author; the SHA is obtained with y.
	v := NewCommitsView(&Context{})
	v.commits = []gitlab.Commit{{
		ID:      "a665c90dfull0000000000000000000000000000",
		ShortID: "a665c90d", Title: "merge migrations", AuthorName: "Jan Všetíček",
	}}

	row := v.commitItems()[0]
	if !strings.Contains(row, "Jan Všetíček") {
		t.Errorf("row = %q, want the author name", row)
	}
	if strings.Contains(row, "a665c90d") {
		t.Errorf("row = %q, should no longer carry the hash", row)
	}
}

func TestOverviewRows_ShowAuthorNotHash(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "bbb2222", Title: "t", AuthorName: "Someone Else"}}

	row := v.commitItems()[0]
	if !strings.Contains(row, "Someone Else") {
		t.Errorf("row = %q, want the author name", row)
	}
	if strings.Contains(row, "bbb2222") {
		t.Errorf("row = %q, should no longer carry the hash", row)
	}
}

func TestCommitRows_LongAuthorIsTruncatedForAlignment(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.commits = []gitlab.Commit{
		{ShortID: "a", Title: "one", AuthorName: "Short"},
		{ShortID: "b", Title: "two", AuthorName: "A Very Long Contributor Name Indeed"},
	}

	rows := v.commitItems()
	// Titles must start at the same column in both rows.
	first := strings.Index(rows[0], "one")
	second := strings.Index(rows[1], "two")
	if first != second {
		t.Errorf("title columns differ: %d vs %d (%q / %q)", first, second, rows[0], rows[1])
	}
}

func TestCommitsView_CopyHashUsesFullSHA(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.commits = []gitlab.Commit{{
		ID: "a665c90dfull0000000000000000000000000000", ShortID: "a665c90d",
	}}

	cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("y should produce a copy command")
	}
	// The batch carries the clipboard write plus a confirmation for the status bar.
	if !batchMentions(cmd, "a665c90d") {
		t.Error("expected the copy to be confirmed in the status bar")
	}
}

func TestCommitsView_CopyFallsBackToShortSHA(t *testing.T) {
	// Commits loaded before the full SHA was mapped still copy something useful.
	v := NewCommitsView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "bbb2222"}}
	if cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd == nil {
		t.Fatal("y should still copy when only the short SHA is known")
	}
}

func TestCommitsView_CopyWithNoCommits(t *testing.T) {
	v := NewCommitsView(&Context{})
	if cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Error("y with an empty list must do nothing")
	}
}

// batchMentions runs a (possibly nested) batch and reports whether any StatusMsg
// mentions the given text.
func batchMentions(cmd tea.Cmd, text string) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if batchMentions(c, text) {
				return true
			}
		}
	case StatusMsg:
		return strings.Contains(msg.Text, text)
	}
	return false
}
