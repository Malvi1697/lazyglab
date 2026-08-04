package views

import (
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestParseViews(t *testing.T) {
	t.Run("empty returns the default tabs", func(t *testing.T) {
		got, warnings := ParseViews(nil)
		// Commits is deliberately not a default tab: Overview lists recent commits and Enter
		// opens the full commit page in place.
		want := []ViewID{ViewDashboard, ViewPipelines, ViewMRs, ViewIssues, ViewTodos}
		if len(got) != len(want) {
			t.Fatalf("want %d views, got %d (%v)", len(want), len(got), got)
		}
		for i, id := range want {
			if got[i] != id {
				t.Errorf("index %d: want %v, got %v", i, id, got[i])
			}
		}
		if warnings != nil {
			t.Errorf("want no warnings, got %v", warnings)
		}
	})

	t.Run("subset in given order", func(t *testing.T) {
		got, warnings := ParseViews([]string{"pipelines", "commits"})
		want := []ViewID{ViewPipelines, ViewCommits}
		if len(got) != len(want) {
			t.Fatalf("want %d views, got %d (%v)", len(want), len(got), got)
		}
		for i, id := range want {
			if got[i] != id {
				t.Errorf("index %d: want %v, got %v", i, id, got[i])
			}
		}
		if warnings != nil {
			t.Errorf("want no warnings, got %v", warnings)
		}
	})

	t.Run("unknown and duplicate dropped with warnings", func(t *testing.T) {
		got, warnings := ParseViews([]string{"pipelines", "bogus", "pipelines"})
		want := []ViewID{ViewPipelines}
		if len(got) != len(want) {
			t.Fatalf("want %d views, got %d (%v)", len(want), len(got), got)
		}
		for i, id := range want {
			if got[i] != id {
				t.Errorf("index %d: want %v, got %v", i, id, got[i])
			}
		}
		if len(warnings) != 2 {
			t.Fatalf("want 2 warnings, got %d (%v)", len(warnings), warnings)
		}
	})
}

func TestDefaultViewIndex(t *testing.T) {
	views := []ViewID{ViewDashboard, ViewPipelines}

	if got := DefaultViewIndex(views, "pipelines"); got != 1 {
		t.Errorf(`DefaultViewIndex(views, "pipelines") = %d, want 1`, got)
	}
	if got := DefaultViewIndex(views, ""); got != 0 {
		t.Errorf(`DefaultViewIndex(views, "") = %d, want 0`, got)
	}
	if got := DefaultViewIndex(views, "issues"); got != 0 {
		t.Errorf(`DefaultViewIndex(views, "issues") = %d, want 0 (absent from enabled list)`, got)
	}
}

func TestListRow_TheNumberAndTheKindAreMetadata(t *testing.T) {
	// The same change read one way in Recent Commits and another in Pipelines: the
	// conventional-commit prefix was dimmed in one list and bright in the others.
	row := renderRow(listRow{ref: "!42", kind: "feat:", subject: "paginate the search endpoint"}, 60)
	plainRow := plain(row)
	if !strings.HasPrefix(plainRow, "!42 feat: paginate the search endpoint") {
		t.Errorf("row = %q, want the number then the kind then the subject", plainRow)
	}

	// Measured over the list, and right-aligned within it: a four-digit number in one row
	// indents the shorter ones so every subject starts in the same column.
	mixed := renderRows([]listRow{
		{ref: "!42", subject: "short number"},
		{ref: "!1234", subject: "long number"},
	}, 60)
	if !strings.HasPrefix(plain(mixed[0]), "  !42 short number") {
		t.Errorf("row = %q, want the shorter number right-aligned to the longer one", plain(mixed[0]))
	}

	// The metadata carries styling; the subject is left alone, so the escape codes stop
	// before it.
	if row == plainRow {
		t.Errorf("row = %q, want the number and kind dimmed", row)
	}
	if i := strings.Index(row, "paginate the search endpoint"); i < 0 || strings.Contains(row[i:], "\x1b[") {
		t.Errorf("row = %q, want no styling from the subject onwards", row)
	}

	// A column no row fills costs nothing: no number means no gap where one would be.
	if got := plain(renderRow(listRow{kind: "chore:", subject: "bump deps"}, 60)); !strings.HasPrefix(got, "chore: bump deps") {
		t.Errorf("got %q, want no room reserved for a number the list does not have", got)
	}
}

func TestListRow_EveryListIsLaidOutTheSameWay(t *testing.T) {
	// The point of the shared layout: pick any list and the columns mean the same thing in
	// the same order, with the subject starting in the same place.
	rows := map[string]listRow{
		"merge request": mrRow(gitlab.MergeRequest{IID: 42, Title: "feat(cart): promote", Author: "jiri"}),
		"issue":         issueRow(gitlab.Issue{IID: 7, Title: "fix(api): crash", Author: "alice.novak"}),
		"pipeline":      pipelineRow(gitlab.Pipeline{ID: 1, Status: "success", CommitTitle: "feat(cart): promote"}, nil),
		"commit":        commitItemRow(gitlab.Commit{ShortID: "abc1234", Title: "feat(cart): promote", AuthorName: "jiri"}),
		"todo":          todoRow(gitlab.Todo{Reference: "!42", Action: "review_requested", Title: "feat(cart): promote"}),
	}
	for name, r := range rows {
		if r.kind == "" && r.ref == "" {
			t.Errorf("%s row = %+v, want it to fill the metadata columns", name, r)
		}
		row := renderRow(r, 120)
		if row == plain(row) {
			t.Errorf("%s row = %q, want its metadata dimmed", name, row)
		}
		if !strings.Contains(plain(row), "promote") && !strings.Contains(plain(row), "crash") {
			t.Errorf("%s row = %q, want the subject in it", name, plain(row))
		}
	}
}
