package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// copied runs a copy command and returns the status line it reports, which is
// the only visible evidence that anything reached the clipboard.
func copied(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a copy command")
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if s, ok := c().(StatusMsg); ok {
				return s.Text
			}
		}
		t.Fatal("the batch said nothing about what was copied")
	case StatusMsg:
		return msg.Text
	}
	t.Fatalf("unexpected message %T", cmd())
	return ""
}

func press(v View, key rune) tea.Cmd {
	return v.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
}

func TestCopy_MergeRequestRefAndLink(t *testing.T) {
	v := NewMRsView(&Context{})
	v.height = 20
	v.mrs = []gitlab.MergeRequest{{IID: 42, Title: "Fix the cart", WebURL: "https://gl/x/-/merge_requests/42"}}

	if got := copied(t, press(v, 'y')); !strings.Contains(got, "!42") {
		t.Errorf("y reported %q, want the reference you would type", got)
	}
	got := copied(t, press(v, 'Y'))
	if !strings.Contains(got, "link") || !strings.Contains(got, "!42") {
		t.Errorf("Y reported %q, want it to say it copied the link to !42", got)
	}
}

func TestCopy_IssueAndPipelineFollowTheSameRule(t *testing.T) {
	issues := NewIssuesView(&Context{})
	issues.height = 20
	issues.issues = []gitlab.Issue{{IID: 7, Title: "Crash", WebURL: "https://gl/x/-/issues/7"}}
	if got := copied(t, press(issues, 'y')); !strings.Contains(got, "#7") {
		t.Errorf("issue y reported %q, want #7", got)
	}

	pipelines := NewPipelinesView(&Context{})
	pipelines.height = 20
	pipelines.pipelines = []gitlab.Pipeline{{ID: 1234, Status: "success", WebURL: "https://gl/x/-/pipelines/1234"}}
	if got := copied(t, press(pipelines, 'y')); !strings.Contains(got, "#1234") {
		t.Errorf("pipeline y reported %q, want #1234", got)
	}
	if got := copied(t, press(pipelines, 'Y')); !strings.Contains(got, "link") {
		t.Errorf("pipeline Y reported %q, want the link", got)
	}
}

func TestCopy_TodoUsesItsTargetsReference(t *testing.T) {
	// The to-do's own id means nothing to anyone else, so y copies "!5".
	v := todosView()
	if got := copied(t, press(v, 'y')); !strings.Contains(got, "!42") {
		t.Errorf("todo y reported %q, want the target's reference", got)
	}
}

func TestCopy_SaysSoWhenThereIsNothingToCopy(t *testing.T) {
	// A to-do on a commit has no reference GitLab would write. Silence would look
	// like a successful copy.
	v := todosView()
	v.cursor = 2 // the build_failed commit todo
	got := copied(t, press(v, 'y'))
	if !strings.Contains(strings.ToLower(got), "nothing to copy") {
		t.Errorf("reported %q, want it to say there was nothing to copy", got)
	}
}

func TestCopy_CommitPageCopiesTheCommitEvenInADiff(t *testing.T) {
	d := legendPage(t)
	d.focus = focusFiles
	d.reading = true

	if got := copied(t, d.handleKey("y", 20)); !strings.Contains(got, "abc1234") {
		t.Errorf("y in a diff reported %q, want the commit SHA", got)
	}
	if got := copied(t, d.handleKey("Y", 20)); !strings.Contains(got, "link") {
		t.Errorf("Y in a diff reported %q, want the commit link", got)
	}
}

func TestCopy_JobsBoxCopiesTheJobNotTheCommit(t *testing.T) {
	// The keys act on the box that has focus, which is what its footer promises.
	d := legendPage(t)
	d.focus = focusJobs

	got := copied(t, d.handleKey("y", 20))
	if !strings.Contains(got, "build") {
		t.Errorf("y in the jobs box reported %q, want the job's name", got)
	}
	if strings.Contains(got, "abc1234") {
		t.Errorf("y in the jobs box reported %q, want the job rather than the commit", got)
	}
}
