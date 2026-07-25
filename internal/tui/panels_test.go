package tui

import (
	"reflect"
	"testing"
)

func TestParsePanels(t *testing.T) {
	all := []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests, PanelIssues}

	t.Run("empty returns default order", func(t *testing.T) {
		got, warn := ParsePanels(nil)
		if !reflect.DeepEqual(got, all) {
			t.Errorf("got %v, want %v", got, all)
		}
		if len(warn) != 0 {
			t.Errorf("unexpected warnings: %v", warn)
		}
	})

	t.Run("hide issues", func(t *testing.T) {
		got, _ := ParsePanels([]string{"projects", "pipelines", "merge_requests"})
		want := []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("reorder", func(t *testing.T) {
		got, _ := ParsePanels([]string{"projects", "issues", "pipelines", "merge_requests"})
		want := []PanelID{PanelProjects, PanelIssues, PanelPipelines, PanelMergeRequests}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("unknown dropped with warning", func(t *testing.T) {
		got, warn := ParsePanels([]string{"projects", "bogus", "issues"})
		want := []PanelID{PanelProjects, PanelIssues}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})

	t.Run("duplicate dropped with warning", func(t *testing.T) {
		got, warn := ParsePanels([]string{"projects", "pipelines", "pipelines"})
		want := []PanelID{PanelProjects, PanelPipelines}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})

	t.Run("projects forced present when omitted", func(t *testing.T) {
		got, warn := ParsePanels([]string{"pipelines", "issues"})
		want := []PanelID{PanelProjects, PanelPipelines, PanelIssues}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
		if len(warn) != 1 {
			t.Errorf("want 1 warning, got %v", warn)
		}
	})
}

func TestWrapLine(t *testing.T) {
	got := wrapLine("the quick brown fox", 9)
	for _, l := range got {
		if len([]rune(l)) > 9 {
			t.Errorf("line %q exceeds width 9", l)
		}
	}
	if len(got) < 2 {
		t.Errorf("expected wrapping into multiple lines, got %v", got)
	}

	// Overlong single word hard-breaks.
	got = wrapLine("supercalifragilistic", 5)
	if len(got) < 4 {
		t.Errorf("expected hard break into >=4 chunks, got %v", got)
	}
}
