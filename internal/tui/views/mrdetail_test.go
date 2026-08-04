package views

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func mrsWithPage(t *testing.T) *MRsView {
	t.Helper()
	v := NewMRsView(&Context{})
	v.width, v.height = 160, 45
	v.items = []gitlab.MergeRequest{
		{IID: 42, Title: "Fix the cart", Author: "alice.novak", SourceBranch: "fix-cart",
			TargetBranch: "main", State: "opened", WebURL: "https://gl/x/-/merge_requests/42"},
		{IID: 12, Title: "Test mr", Author: "bob", SourceBranch: "test",
			TargetBranch: "main", State: "opened", WebURL: "https://gl/x/-/merge_requests/12"},
	}
	return v
}

// loadedPage fills the page for !42 the way a reply would, without HTTP.
func loadedPage(v *MRsView) {
	const iid = 42
	v.detail.update(MRDetailLoadedMsg{
		IID: iid,
		MR: &gitlab.MergeRequest{
			IID: iid, Title: "Fix the cart", Author: "alice.novak",
			SourceBranch: "fix-cart", TargetBranch: "main", State: "opened",
			Description: "Why the cart was wrong.\n\nAnd how this fixes it.",
			MergeStatus: "ci_still_running", Labels: []string{"bug"},
			Reviewers: []string{"carol"}, WebURL: fmt.Sprintf("https://gl/x/-/merge_requests/%d", iid),
		},
		Approvals: &gitlab.MRApprovals{Required: 2, Left: 1, ApprovedBy: []string{"dave"}},
		Pipeline:  &gitlab.Pipeline{ID: 77, Status: "running"},
		Jobs:      []gitlab.Job{{ID: 1, Name: "build", Stage: "build", Status: "running"}},
		Diffs: []gitlab.FileDiff{
			{NewPath: "cart.py", Diff: "@@ -1 +1 @@\n-x\n+y\n", Added: 1, Removed: 1},
			{NewPath: "cart_test.py", Diff: "@@ -1 +1 @@\n+z\n", Added: 1},
		},
	})
}

func TestMRPage_EnterOpensItInPlace(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if !v.detail.active {
		t.Fatal("Enter should open the merge-request page")
	}
	if v.detail.iid != 42 {
		t.Errorf("page opened on !%d, want !42", v.detail.iid)
	}
	// The page replaces the body; the list is what Esc returns to.
	v.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v.detail.active {
		t.Error("Esc should return to the list")
	}
}

func TestMRPage_SaysWhatStandsBetweenItAndBeingMerged(t *testing.T) {
	// The whole point of the page: branch, CI, mergeability, approvals — before the
	// description, which can be long.
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)

	body := plain(v.detail.body(160, 45))
	for _, want := range []string{
		"fix-cart", "main", "alice", "carol",
		"#77 running",      // its pipeline
		"ci still running", // GitLab's own merge status, made readable
		"1 still needed",   // approvals left
		"dave",             // who has approved
		"bug",              // labels
		"Changes (2)",      // the files it touches
		"Jobs (1)",         // and its pipeline's jobs
		"Why the cart was wrong.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not mention %q:\n%s", want, body)
		}
	}
}

func TestMRPage_ConflictsAreSaidInTheirOwnRight(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v.detail.update(MRDetailLoadedMsg{IID: 42, MR: &gitlab.MergeRequest{
		IID: 42, Title: "t", TargetBranch: "main", HasConflicts: true,
		MergeStatus: "broken_status",
	}})

	if body := plain(v.detail.body(160, 45)); !strings.Contains(body, "conflicts with main") {
		t.Errorf("want the conflict named:\n%s", body)
	}
	// And merging is refused before a request is sent.
	cmd := v.detail.handleKey("m", 45)
	if cmd == nil {
		t.Fatal("m should report why it cannot merge")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok || !msg.IsErr || !strings.Contains(msg.Text, "conflicts") {
		t.Errorf("m reported %v, want the conflict as an error", cmd())
	}
}

func TestMRPage_StepsBetweenMergeRequests(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)

	v.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if v.detail.iid != 12 {
		t.Errorf("l landed on !%d, want !12", v.detail.iid)
	}
	if v.detail.total != 2 || v.detail.index != 1 {
		t.Errorf("page says %d/%d, want 2/2", v.detail.index+1, v.detail.total)
	}

	v.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if v.detail.iid != 42 {
		t.Errorf("h landed on !%d, want !42 again", v.detail.iid)
	}
}

func TestMRPage_TabCyclesTheBoxesAndEnterStepsIn(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)

	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // into the changed files
	if v.detail.focus != focusFiles {
		t.Fatalf("focus = %v, want the changed files", v.detail.focus)
	}
	v.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if v.detail.focus != focusJobs {
		t.Errorf("focus = %v, want the jobs", v.detail.focus)
	}

	// Reading a diff, and stepping files rather than merge requests.
	v.detail.focus = focusFiles
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !v.detail.reading {
		t.Fatal("Enter on a file should read its diff")
	}
	v.Update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if v.detail.fileCursor != 1 {
		t.Errorf("fileCursor = %d, want l to step files inside a diff", v.detail.fileCursor)
	}
	if v.detail.iid != 42 {
		t.Error("l inside a diff must not step to another merge request")
	}
	if body := plain(v.detail.body(160, 45)); !strings.Contains(body, "cart_test.py  2/2") {
		t.Errorf("the reader should name the file and its place:\n%s", body)
	}
}

func TestMRPage_ApprovingTwiceIsRefusedLocally(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v.detail.update(MRDetailLoadedMsg{IID: 42,
		MR:        &gitlab.MergeRequest{IID: 42, Title: "t", TargetBranch: "main"},
		Approvals: &gitlab.MRApprovals{HasApproved: true, Approved: true, ApprovedBy: []string{"me"}},
	})

	cmd := v.detail.handleKey("a", 45)
	if cmd == nil {
		t.Fatal("a should say something")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Text, "already approved") {
		t.Errorf("a reported %v, want it to say the approval is already given", cmd())
	}
}

func TestMRPage_ApprovingAsksFirstAndRefetchesAfter(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)
	v.detail.ctx = &Context{Client: &gitlab.Client{}, Project: &gitlab.Project{ID: 1}}

	cmd := v.detail.handleKey("a", 45)
	if cmd == nil {
		t.Fatal("a should ask to approve")
	}
	confirm, ok := cmd().(ConfirmMsg)
	if !ok || !strings.Contains(confirm.Prompt, "!42") {
		t.Fatalf("expected a confirmation naming !42, got %v", cmd())
	}

	// Once done, the page is refetched: approving changes what it says about itself.
	after := v.detail.update(MRActionDoneMsg{Text: "Approved !42"})
	if after == nil {
		t.Fatal("approving should reload the page and report it")
	}
	if _, cached := v.detail.pages[42]; cached {
		t.Error("the cached page should have been dropped")
	}
}

func TestMRPage_KeyHintsFollowTheFocusedBox(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)
	v.detail.body(160, 45) // hints depend on what was rendered

	page := hintsFor(v.detail.keyHints())
	for _, want := range []string{"h/l", "Approve", "Merge", "Tab"} {
		if !strings.Contains(page, want) {
			t.Errorf("page hints = %q, want %q", page, want)
		}
	}

	v.detail.focus = focusFiles
	if got := hintsFor(v.detail.keyHints()); !strings.Contains(got, "Read diff") {
		t.Errorf("files hints = %q, want the reader offered", got)
	}
}

func TestMRPage_NoPipelineSaysSoRatherThanNothing(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v.detail.update(MRDetailLoadedMsg{IID: 42,
		MR: &gitlab.MergeRequest{IID: 42, Title: "t", TargetBranch: "main"}})

	if body := plain(v.detail.body(160, 45)); !strings.Contains(body, "no pipeline") {
		t.Errorf("want the absent pipeline said plainly:\n%s", body)
	}
	// And stepping into the jobs box explains itself instead of doing nothing.
	cmd := v.detail.focusJobs()
	if cmd == nil {
		t.Fatal("focusing absent jobs should report why")
	}
	if msg, ok := cmd().(StatusMsg); !ok || !strings.Contains(msg.Text, "No pipeline") {
		t.Errorf("reported %v, want the missing pipeline explained", cmd())
	}
}

func TestMRList_ColumnsSayWhoFromWhereAndWhen(t *testing.T) {
	// The reference is GitLab's own list: the number, what it is, and then who, from which
	// branch, and when it last moved.
	mr := gitlab.MergeRequest{
		IID: 42, Title: "feat(api): paginate the search endpoint", Author: "alice.novak",
		SourceBranch: "feature/long-branch-name", TargetBranch: "develop",
		UpdatedAt: time.Date(time.Now().Year(), 7, 27, 9, 12, 0, 0, time.Local),
	}

	row := plain(renderRow(mrRow(mr), 140))
	for _, want := range []string{"!42", "paginate the search endpoint", "alice.novak", "feature/long", "27.7."} {
		if !strings.Contains(row, want) {
			t.Errorf("row = %q, want it to carry %q", row, want)
		}
	}
	// In that order.
	inOrder := []string{"!42", "paginate", "alice.novak", "feature/long", "27.7."}
	for i := 1; i < len(inOrder); i++ {
		if strings.Index(row, inOrder[i-1]) >= strings.Index(row, inOrder[i]) {
			t.Errorf("row = %q, want %q before %q", row, inOrder[i-1], inOrder[i])
		}
	}
}

func TestMRList_NarrowTerminalKeepsTheTitle(t *testing.T) {
	// What it is matters more than who wrote it, and a branch cut to eight characters says
	// nothing — so the extra columns go, widest first.
	mr := gitlab.MergeRequest{
		IID: 42, Title: "feat(api): paginate the search endpoint", Author: "alice.novak",
		SourceBranch: "feature/long-branch-name", UpdatedAt: time.Now(),
	}

	narrow := plain(renderRow(mrRow(mr), 60))
	if !strings.Contains(narrow, "paginate the") {
		t.Errorf("narrow row = %q, want the title kept", narrow)
	}
	if strings.Contains(narrow, "feature/long") {
		t.Errorf("narrow row = %q, want the branch dropped before the title", narrow)
	}
	if lipgloss.Width(narrow) > 60 {
		t.Errorf("narrow row is %d wide, want at most 60: %q", lipgloss.Width(narrow), narrow)
	}
}
