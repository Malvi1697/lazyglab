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
	v.items = []gitlab.Commit{{
		ID:      "a665c90dfull0000000000000000000000000000",
		ShortID: "a665c90d", Title: "merge migrations", AuthorName: "Jan Všetíček",
	}}

	row := renderRow(commitItemRow(v.visible()[0]), 120)
	if !strings.Contains(row, "Jan Všetíček") {
		t.Errorf("row = %q, want the author name", row)
	}
	if strings.Contains(row, "a665c90d") {
		t.Errorf("row = %q, should no longer carry the hash", row)
	}
}

func TestOverviewRows_ShowAuthorNotHash(t *testing.T) {
	v := NewDashboardView(&Context{})
	v.items = []gitlab.Commit{{ShortID: "bbb2222", Title: "t", AuthorName: "Someone Else"}}

	row := renderRow(v.commitRow(v.visible()[0]), 120)
	if !strings.Contains(row, "Someone Else") {
		t.Errorf("row = %q, want the author name", row)
	}
	if strings.Contains(row, "bbb2222") {
		t.Errorf("row = %q, should no longer carry the hash", row)
	}
}

func TestCommitRows_LongAuthorIsTruncatedForAlignment(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.items = []gitlab.Commit{
		{ShortID: "a", Title: "one", AuthorName: "Short"},
		{ShortID: "b", Title: "two", AuthorName: "A Very Long Contributor Name Indeed"},
	}

	visible := v.visible()
	rows := renderRows([]listRow{commitItemRow(visible[0]), commitItemRow(visible[1])}, 120)
	// Titles must start at the same column in both rows.
	first := strings.Index(rows[0], "one")
	second := strings.Index(rows[1], "two")
	if first != second {
		t.Errorf("title columns differ: %d vs %d (%q / %q)", first, second, rows[0], rows[1])
	}
}

func TestCommitsView_CopyFromListUsesFullSHA(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.items = []gitlab.Commit{{
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
