package views

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestCommitStatus(t *testing.T) {
	pipelines := []gitlab.Pipeline{
		{SHA: "f138bae6729508b923de684d5a8e4f8a72eda3f2", Status: "success"},
	}

	if got := commitStatus("f138bae6", pipelines); got != "success" {
		t.Errorf(`commitStatus("f138bae6", pipelines) = %q, want "success"`, got)
	}
	if got := commitStatus("deadbeef", pipelines); got != "" {
		t.Errorf(`commitStatus("deadbeef", pipelines) = %q, want ""`, got)
	}
	if got := commitStatus("", pipelines); got != "" {
		t.Errorf(`commitStatus("", pipelines) = %q, want ""`, got)
	}
}

// TestOverviewView_Navigation covers the gap that made j/k feel broken: Overview
// is the default view, and it used to handle no keys at all.
func TestOverviewView_Navigation(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{
		{ShortID: "aaa1111", Title: "first"},
		{ShortID: "bbb2222", Title: "second"},
		{ShortID: "ccc3333", Title: "third"},
	}

	navDown := tea.KeyPressMsg{Code: 'j', Text: "j"}
	navUp := tea.KeyPressMsg{Code: 'k', Text: "k"}

	v.Update(navDown)
	if v.cursor != 1 {
		t.Errorf("after j, cursor = %d, want 1", v.cursor)
	}
	v.Update(navDown)
	v.Update(navDown) // past the end
	if v.cursor != 2 {
		t.Errorf("cursor = %d, want it clamped to 2", v.cursor)
	}
	v.Update(navUp)
	if v.cursor != 1 {
		t.Errorf("after k, cursor = %d, want 1", v.cursor)
	}

	v.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if v.cursor != 2 {
		t.Errorf("after G, cursor = %d, want the last commit", v.cursor)
	}
	v.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if v.cursor != 0 {
		t.Errorf("after g, cursor = %d, want 0", v.cursor)
	}

	// Arrows must work as well as the vim keys.
	v.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if v.cursor != 1 {
		t.Errorf("after Down, cursor = %d, want 1", v.cursor)
	}
}

func TestOverviewView_CursorClampedWhenCommitsShrink(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "a"}, {ShortID: "b"}, {ShortID: "c"}}
	v.cursor = 2

	// A refresh on a different branch can return fewer commits.
	v.Update(CommitsLoadedMsg{Commits: []gitlab.Commit{{ShortID: "a"}}})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped into the shorter list", v.cursor)
	}
}

func TestOverviewView_EmptyListNavigationIsSafe(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	v.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 with no commits", v.cursor)
	}
	if cmd := v.Update(tea.KeyPressMsg{Code: 'o', Text: "o"}); cmd != nil {
		t.Error("o with no commits must do nothing")
	}
}
