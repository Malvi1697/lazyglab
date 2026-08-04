package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

func helpApp(width, height int) *App {
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})
	a.width, a.height = width, height
	a.overlay = overlayHelp
	return a
}

func TestHelp_ScrollsAllTheWayToTheEnd(t *testing.T) {
	// The blank line before each section is a rendered row too.
	a := helpApp(120, 30)

	for i := 0; i < 200; i++ {
		a.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	box := ansi.Strip(a.renderHelp())
	if !strings.Contains(box, "Mark the whole list done") {
		t.Errorf("the last entry never came into view:\n%s", box)
	}
}

func TestHelp_EndKeyReachesTheLastEntry(t *testing.T) {
	a := helpApp(120, 30)
	a.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})

	box := ansi.Strip(a.renderHelp())
	if !strings.Contains(box, "Todos") {
		t.Errorf("G should jump to the end of the help:\n%s", box)
	}
}

func TestHelp_CounterMatchesWhatIsShown(t *testing.T) {
	a := helpApp(120, 30)
	rows, _, boxHeight := a.helpLayout()
	visible := boxHeight - 3

	box := ansi.Strip(a.renderHelp())
	// The first page shows rows 1..visible of however many there are.
	want := "(1-" + itoa(visible) + " of " + itoa(len(rows)) + ")"
	if !strings.Contains(box, want) {
		t.Errorf("counter should read %s:\n%s", want, box)
	}
}

func TestHelp_ShortTerminalStillRenders(t *testing.T) {
	a := helpApp(60, 8)
	if box := a.renderHelp(); box == "" {
		t.Error("the help must render even in a short terminal")
	}
}
