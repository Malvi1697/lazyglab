package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

var enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}

func TestCommitsView_EnterOpensTheCommitDetail(t *testing.T) {
	// Enter drills into the commit, staying in this view — it must be useful even
	// for a commit no pipeline ever ran for.
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", Title: "first"}, {ShortID: "bbb2222", Title: "second"}}
	v.cursor = 1

	v.Update(enterKey)

	if !v.viewingCommit {
		t.Fatal("Enter should open the commit detail")
	}
	if v.detailCommit == nil || v.detailCommit.ShortID != "bbb2222" {
		t.Errorf("detail is for %+v, want the selected commit", v.detailCommit)
	}
}

func TestCommitsView_EscLeavesTheDetail(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)

	v.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v.viewingCommit {
		t.Error("Esc should return to the list")
	}
}

func TestCommitsView_DetailEnterGoesToPipelines(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey) // open the detail

	cmd := v.Update(enterKey) // and drill on
	if cmd == nil {
		t.Fatal("Enter in the detail should ask for the Pipelines view")
	}
	msg, ok := cmd().(ShowCommitPipelineMsg)
	if !ok {
		t.Fatalf("expected ShowCommitPipelineMsg, got %T", cmd())
	}
	if msg.ShortSHA != "aaa1111" {
		t.Errorf("ShortSHA = %q", msg.ShortSHA)
	}
}

func TestOverviewView_EnterAsksForTheCommit(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "ccc3333"}}

	cmd := v.Update(enterKey)
	if cmd == nil {
		t.Fatal("Enter on a commit should produce a command")
	}
	if msg := cmd().(ShowCommitMsg); msg.ShortSHA != "ccc3333" {
		t.Errorf("ShortSHA = %q, want ccc3333", msg.ShortSHA)
	}
}

func TestCommitsView_EnterWithNoCommitsDoesNothing(t *testing.T) {
	v := NewCommitsView(&Context{})
	if cmd := v.Update(enterKey); cmd != nil {
		t.Error("Enter with an empty list must do nothing")
	}
	if v.viewingCommit {
		t.Error("no commit to drill into, so no detail")
	}
}

func TestCommitDetail_SaysWhenNoPipelineRan(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 100, 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", Title: "no ci here"}}
	v.Update(enterKey)
	v.detailLoading = false

	out := v.commitDetailFull()
	if !strings.Contains(out, "No pipeline ran for this commit") {
		t.Errorf("detail = %q, want it to say no pipeline ran", out)
	}
	// And it must explain what p actually does, since GitLab builds refs.
	if !strings.Contains(out, "branch head") {
		t.Error("expected the detail to say a run targets the branch head")
	}
}

func TestCommitDetail_ListsPipelines(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 100, 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", ID: "aaa1111full"}}
	v.Update(enterKey)

	v.Update(CommitDetailLoadedMsg{
		SHA:    v.detailSHA,
		Commit: &gitlab.Commit{ShortID: "aaa1111", Title: "t", Message: "subject\n\nbody text"},
		Pipelines: []gitlab.Pipeline{
			{ID: 722331, Status: "failed", Ref: "develop"},
			{ID: 722100, Status: "success", Ref: "develop"},
		},
	})

	out := v.commitDetailFull()
	for _, want := range []string{"722331", "722100", "develop", "body text"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail is missing %q:\n%s", want, out)
		}
	}
}

func TestCommitDetail_IgnoresStaleReply(t *testing.T) {
	// Moving on before a slow reply arrives must not repaint the old commit.
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)

	v.Update(CommitDetailLoadedMsg{
		SHA:       "someothersha",
		Pipelines: []gitlab.Pipeline{{ID: 999, Status: "success"}},
	})
	if len(v.detailPipelines) != 0 {
		t.Error("a reply for a different commit must be ignored")
	}
}

func TestCommitDetail_RetryWithoutPipelineExplains(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)

	cmd := v.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if cmd == nil {
		t.Fatal("R should report that there is nothing to retry")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", cmd())
	}
	if !msg.IsErr || !strings.Contains(msg.Text, "No pipeline to retry") {
		t.Errorf("status = %+v", msg)
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

func TestCommitDetail_ShowsWarningsDistinctFromSuccess(t *testing.T) {
	// The whole point of asking GitLab per pipeline: a success whose allowed-to-fail
	// job failed must not look like a clean pass.
	v := NewCommitsView(&Context{})
	v.width, v.height = 100, 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", ID: "aaa1111full"}}
	v.Update(enterKey)

	v.Update(CommitDetailLoadedMsg{
		SHA:    v.detailSHA,
		Commit: &gitlab.Commit{ShortID: "aaa1111", Title: "t"},
		Pipelines: []gitlab.Pipeline{{
			ID: 722175, Status: "success", Ref: "main",
			StatusLabel: "passed with warnings", HasWarnings: true,
		}},
	})

	out := v.commitDetailFull()
	if !strings.Contains(out, "passed with warnings") {
		t.Errorf("detail must say it passed with warnings:\n%s", out)
	}
	// The plain green check is reserved for a clean pass.
	if strings.Contains(out, components.StatusIcon("success")) {
		t.Error("a warning pipeline must not render the success icon")
	}
	if !strings.Contains(out, components.StatusIcon(components.StatusWarning)) {
		t.Error("expected the warning icon")
	}
}

func TestCommitDetail_ShowsRefsParentAndMRs(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 100, 24
	v.commits = []gitlab.Commit{{ShortID: "aaa1111", ID: "aaa1111full"}}
	v.Update(enterKey)

	v.Update(CommitDetailLoadedMsg{
		SHA: v.detailSHA,
		Commit: &gitlab.Commit{
			ShortID: "aaa1111", Title: "fix: the thing", AuthorName: "Jan",
			Message:   "fix: the thing\n\nwhy it was broken",
			ParentIDs: []string{"4fb11974cafe0000000000000000000000000000"},
		},
		Refs: []gitlab.CommitRef{{Type: "branch", Name: "main"}, {Type: "tag", Name: "v1.2.0"}},
		MRs:  []gitlab.MergeRequest{{IID: 42, Title: "Add the thing"}},
	})

	out := v.commitDetailFull()
	for _, want := range []string{"why it was broken", "4fb11974", "main", "v1.2.0", "!42", "Jan"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail is missing %q:\n%s", want, out)
		}
	}
	// The subject must not be repeated in the body.
	if strings.Count(out, "fix: the thing") != 1 {
		t.Errorf("the subject should appear once, got %d:\n%s", strings.Count(out, "fix: the thing"), out)
	}
}

func TestCommitDetail_NoRefsIsStated(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 100, 20
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.Update(CommitDetailLoadedMsg{SHA: v.detailSHA, Commit: &gitlab.Commit{ShortID: "aaa1111", Title: "t"}})

	if out := v.commitDetailFull(); !strings.Contains(out, "no branches or tags") {
		t.Errorf("expected the empty refs case to be stated:\n%s", out)
	}
}
