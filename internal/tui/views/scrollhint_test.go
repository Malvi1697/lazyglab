package views

import (
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestScrollHint_OnlyOfferedWhenSomethingCanScroll(t *testing.T) {
	// j/k scrolling a message that already fits looks like a broken key. The
	// footer now only names them where they would move something.
	page := newCommitDetail(&Context{})
	d := &page
	d.openAt(&gitlab.Commit{ID: "abc1234", ShortID: "abc1234", Title: "short"}, 0, 2)
	d.commit.Message = "short"

	d.body(120, 40)
	if got := hintsFor(d.keyHints()); strings.Contains(got, "j/k") {
		t.Errorf("hints = %q, want no scroll offered for a message that fits", got)
	}

	// A message far longer than the box does scroll, and says so.
	d.commit.Message = "short\n\n" + strings.Repeat("a line of the body\n", 60)
	d.body(120, 40)
	if got := hintsFor(d.keyHints()); !strings.Contains(got, "j/k") {
		t.Errorf("hints = %q, want the scroll offered for a message that overflows", got)
	}
}

func TestScrollHint_DiffSaysSoOnlyWhenItOverflows(t *testing.T) {
	page := newCommitDetail(&Context{})
	d := &page
	d.openAt(&gitlab.Commit{ID: "abc1234", ShortID: "abc1234", Title: "one"}, 0, 2)
	d.diffs = []gitlab.FileDiff{{NewPath: "a.py", Diff: "+x = 1\n"}}
	d.focus = focusFiles
	d.reading = true

	d.body(120, 40)
	if got := hintsFor(d.keyHints()); strings.Contains(got, "j/k") {
		t.Errorf("hints = %q, want no scroll offered for a one-line diff", got)
	}

	d.diffs[0].Diff = strings.Repeat("+x = 1\n", 200)
	d.diffCache = diffRender{} // the file changed under the same name
	d.body(120, 40)
	if got := hintsFor(d.keyHints()); !strings.Contains(got, "j/k") {
		t.Errorf("hints = %q, want the scroll offered for a long diff", got)
	}
}

func TestJobLog_EmptyLogSaysSoInsteadOfNothing(t *testing.T) {
	// A manual job has written nothing. Enter on one used to look like a dead key.
	page := newCommitDetail(&Context{})
	d := &page
	d.jobs.adopt(9, []gitlab.Job{{ID: 1, Name: "deploy", Status: "manual"}})

	_, status := d.update(JobTraceLoadedMsg{Trace: "   \n"})
	if !strings.Contains(status, "not written a log") {
		t.Errorf("status = %q, want it to say the job has no log yet", status)
	}
	if d.jobs.showingTrace() {
		t.Error("an empty log must not take the screen")
	}
}

func TestViewKeys_LowercaseHLStepsCommitsNotViews(t *testing.T) {
	// The swap the shell relies on: h/l move within what is open.
	if step, ok := commitStep("l"); !ok || step != 1 {
		t.Errorf(`commitStep("l") = %d, %v; want a step forward`, step, ok)
	}
	if step, ok := commitStep("h"); !ok || step != -1 {
		t.Errorf(`commitStep("h") = %d, %v; want a step back`, step, ok)
	}
	// Uppercase belongs to the shell's tabs, so the page must leave it alone.
	if _, ok := commitStep("L"); ok {
		t.Error(`commitStep("L") should not step: L switches views`)
	}
	if _, ok := commitStep("H"); ok {
		t.Error(`commitStep("H") should not step: H switches views`)
	}
}
