package views

import (
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestDiffLine_MarkerColumnCarriesTheMeaning(t *testing.T) {
	added := styleDiffLine("thing.py", `+    x = "one"`, 40)
	if len(added) != 1 {
		t.Fatalf("got %d rows, want one", len(added))
	}
	if !strings.HasPrefix(plain(added[0]), "+") {
		t.Errorf("row = %q, want the + kept in the gutter", plain(added[0]))
	}
	// The code beside the marker is highlighted, so the row is more than one colour.
	if added[0] == plain(added[0]) {
		t.Error("the code beside the marker should be syntax highlighted")
	}

	context := styleDiffLine("thing.py", `     x = 1`, 40)
	if got := plain(context[0]); !strings.HasPrefix(got, " ") {
		t.Errorf("context row = %q, want an empty gutter", got)
	}
}

func TestDiffLine_WrappedRowsStayAligned(t *testing.T) {
	// A wrapped line used to lose its colour entirely on the continuation rows; now the
	// gutter stays a gutter, so the code column lines up down the screen.
	long := "+" + strings.Repeat("word ", 20)
	rows := styleDiffLine("notes.txt", long, 30)
	if len(rows) < 2 {
		t.Fatalf("got %d rows, want the line wrapped", len(rows))
	}
	if !strings.HasPrefix(plain(rows[0]), "+") {
		t.Errorf("first row = %q, want the marker", plain(rows[0]))
	}
	for i, r := range rows[1:] {
		got := plain(r)
		if strings.HasPrefix(got, "+") {
			t.Errorf("continuation row %d = %q, want no second claim of the marker", i+1, got)
		}
		if !strings.HasPrefix(got, " ") {
			t.Errorf("continuation row %d = %q, want it aligned under the code", i+1, got)
		}
	}
}

func TestDiffLine_HunkHeaderAndNotesAreNotCode(t *testing.T) {
	hunk := styleDiffLine("thing.py", "@@ -1,4 +1,6 @@ def f():", 60)
	if len(hunk) != 1 || !strings.HasPrefix(plain(hunk[0]), "@@") {
		t.Errorf("hunk = %v, want the header kept whole", hunk)
	}

	note := styleDiffLine("thing.py", `\ No newline at end of file`, 60)
	if len(note) != 1 || !strings.Contains(plain(note[0]), "No newline") {
		t.Errorf("note = %v, want the note kept whole", note)
	}
}

func TestDiffLines_AreCachedPerFileAndWidth(t *testing.T) {
	d := newCommitDetail(&Context{})
	f := &gitlab.FileDiff{NewPath: "thing.py", Diff: "+x = 1\n-x = 2\n"}

	first := d.diffLines(f, 40)
	if d.diffCache.path != "thing.py" || d.diffCache.width != 40 {
		t.Fatalf("cache = %+v, want it keyed by file and width", d.diffCache)
	}
	if got := d.diffLines(f, 40); &got[0] != &first[0] {
		t.Error("a second render at the same width should reuse the rendered lines")
	}
	// A resize has to re-wrap, so the cache must not answer for another width.
	d.diffLines(f, 20)
	if d.diffCache.width != 20 {
		t.Errorf("cache width = %d, want the new width", d.diffCache.width)
	}
}

func TestDiffCache_DoesNotSurviveIntoAnotherCommit(t *testing.T) {
	// Two commits touching the same file would otherwise show the first one's diff.
	d := newCommitDetail(&Context{})
	d.diffCache = diffRender{path: "thing.py", width: 40, lines: []string{"stale"}}

	d.openAt(&gitlab.Commit{ID: "abc", ShortID: "abc"}, 0, 2)
	if d.diffCache.lines != nil {
		t.Errorf("cache = %+v, want it dropped when another commit opens", d.diffCache)
	}
}
