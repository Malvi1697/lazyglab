package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

var (
	enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}
	escKey   = tea.KeyPressMsg{Code: tea.KeyEscape}
)

// loadedDetail is a fully populated reply, as the API layer would produce.
func loadedDetail(sha string) CommitDetailLoadedMsg {
	return CommitDetailLoadedMsg{
		SHA: sha,
		Commit: &gitlab.Commit{
			ID: "38333fa410c18de0837225e792b28c29c33ee4fb", ShortID: "38333fa4",
			Title:      "fix(#0): count only free wild cards",
			Message:    "fix(#0): count only free wild cards\n\nThe header double-counted slots.",
			AuthorName: "Jan Všetíček",
			ParentIDs:  []string{"4fb11974cafe0000000000000000000000000000"},
			WebURL:     "https://gitlab.example.com/g/p/-/commit/38333fa4",
		},
		Pipelines: []gitlab.Pipeline{{
			ID: 722175, Status: "success", Ref: "main",
			StatusLabel: "passed with warnings", HasWarnings: true,
		}},
		Refs: []gitlab.CommitRef{{Type: "branch", Name: "main"}, {Type: "tag", Name: "v1.2.0"}},
		MRs:  []gitlab.MergeRequest{{IID: 42, Title: "Add the thing"}},
		Jobs: []gitlab.Job{
			{ID: 1, Name: "build", Stage: "build", Status: "success", Duration: 82},
			{ID: 2, Name: "lint", Stage: "test", Status: "failed", Duration: 12},
			{ID: 3, Name: "deploy", Stage: "deploy", Status: "manual"},
		},
	}
}

func TestCommitsView_EnterOpensDetailInPlace(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}, {ShortID: "38333fa4"}}
	v.cursor = 1

	if cmd := v.Update(enterKey); cmd != nil {
		cmd() // no client configured, so this is a no-op
	}
	if !v.detail.active {
		t.Fatal("Enter should open the commit detail")
	}
	if v.detail.commit == nil || v.detail.commit.ShortID != "38333fa4" {
		t.Errorf("detail is for %+v, want the selected commit", v.detail.commit)
	}

	// The detail takes the whole body rather than the narrow right pane.
	body := plain(v.Body(120, 30))
	if strings.Contains(body, "Commits") {
		t.Error("the list should be replaced by the detail, not sit beside it")
	}
	if !strings.Contains(body, "Commit 38333fa4") {
		t.Errorf("expected the detail box for the commit, got:\n%s", body)
	}
}

func TestOverviewView_EnterOpensDetailInPlace(t *testing.T) {
	// Drilling in must not move the user to another tab.
	v := NewDashboardView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}

	v.Update(enterKey)
	if !v.detail.active {
		t.Fatal("Enter should open the commit detail in Overview itself")
	}
	if strings.Contains(plain(v.Body(120, 30)), "Recent Commits") {
		t.Error("the detail should replace the dashboard body")
	}
}

func TestCommitDetail_EscGoesBack(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)

	v.Update(escKey)
	if v.detail.active {
		t.Error("Esc should return to the list")
	}
	if !strings.Contains(plain(v.Body(120, 30)), "Commits") {
		t.Error("the list should be back")
	}
}

func TestCommitDetail_ShowsWhatGitLabsCommitPageShows(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 40
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	page := plain(v.Body(120, 40))
	for _, want := range []string{
		"fix(#0): count only free wild cards", // subject
		"The header double-counted slots",     // body
		"Jan Všetíček",                        // author
		"4fb11974",                            // parent
		"main",                                // branch
		"v1.2.0",                              // tag
		"!42",                                 // merge request
		"722175",                              // pipeline
		"passed with warnings",                // its detailed status
		"build", "lint", "deploy",             // jobs
		"1m22s", // job duration
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page is missing %q:\n%s", want, page)
		}
	}

	// The subject must not be repeated as part of the body.
	if n := strings.Count(page, "fix(#0): count only free wild cards"); n != 1 {
		t.Errorf("subject appears %d times, want once", n)
	}
}

func TestCommitDetail_WarningIsNotAPlainSuccess(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 40
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	// Compare the glyph, not the styled string: the assertion reads the text a
	// user sees, with the escapes stripped.
	page := plain(v.Body(120, 40))
	if !strings.Contains(page, plain(components.StatusIcon(components.StatusWarning))) {
		t.Errorf("expected the warning icon for a pipeline that passed with warnings:\n%s", page)
	}
	if !strings.Contains(page, "passed with warnings") {
		t.Error("expected GitLab's own wording for the status")
	}
}

func TestCommitDetail_NoPipelineIsExplained(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.detail.sha = "aaa1111"
	v.Update(CommitDetailLoadedMsg{SHA: "aaa1111", Commit: &gitlab.Commit{ShortID: "aaa1111", Title: "t"}})

	page := plain(v.Body(120, 40))
	if !strings.Contains(page, "no pipeline") {
		t.Errorf("expected it to say no pipeline ran:\n%s", page)
	}
	// And what p would actually do, since GitLab only builds refs.
	if !strings.Contains(page, "branch head") {
		t.Error("expected the branch-head caveat")
	}
	if !strings.Contains(page, "no branches or tags contain it") {
		t.Error("expected the empty refs case to be stated")
	}
}

func TestCommitDetail_EnterWithoutPipelineExplains(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.detail.sha = "aaa1111"
	v.Update(CommitDetailLoadedMsg{SHA: "aaa1111", Commit: &gitlab.Commit{ShortID: "aaa1111"}})

	cmd := v.Update(enterKey)
	if cmd == nil {
		t.Fatal("Enter should report that there is no pipeline to open")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", cmd())
	}
	if !msg.IsErr || !strings.Contains(msg.Text, "No pipeline") {
		t.Errorf("status = %+v", msg)
	}
}

func TestCommitDetail_EnterStepsIntoTheJobsOnThePage(t *testing.T) {
	// The jobs are already rendered on the page, so Enter moves the focus into
	// them rather than replacing the page with a panel.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	before := plain(v.Body(120, 30))

	v.Update(enterKey)
	if v.detail.focus != focusJobs {
		t.Fatal("Enter should move the focus into the jobs")
	}
	if v.detail.jobs.pipelineID != 722175 {
		t.Errorf("panel is on pipeline %d, want 722175", v.detail.jobs.pipelineID)
	}

	// The page is still the page: the commit message has not been swapped away.
	after := plain(v.Body(120, 30))
	for _, want := range []string{"38333fa4", "build", "deploy"} {
		if !strings.Contains(after, want) {
			t.Errorf("stepping in lost %q from the page", want)
		}
	}
	if before == after {
		t.Error("stepping in should show a cursor on a job")
	}
}

func TestCommitDetail_LogTakesTheWholeBody(t *testing.T) {
	// A log needs the room, so it is the one thing that replaces the page.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))
	v.Update(enterKey) // focus the jobs
	v.Update(JobTraceLoadedMsg{Trace: "line one\nline two\nline three"})

	body := plain(v.Body(120, 20))
	if !strings.Contains(body, "line two") {
		t.Errorf("expected the log:\n%s", body)
	}
	if strings.Contains(body, "The header double-counted") {
		t.Error("the log should take the body, not share it with the message")
	}
	if strings.Contains(body, "‹") {
		t.Error("the step arrows belong to the page, not to a log")
	}
}

func TestCommitDetail_RetryWithoutPipelineExplains(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)

	cmd := v.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if cmd == nil {
		t.Fatal("R should report there is nothing to retry")
	}
	if msg := cmd().(StatusMsg); !msg.IsErr || !strings.Contains(msg.Text, "No pipeline to retry") {
		t.Errorf("status = %+v", msg)
	}
}

func TestCommitDetail_IgnoresStaleReply(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.detail.sha = "aaa1111"

	v.Update(loadedDetail("someothersha"))
	if len(v.detail.pipelines) != 0 {
		t.Error("a reply for a commit we moved off must be ignored")
	}
}

func TestCommitDetail_LongPageScrolls(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 12 // a short terminal
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"

	msg := loadedDetail("38333fa4")
	msg.Commit.Message = "subject\n\n" + strings.Repeat("a long explanation line\n", 30)
	v.Update(msg)

	before := v.detail.scroll
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	v.Body(120, 12)
	if v.detail.scroll == before {
		t.Error("j should scroll a page that does not fit")
	}

	// The box says where you are.
	if !strings.Contains(plain(v.Body(120, 12)), " of ") {
		t.Error("expected a position indicator for a scrolled page")
	}
}

func TestCommitDetail_CopyUsesFullSHA(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	cmd := v.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("y should copy the SHA")
	}
	if !batchMentions(cmd, "38333fa4") {
		t.Error("expected the copy to be confirmed in the status bar")
	}
}

// plain strips styling so assertions read the text a user sees, not the escapes
// woven through it.
func plain(s string) string { return ansi.Strip(s) }

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

func TestCommitPage_ArrowsStepBetweenCommits(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{
		{ShortID: "aaa1111", Title: "first"},
		{ShortID: "bbb2222", Title: "second"},
		{ShortID: "ccc3333", Title: "third"},
	}
	v.Update(enterKey) // page for the first commit

	if v.detail.index != 0 || v.detail.total != 3 {
		t.Fatalf("position = %d/%d, want 0/3", v.detail.index, v.detail.total)
	}
	if v.detail.hasPrev() {
		t.Error("the first commit has no previous")
	}

	right := tea.KeyPressMsg{Code: tea.KeyRight}
	v.Update(right)
	if v.detail.commit.ShortID != "bbb2222" {
		t.Errorf("after →, page is on %s, want bbb2222", v.detail.commit.ShortID)
	}
	if !v.detail.active {
		t.Error("stepping must keep the page open")
	}
	if v.cursor != 1 {
		t.Errorf("the list cursor should follow, got %d", v.cursor)
	}

	left := tea.KeyPressMsg{Code: tea.KeyLeft}
	v.Update(left)
	if v.detail.commit.ShortID != "aaa1111" {
		t.Errorf("after ←, page is on %s, want aaa1111", v.detail.commit.ShortID)
	}

	// H and L do the same, for hands that stay on the home row.
	v.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if v.detail.commit.ShortID != "bbb2222" {
		t.Errorf("after L, page is on %s", v.detail.commit.ShortID)
	}
	v.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if v.detail.commit.ShortID != "aaa1111" {
		t.Errorf("after H, page is on %s", v.detail.commit.ShortID)
	}
}

func TestCommitPage_ArrowsStopAtTheEnds(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "only", Title: "one"}}
	v.Update(enterKey)

	v.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	v.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if v.detail.commit.ShortID != "only" || !v.detail.active {
		t.Error("a single commit must not be stepped off")
	}
	if v.detail.hasPrev() || v.detail.hasNext() {
		t.Error("neither neighbour exists for a single commit")
	}
}

func TestCommitPage_ArrowsAreDrawnInTheMargins(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "a"}, {ShortID: "b"}, {ShortID: "c"}}
	v.cursor = 1
	v.Update(enterKey)

	body := plain(v.Body(120, 20))
	if !strings.Contains(body, "‹") || !strings.Contains(body, "›") {
		t.Fatalf("expected step arrows in the margins:\n%s", body)
	}
	// They belong on one row, level with the middle of the page.
	rows := strings.Split(body, "\n")
	arrowRows := 0
	for _, r := range rows {
		if strings.Contains(r, "‹") {
			arrowRows++
		}
	}
	if arrowRows != 1 {
		t.Errorf("arrows appear on %d rows, want 1", arrowRows)
	}
	// And the page says where you are.
	if !strings.Contains(body, "2/3") {
		t.Errorf("expected the position in the heading:\n%s", rows[0])
	}
}

func TestCommitPage_NarrowTerminalDropsTheMargins(t *testing.T) {
	// Squeezing the text to keep decoration would be the wrong trade.
	v := NewCommitsView(&Context{})
	v.width, v.height = 24, 12
	v.items = []gitlab.Commit{{ShortID: "a", Title: "a commit title here"}}
	v.Update(enterKey)

	body := plain(v.Body(24, 12))
	if strings.Contains(body, "‹") {
		t.Error("a narrow terminal should keep the text and drop the arrows")
	}
}

func TestCommitDetail_JobsStayVisibleBesideALongMessage(t *testing.T) {
	// The jobs have a column of their own, so a long message cannot push them off
	// the screen and stepping into them needs no scrolling.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 24
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"

	msg := loadedDetail("38333fa4")
	msg.Commit.Message = "subject\n\n" + strings.Repeat("a long explanation line\n", 40)
	v.Update(msg)

	body := plain(v.Body(120, 24))
	for _, want := range []string{"a long explanation line", "Jobs (3)", "build"} {
		if !strings.Contains(body, want) {
			t.Errorf("the body is missing %q:\n%s", want, body)
		}
	}

	// This reply carries no diffs, so Enter steps straight into the jobs.
	v.Update(enterKey)
	if v.detail.focus != focusJobs {
		t.Fatal("expected the jobs to take the focus")
	}
	if !strings.Contains(plain(v.Body(120, 24)), "build") {
		t.Error("the jobs must still be on screen once focused")
	}
}

func TestCommitDetail_JobCursorMovesWithinThePage(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))
	v.Update(enterKey)

	if v.detail.jobs.cursor != 0 {
		t.Fatalf("cursor = %d, want the first job", v.detail.jobs.cursor)
	}
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if v.detail.jobs.cursor != 1 {
		t.Errorf("j should move to the next job, got %d", v.detail.jobs.cursor)
	}
	// The page did not scroll away from itself: the commit is still on screen.
	if !strings.Contains(plain(v.Body(120, 30)), "38333fa4") {
		t.Error("moving between jobs should not leave the commit page")
	}
}

func TestCommitDetail_StepInWithoutPipelineExplains(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.detail.sha = "aaa1111"
	v.Update(CommitDetailLoadedMsg{SHA: "aaa1111", Commit: &gitlab.Commit{ShortID: "aaa1111"}})

	cmd := v.Update(enterKey)
	if cmd == nil {
		t.Fatal("Enter should report there is nothing to step into")
	}
	if msg := cmd().(StatusMsg); !msg.IsErr || !strings.Contains(msg.Text, "No pipeline") {
		t.Errorf("status = %+v", msg)
	}
	if v.detail.focus != focusPage {
		t.Error("focus must stay on the page when there are no jobs")
	}
}

// diffedDetail is a reply for the sample commit that also carries changed files.
func diffedDetail() CommitDetailLoadedMsg {
	msg := loadedDetail("38333fa4")
	msg.Diffs = []gitlab.FileDiff{
		{
			NewPath: "internal/app/config.go", OldPath: "internal/app/config.go",
			Diff:  "@@ -1,3 +1,4 @@\n context line\n-removed line\n+added line\n+another added\n",
			Added: 2, Removed: 1,
		},
		{NewPath: "docs/new.md", New: true, Diff: "@@ -0,0 +1 @@\n+hello\n", Added: 1},
		{OldPath: "old.go", Deleted: true, Diff: "@@ -1 +0,0 @@\n-gone\n", Removed: 1},
		{NewPath: "huge.bin", New: true, Withheld: true},
	}
	return msg
}

func TestCommitPage_ListsChangedFiles(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 40
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())

	page := plain(v.Body(120, 40))
	for _, want := range []string{
		"Changes (4)",
		"internal/app/config.go", "+2", "-1",
		"docs/new.md",
		"old.go",
		"too large to show", // the withheld one says so rather than looking empty
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page is missing %q:\n%s", want, page)
		}
	}
}

func TestCommitPage_EnterStepsIntoTheChangesFirst(t *testing.T) {
	// A commit is its diff, so the changes are the first thing Enter reaches.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 40
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())

	v.Update(enterKey)
	if v.detail.focus != focusFiles {
		t.Fatalf("focus = %v, want the changed files", v.detail.focus)
	}

	// Tab moves on to the jobs; the full cycle is covered by its own test.
	v.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if v.detail.focus != focusJobs {
		t.Errorf("Tab should move the focus to the jobs, got %v", v.detail.focus)
	}
}

func TestCommitPage_ReadsADiff(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())
	v.Update(enterKey) // focus the files

	v.Update(enterKey) // read the highlighted one
	if !v.detail.reading {
		t.Fatal("Enter on a file should open its diff")
	}

	body := plain(v.Body(120, 20))
	for _, want := range []string{"internal/app/config.go", "@@ -1,3 +1,4 @@", "+added line", "-removed line"} {
		if !strings.Contains(body, want) {
			t.Errorf("the diff is missing %q:\n%s", want, body)
		}
	}
	// The diff owns the body while it is being read.
	if strings.Contains(body, "The header double-counted") {
		t.Error("the commit message should not share the body with the diff")
	}

	v.Update(escKey)
	if v.detail.reading {
		t.Error("Esc should close the diff")
	}
	if v.detail.focus != focusFiles {
		t.Error("closing a diff should leave the focus on the files")
	}
}

func TestCommitPage_DiffLinesAreColoured(t *testing.T) {
	// Colour is what makes a diff readable at a glance; check the raw output.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())
	v.Update(enterKey)
	v.Update(enterKey)

	raw := v.detail.diffView(100, 20)
	added := strings.Split(raw, "\n")
	var plusStyled, minusStyled bool
	for _, l := range added {
		switch {
		case strings.Contains(plain(l), "+added line"):
			plusStyled = l != plain(l)
		case strings.Contains(plain(l), "-removed line"):
			minusStyled = l != plain(l)
		}
	}
	if !plusStyled || !minusStyled {
		t.Error("added and removed lines must be coloured")
	}
}

func TestCommitPage_WithheldDiffSaysSo(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())
	v.Update(enterKey)
	v.detail.fileCursor = 3 // the withheld one
	v.Update(enterKey)

	if got := plain(v.detail.diffView(100, 20)); !strings.Contains(got, "too large") {
		t.Errorf("expected an explanation, got %q", got)
	}
}

func TestCommitPage_NoDiffFallsBackToJobs(t *testing.T) {
	// A commit whose diff GitLab would not send still has to be steppable.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4")) // jobs but no diffs

	v.Update(enterKey)
	if v.detail.focus != focusJobs {
		t.Errorf("focus = %v, want the jobs when there are no changes", v.detail.focus)
	}
}

func TestCommitPage_TabCyclesTheBoxesNotTheViews(t *testing.T) {
	// Tab used to be taken by the shell for switching views, so it never reached
	// the page it was supposed to move around in.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())

	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	shiftTab := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}

	// message -> changes -> jobs -> message
	if v.detail.focus != focusPage {
		t.Fatalf("focus = %v, want the message to start with it", v.detail.focus)
	}
	v.Update(tab)
	if v.detail.focus != focusFiles {
		t.Fatalf("after Tab, focus = %v, want the changes", v.detail.focus)
	}
	v.Update(tab)
	if v.detail.focus != focusJobs {
		t.Fatalf("after a second Tab, focus = %v, want the jobs", v.detail.focus)
	}
	v.Update(tab)
	if v.detail.focus != focusPage {
		t.Fatalf("Tab should wrap back to the message, got %v", v.detail.focus)
	}

	// And backwards.
	v.Update(shiftTab)
	if v.detail.focus != focusJobs {
		t.Errorf("Shift+Tab should go back to the jobs, got %v", v.detail.focus)
	}
}

func TestCommitPage_TabSkipsEmptyBoxes(t *testing.T) {
	// A commit with no changes reported must not have a Tab stop on an empty box.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4")) // jobs, no diffs

	tab := tea.KeyPressMsg{Code: tea.KeyTab}
	v.Update(tab)
	if v.detail.focus != focusJobs {
		t.Errorf("focus = %v, want the jobs (the changes box is empty)", v.detail.focus)
	}
	v.Update(tab)
	if v.detail.focus != focusPage {
		t.Errorf("focus = %v, want it back on the message", v.detail.focus)
	}
}

func TestCommitPage_ArrowsStepFilesWhileReadingADiff(t *testing.T) {
	// Inside a diff the arrows belong to the files: stepping commits would swap the
	// file under you for one from another commit.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}, {ShortID: "bbb2222"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())
	v.Update(enterKey) // focus the changes
	v.Update(enterKey) // read the first file

	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyRight},
		{Code: 'l', Text: "l"},
	} {
		before := v.detail.fileCursor
		v.Update(key)
		if v.detail.fileCursor != before+1 {
			t.Fatalf("%v should step to the next file, cursor %d -> %d", key, before, v.detail.fileCursor)
		}
		if !v.detail.reading {
			t.Fatal("stepping files must keep the diff open")
		}
		if v.detail.commit.ShortID != "38333fa4" {
			t.Fatal("the commit must not change while reading a diff")
		}
	}

	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyLeft},
		{Code: 'h', Text: "h"},
	} {
		before := v.detail.fileCursor
		v.Update(key)
		if v.detail.fileCursor != before-1 {
			t.Fatalf("%v should step to the previous file, cursor %d -> %d", key, before, v.detail.fileCursor)
		}
	}

	// The title says which file of how many, as the page does for commits.
	if body := plain(v.Body(120, 20)); !strings.Contains(body, "1/4") {
		t.Errorf("expected the file's position in the title:\n%s", strings.Split(body, "\n")[0])
	}
}

func TestCommitPage_ArrowsStepCommitsFromThePageNotFiles(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 20
	v.items = []gitlab.Commit{{ShortID: "38333fa4"}, {ShortID: "bbb2222"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(diffedDetail())

	v.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if v.detail.commit.ShortID != "bbb2222" {
		t.Errorf("from the page the arrows step commits, got %s", v.detail.commit.ShortID)
	}
}

// renderRows lays out a list of rows at one width and renders them, the way a view
// does: measure over the whole list, then render each row to those columns.
func renderRows(rows []listRow, width int) []string {
	cols := measureColumns(rows, width)
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = renderListRow(r, cols, width)
	}
	return out
}

// renderRow is renderRows for a single row.
func renderRow(r listRow, width int) string { return renderRows([]listRow{r}, width)[0] }
