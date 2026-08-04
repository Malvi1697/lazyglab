package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

func TestHintBar_ReadsAsActionThenKey(t *testing.T) {
	// One shape everywhere, lazygit's: what it does, then the key that does it.
	bar := ansi.Strip(hintBar(120,
		views.KeyHint{Key: "Enter", Desc: "Commit page"}, views.KeyHint{Key: "y/Y", Desc: "Copy SHA/link"}))

	if bar != "Commit page: Enter | Copy SHA/link: y/Y" {
		t.Errorf("bar = %q", bar)
	}
}

func TestHintBar_DropsWholeHintsRatherThanCuttingOne(t *testing.T) {
	// A half-written hint is worse than a missing one; the help overlay has them all.
	hints := []views.KeyHint{
		{Key: "Enter", Desc: "Commit page"}, {Key: "t", Desc: "Show readme"}, {Key: "y/Y", Desc: "Copy SHA/link"},
	}
	bar := ansi.Strip(hintBar(24, hints...))

	if lipgloss.Width(bar) > 24 {
		t.Errorf("bar is %d columns wide: %q", lipgloss.Width(bar), bar)
	}
	if !strings.HasPrefix(bar, "Commit page: Enter") {
		t.Errorf("bar = %q, want the first hint kept whole", bar)
	}
	if strings.Contains(bar, "Copy") {
		t.Errorf("bar = %q, want the hints that do not fit dropped", bar)
	}
}

func TestFooter_KeepsTheWayOutHoweverNarrow(t *testing.T) {
	// Whatever is dropped, "?" is not: it is how you find the rest.
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})

	for _, width := range []int{40, 80, 200} {
		footer := ansi.Strip(renderFooter(width, a.globalHints(), a.activeView().KeyHints()))
		if !strings.Contains(footer, "Keybindings: ?") {
			t.Errorf("width %d: footer = %q, want the help key kept", width, strings.TrimSpace(footer))
		}
		if lipgloss.Width(footer) > width {
			t.Errorf("width %d: footer is %d columns wide", width, lipgloss.Width(footer))
		}
	}
}

func TestFooter_TheViewsOwnKeysComeFirst(t *testing.T) {
	// They are what this screen can do, so they are what a narrow terminal keeps.
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})

	footer := ansi.Strip(renderFooter(200, a.globalHints(), a.activeView().KeyHints()))
	view, global := strings.Index(footer, "Commit page: Enter"), strings.Index(footer, "Project: P")
	if view < 0 || global < 0 {
		t.Fatalf("footer = %q, want both kinds of hint", footer)
	}
	if view > global {
		t.Errorf("footer = %q, want the view's keys before the global ones", footer)
	}
}
