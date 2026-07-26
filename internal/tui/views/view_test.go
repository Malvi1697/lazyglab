package views

import (
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestParseViews(t *testing.T) {
	t.Run("empty returns the default tabs", func(t *testing.T) {
		got, warnings := ParseViews(nil)
		// Commits is deliberately not a default tab: Overview lists recent commits
		// and Enter opens the full commit page in place. It stays opt-in.
		want := []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues, ViewTodos}
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
	views := []ViewID{ViewOverview, ViewPipelines}

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

func TestRefAndTitle_TheNumberIsMetadataLikeTheCommitPrefix(t *testing.T) {
	// The same change read one way in Recent Commits and another in Pipelines: the
	// conventional-commit prefix was dimmed in one list and bright in the others.
	row := refAndTitle("!42", "feat(cart): capacity-aware promotion")
	plainRow := plain(row)
	if plainRow != "!42 feat(cart): capacity-aware promotion" {
		t.Errorf("row = %q, want the text unchanged underneath the styling", plainRow)
	}

	// The number and the prefix carry styling; the subject is left alone, so the
	// escape codes stop before it.
	if row == plainRow {
		t.Errorf("row = %q, want the number and prefix dimmed", row)
	}
	if i := strings.Index(row, "capacity-aware promotion"); i < 0 || strings.Contains(row[i:], "\x1b[") {
		t.Errorf("row = %q, want no styling from the subject onwards", row)
	}

	// Without a number it is just a title.
	if got := plain(refAndTitle("", "chore: bump deps")); got != "chore: bump deps" {
		t.Errorf("got %q, want the bare title", got)
	}
}

func TestRefAndTitle_EveryListUsesIt(t *testing.T) {
	// Each of these renders a row that names something by number and then says what
	// it is; they must all read the same way.
	rows := map[string]string{
		"merge request": mrRow(gitlab.MergeRequest{IID: 42, Title: "feat(cart): promote"}),
		"issue":         issueRow(gitlab.Issue{IID: 7, Title: "fix(api): crash"}),
		"pipeline":      pipelineRow(gitlab.Pipeline{ID: 1, Status: "success", CommitTitle: "feat(cart): promote"}),
	}
	for name, row := range rows {
		if row == plain(row) {
			t.Errorf("%s row = %q, want the number and prefix dimmed", name, row)
		}
	}
}
