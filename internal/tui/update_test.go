package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

func updateApp(width int, check CheckUpdateFunc) *App {
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard, views.ViewPipelines},
		CheckUpdate:  check,
	})
	a.width, a.height = width, 30
	return a
}

// tabsRow is the second line of the frame, where the notice lives.
func tabsRow(a *App) string {
	lines := strings.Split(ansi.Strip(a.View().Content), "\n")
	if len(lines) < 2 {
		return ""
	}
	return lines[1]
}

func TestUpdateNotice_AppearsBesideTheTabsAndNamesTheCommand(t *testing.T) {
	// The notice used to be printed to stderr just before the alt screen replaced
	// it, so nobody ever read it. It has to be somewhere that survives, and it has
	// to say what to type — "update available" alone sends people searching.
	a := updateApp(100, func() string { return "0.5.0" })
	a.Update(updateFoundMsg{version: "0.5.0"})

	row := tabsRow(a)
	if !strings.Contains(row, "v0.5.0") {
		t.Errorf("tabs row = %q, want the new version named", row)
	}
	if !strings.Contains(row, "lazyglab update") {
		t.Errorf("tabs row = %q, want the command that installs it", row)
	}
	// It must not push the tabs off their line.
	if !strings.Contains(row, "[1] ") {
		t.Errorf("tabs row = %q, want the tabs still there", row)
	}
	// Measured in columns, not bytes: the arrow and the separators are multibyte.
	if got := lipgloss.Width(row); got > 100 {
		t.Errorf("tabs row is %d columns wide, want at most 100", got)
	}
}

func TestUpdateNotice_SaysNothingWhenUpToDate(t *testing.T) {
	a := updateApp(100, func() string { return "" })
	a.Update(updateFoundMsg{version: ""})

	if row := tabsRow(a); strings.Contains(row, "available") {
		t.Errorf("tabs row = %q, want no notice when there is nothing newer", row)
	}
}

func TestUpdateNotice_ANarrowTerminalKeepsTheTabs(t *testing.T) {
	// Knowing which view you are in beats knowing a release is out, so the notice is
	// dropped rather than wrapped onto the row below.
	a := updateApp(40, func() string { return "0.5.0" })
	a.Update(updateFoundMsg{version: "0.5.0"})

	row := tabsRow(a)
	if strings.Contains(row, "available") {
		t.Errorf("tabs row = %q, want the notice dropped when there is no room", row)
	}
	if !strings.Contains(row, "[1] ") {
		t.Errorf("tabs row = %q, want the tabs intact", row)
	}
}

func TestUpdateCheck_RunsOnceAndOnlyWhenThereIsSomethingToAsk(t *testing.T) {
	// The check is a network call, so it belongs in a command rather than on the
	// render path, and one per session is enough — a release that lands while the
	// app is open can wait for the next launch.
	calls := 0
	a := updateApp(100, func() string { calls++; return "0.5.0" })

	if calls != 0 {
		t.Fatalf("the check ran %d times before any command did", calls)
	}
	cmd := a.updateCheckCmd()
	if cmd == nil {
		t.Fatal("expected a command to run the check")
	}
	msg, ok := cmd().(updateFoundMsg)
	if !ok {
		t.Fatalf("check returned %T, want updateFoundMsg", cmd())
	}
	if msg.version != "0.5.0" || calls != 1 {
		t.Errorf("msg = %+v after %d calls, want 0.5.0 after exactly one", msg, calls)
	}

	// Nothing injected (a test, or an install that wants no network) means no cmd.
	if got := updateApp(100, nil).updateCheckCmd(); got != nil {
		t.Error("want no command when no check was injected")
	}
}

func TestUpdateFound_IsNotHandedToTheView(t *testing.T) {
	// Every unrecognised message falls through to the active view; this one is the
	// shell's own and would otherwise reach a view that cannot make sense of it.
	a := updateApp(100, func() string { return "0.5.0" })
	if cmd := a.updateCheckCmd(); cmd != nil {
		a.Update(cmd())
	}
	if a.updateVersion != "0.5.0" {
		t.Errorf("updateVersion = %q, want the shell to have kept it", a.updateVersion)
	}
	// Not a load result, so it must not be mistaken for data arriving and reset the
	// refresh note.
	if !a.lastRefresh.IsZero() {
		t.Error("the update check must not count as a refresh")
	}
}
