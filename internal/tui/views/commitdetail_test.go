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
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}, {ShortID: "38333fa4"}}
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
	v := NewOverviewView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}

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
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
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
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	page := strings.Join(v.detail.lines(80), "\n")
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
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	page := strings.Join(v.detail.lines(80), "\n")
	if !strings.Contains(page, components.StatusIcon(components.StatusWarning)) {
		t.Error("expected the warning icon for a pipeline that passed with warnings")
	}
}

func TestCommitDetail_NoPipelineIsExplained(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
	v.Update(enterKey)
	v.detail.sha = "aaa1111"
	v.Update(CommitDetailLoadedMsg{SHA: "aaa1111", Commit: &gitlab.Commit{ShortID: "aaa1111", Title: "t"}})

	page := strings.Join(v.detail.lines(80), "\n")
	if !strings.Contains(page, "No pipeline ran for this commit") {
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
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
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

func TestCommitDetail_EnterDrillsIntoTheJobsPanel(t *testing.T) {
	// The pipeline is driven from here, not by being sent to another tab.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))

	v.Update(enterKey)
	if !v.detail.jobs.active() {
		t.Fatal("Enter should open the jobs panel for the commit's pipeline")
	}
	if v.detail.jobs.pipelineID != 722175 {
		t.Errorf("panel is on pipeline %d, want 722175", v.detail.jobs.pipelineID)
	}
}

func TestCommitDetail_RetryWithoutPipelineExplains(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
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
	v.commits = []gitlab.Commit{{ShortID: "aaa1111"}}
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
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
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
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
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
	v.commits = []gitlab.Commit{
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
	v.Update(tea.KeyPressMsg{Code: 'L', Text: "L"})
	if v.detail.commit.ShortID != "bbb2222" {
		t.Errorf("after L, page is on %s", v.detail.commit.ShortID)
	}
	v.Update(tea.KeyPressMsg{Code: 'H', Text: "H"})
	if v.detail.commit.ShortID != "aaa1111" {
		t.Errorf("after H, page is on %s", v.detail.commit.ShortID)
	}
}

func TestCommitPage_ArrowsStopAtTheEnds(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "only", Title: "one"}}
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
	v.commits = []gitlab.Commit{{ShortID: "a"}, {ShortID: "b"}, {ShortID: "c"}}
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
	v.commits = []gitlab.Commit{{ShortID: "a", Title: "a commit title here"}}
	v.Update(enterKey)

	body := plain(v.Body(24, 12))
	if strings.Contains(body, "‹") {
		t.Error("a narrow terminal should keep the text and drop the arrows")
	}
}
