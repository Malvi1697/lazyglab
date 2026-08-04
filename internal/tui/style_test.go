package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// hasBackground reports whether s paints a background colour anywhere — the difference
// between text in the accent's colour and text on top of it.
func hasBackground(s string) bool {
	return strings.Contains(s, "\x1b[4") || strings.Contains(s, "\x1b[10") ||
		strings.Contains(s, ";4") || strings.Contains(s, ";10")
}

func TestTabs_TheTabYouAreOnIsAnAccentChip(t *testing.T) {
	// Six similar labels in a row need one of them to be obviously the answer to "where am
	// I".
	ids := []views.ViewID{views.ViewDashboard, views.ViewPipelines, views.ViewMRs}
	row := renderTabs(120, ids, 1, []string{"Dashboard", "Pipelines", "Merge Requests"}, "")

	if !hasBackground(row) {
		t.Errorf("tabs = %q, want the active tab painted, not merely coloured", row)
	}
	// The brackets carry the same information without any colour at all, for NO_COLOR and
	// for terminals that swallow backgrounds.
	if !strings.Contains(ansi.Strip(row), "[2] Pipelines") {
		t.Errorf("tabs = %q, want the active tab bracketed too", ansi.Strip(row))
	}
	if strings.Contains(ansi.Strip(row), "[1]") {
		t.Errorf("tabs = %q, want brackets on the active tab only", ansi.Strip(row))
	}
}

func TestSelectedRow_IsAnAccentBand(t *testing.T) {
	// Where you are should be the first thing you see on the page.
	selected := components.SelectRow("feat: a commit", 40, true)
	if !hasBackground(selected) {
		t.Errorf("row = %q, want the accent behind the current row", selected)
	}
	if plain := components.SelectRow("feat: a commit", 40, false); hasBackground(plain) {
		t.Errorf("row = %q, want nothing painted behind a row that is not current", plain)
	}
}

func TestPanelHeading_TheBoxWithTheKeysTakesTheAccent(t *testing.T) {
	// Two greys and one accent: the page says which box the keys drive without having to
	// be read.
	focused := components.RenderPanel("Recent Commits", []string{"a row"}, 60, 4, true)
	unfocused := components.RenderPanel("Readme", []string{"a line"}, 60, 4, false)

	if focused == ansi.Strip(focused) || unfocused == ansi.Strip(unfocused) {
		t.Fatal("both headings should carry styling")
	}
	if strings.Contains(focused, "\x1b[2m") {
		t.Errorf("focused heading = %q, want it not dimmed", focused)
	}
	if focused == unfocused {
		t.Error("the focused and unfocused headings should not look the same")
	}
}
