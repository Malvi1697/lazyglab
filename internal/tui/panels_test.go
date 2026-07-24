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
