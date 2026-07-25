package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

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

func TestCommitsView_CopyFromListUsesFullSHA(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.commits = []gitlab.Commit{{
		ID: "a665c90dfull0000000000000000000000000000", ShortID: "a665c90d",
	}}

	cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("y should produce a copy command")
	}
	if !batchMentions(cmd, "a665c90d") {
		t.Error("expected the copy to be confirmed in the status bar")
	}
}

func TestCommitsView_CopyWithNoCommits(t *testing.T) {
	v := NewCommitsView(&Context{})
	if cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Error("y with an empty list must do nothing")
	}
}

func TestPipelinesView_SelectsPipelineForCommit(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20
	v.pipelines = []gitlab.Pipeline{
		{ID: 1, SHA: "1111111111111111111111111111111111111111"},
		{ID: 2, SHA: "bbb2222bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}

	// Commit SHAs are short, pipeline SHAs full: the match is by prefix.
	v.Update(ShowCommitPipelineMsg{ShortSHA: "bbb2222"})
	if v.cursor != 1 {
		t.Errorf("cursor = %d, want 1 (the pipeline for that commit)", v.cursor)
	}
}

func TestPipelinesView_WaitsForTheListToArrive(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.height = 20

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
