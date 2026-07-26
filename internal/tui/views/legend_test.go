package views

import (
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// hintsFor renders a state's footer hints as one searchable string.
func hintsFor(hints []KeyHint) string {
	var parts []string
	for _, h := range hints {
		parts = append(parts, h.Key+" "+h.Desc)
	}
	return plain(strings.Join(parts, " · "))
}

// legendPage is a commit page with two files and two jobs, so every box exists.
func legendPage(t *testing.T) *commitDetail {
	t.Helper()
	page := newCommitDetail(&Context{})
	d := &page
	d.openAt(&gitlab.Commit{ID: "abc1234", ShortID: "abc1234", Title: "one"}, 0, 3)
	d.diffs = []gitlab.FileDiff{
		{NewPath: "a.py", Diff: "+x = 1\n"},
		{NewPath: "b.py", Diff: "-y = 2\n"},
	}
	d.jobs.adopt(9, []gitlab.Job{{ID: 1, Name: "build", Stage: "build"}})
	return d
}

func TestLegend_CommitPageNamesBothWaysToStepCommits(t *testing.T) {
	// h/l work everywhere the arrows do, and were never mentioned anywhere.
	d := legendPage(t)
	got := hintsFor(d.keyHints())
	if !strings.Contains(got, "h/l") {
		t.Errorf("page hints = %q, want the h/l pair named", got)
	}
}

func TestLegend_DiffReaderNamesTheFileKeys(t *testing.T) {
	d := legendPage(t)
	d.focus = focusFiles
	d.reading = true

	got := hintsFor(d.keyHints())
	if !strings.Contains(got, "h/l") || !strings.Contains(strings.ToLower(got), "file") {
		t.Errorf("diff hints = %q, want it to say the arrows and h/l step files", got)
	}

	// And the keys it names really do that, rather than stepping commits.
	d.handleKey("l", 20)
	if d.fileCursor != 1 {
		t.Errorf("fileCursor = %d, want l to have stepped to the next file", d.fileCursor)
	}
	d.handleKey("left", 20)
	if d.fileCursor != 0 {
		t.Errorf("fileCursor = %d, want ← to have stepped back", d.fileCursor)
	}
}

func TestLegend_FilesBoxSaysTheArrowsStillMoveCommits(t *testing.T) {
	// In the files box the arrows belong to the host list, not to the box — the
	// footer has to say which, since it is not the same as inside a diff.
	d := legendPage(t)
	d.focus = focusFiles

	got := hintsFor(d.keyHints())
	if !strings.Contains(got, "h/l") || !strings.Contains(strings.ToLower(got), "commit") {
		t.Errorf("files hints = %q, want it to say the arrows step commits", got)
	}
	if d.readingBody() {
		t.Error("the files box does not own the arrows; the host must still see them")
	}
}

func TestLegend_JobsBoxKeepsThePageKeys(t *testing.T) {
	d := legendPage(t)
	d.focus = focusJobs

	got := hintsFor(d.keyHints())
	for _, want := range []string{"Enter", "Tab", "h/l"} {
		if !strings.Contains(got, want) {
			t.Errorf("jobs hints = %q, want %q named", got, want)
		}
	}
}

func TestLegend_AnOpenLogOwnsTheArrows(t *testing.T) {
	// The footer must not offer commit stepping while a log has the screen: it
	// would swap out what you are reading.
	d := legendPage(t)
	d.focus = focusJobs
	d.jobs.setTrace("some output")

	if !d.readingBody() {
		t.Fatal("an open log should own the arrows")
	}
	if got := hintsFor(d.keyHints()); strings.Contains(got, "h/l") {
		t.Errorf("log hints = %q, want no commit stepping offered", got)
	}
}

func TestLegend_EveryListOffersTheSearch(t *testing.T) {
	ctx := &Context{}
	for name, hints := range map[string][]KeyHint{
		"overview":  NewOverviewView(ctx).KeyHints(),
		"pipelines": NewPipelinesView(ctx).KeyHints(),
		"mrs":       NewMRsView(ctx).KeyHints(),
		"issues":    NewIssuesView(ctx).KeyHints(),
		"todos":     NewTodosView(ctx).KeyHints(),
		"commits":   NewCommitsView(ctx).KeyHints(),
	} {
		if got := hintsFor(hints); !strings.Contains(got, "/ Search") {
			t.Errorf("%s hints = %q, want the search offered", name, got)
		}
	}
}
