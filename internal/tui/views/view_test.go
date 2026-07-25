package views

import "testing"

func TestParseViews(t *testing.T) {
	t.Run("empty returns all five in default order", func(t *testing.T) {
		got, warnings := ParseViews(nil)
		want := []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues, ViewCommits}
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
