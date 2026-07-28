package views

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// A failed load has to reach the status bar. Every view used to store the message
// in a field of its own that nothing ever rendered, so a 500, a timeout or a
// broken connection left the screen looking merely empty.
func TestLoadErrors_ReachTheStatusBar(t *testing.T) {
	boom := errors.New("connection refused")
	ctx := &Context{}

	for _, tc := range []struct {
		name string
		view View
		msg  tea.Msg
		want string
	}{
		{"pipelines", NewPipelinesView(ctx), PipelinesLoadedMsg{Err: boom}, "pipelines"},
		{"jobs", NewPipelinesView(ctx), JobsLoadedMsg{Err: boom}, "jobs"},
		{"job log", NewPipelinesView(ctx), JobTraceLoadedMsg{Err: boom}, "log"},
		{"merge requests", NewMRsView(ctx), MRsLoadedMsg{Err: boom}, "merge requests"},
		{"issues", NewIssuesView(ctx), IssuesLoadedMsg{Err: boom}, "issues"},
		{"todos", NewTodosView(ctx), TodosLoadedMsg{Err: boom}, "todos"},
		{"commits", NewCommitsView(ctx), CommitsLoadedMsg{Err: boom}, "commits"},
	} {
		cmd := tc.view.Update(tc.msg)
		if cmd == nil {
			t.Errorf("%s: a failed load reported nothing", tc.name)
			continue
		}
		msg, ok := cmd().(StatusMsg)
		if !ok {
			t.Errorf("%s: reported %T, want a StatusMsg", tc.name, cmd())
			continue
		}
		if !msg.IsErr {
			t.Errorf("%s: reported %q without marking it an error", tc.name, msg.Text)
		}
		if !strings.Contains(msg.Text, tc.want) || !strings.Contains(msg.Text, "connection refused") {
			t.Errorf("%s: reported %q, want it to name what failed and why", tc.name, msg.Text)
		}
	}
}

func TestLoadErrors_TheCommitPageIsNotSilentEither(t *testing.T) {
	// The page returned its message as a string, and Overview — the default view —
	// dropped it on the floor, so nothing on the commit page could report anything.
	v := NewDashboardView(&Context{})
	v.commits = []gitlab.Commit{{ID: "abc1234", ShortID: "abc1234", Title: "one"}}
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // open the page

	cmd := v.Update(CommitDetailLoadedMsg{SHA: "abc1234", Err: errors.New("boom")})
	if cmd == nil {
		t.Fatal("a failed commit load reported nothing")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok || !msg.IsErr || !strings.Contains(msg.Text, "boom") {
		t.Errorf("reported %v, want the failure in the status bar", cmd())
	}
}
