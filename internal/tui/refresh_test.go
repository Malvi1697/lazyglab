package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// refreshApp is a shell with one view and a pinned clock, so the note's wording
// can be asserted at an exact moment.
func refreshApp(t *testing.T, at time.Time) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:         map[string]*gitlab.Client{"h": nil},
		HostNames:       []string{"h"},
		DetectedHost:    "h",
		ViewIDs:         []views.ViewID{views.ViewOverview},
		RefreshInterval: 30 * time.Second,
	})
	a.width, a.height = 120, 40
	a.now = func() time.Time { return at }
	// A project and a client, so the view has something to fetch and Focus returns
	// a command. The command itself is never run here.
	a.ctx.Project = &gitlab.Project{ID: 1, PathWithNamespace: "g/p"}
	a.ctx.Client = &gitlab.Client{}
	return a
}

func note(a *App, at time.Time) string { return ansi.Strip(a.refreshNote(at)) }

func TestRefresh_NothingToSayBeforeTheFirstLoad(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	if got := note(a, now); got != "" {
		t.Errorf("note = %q, want nothing before any data has arrived", got)
	}
}

func TestRefresh_PressingRSpins(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)

	_, cmd := a.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if cmd == nil {
		t.Fatal("r should start a refresh")
	}
	if !a.refreshing {
		t.Error("r should mark the refresh as in flight")
	}
	if got := note(a, now); !strings.Contains(got, "refreshing") {
		t.Errorf("note = %q, want it to say a refresh is running", got)
	}
}

func TestRefresh_ClaimsNothingWhenThereIsNothingToFetch(t *testing.T) {
	// Without a project the view has no request to make; a spinner would turn
	// forever with nothing coming back to stop it.
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.ctx.Project = nil

	a.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if a.refreshing {
		t.Error("a refresh with nothing to fetch must not report as in flight")
	}
}

func TestRefresh_DataArrivingIsCalledOut(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	a.Update(views.CommitsLoadedMsg{Commits: []gitlab.Commit{{ShortID: "abc1234"}}})
	if a.refreshing {
		t.Error("data arriving should end the refresh")
	}
	if got := note(a, now); !strings.Contains(got, "updated now") {
		t.Errorf("note = %q, want it to say the data just arrived", got)
	}
}

func TestRefresh_ThenAgesAndCountsDown(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.lastRefresh = now
	a.nextRefresh = now.Add(30 * time.Second)

	got := note(a, now.Add(12*time.Second))
	if !strings.Contains(got, "updated 12s ago") {
		t.Errorf("note = %q, want it to say how stale the data is", got)
	}
	if !strings.Contains(got, "next 18s") {
		t.Errorf("note = %q, want the countdown to the automatic refresh", got)
	}
}

func TestRefresh_NoCountdownWithoutAutoRefresh(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.refreshInterval = 0
	a.lastRefresh = now

	got := note(a, now.Add(time.Minute))
	if !strings.Contains(got, "updated 1m ago") {
		t.Errorf("note = %q, want the age of the data", got)
	}
	if strings.Contains(got, "next") {
		t.Errorf("note = %q, want no countdown when nothing is scheduled", got)
	}
}

func TestRefresh_TickReschedulesTheCountdown(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)

	a.Update(tickMsg{})
	if want := now.Add(30 * time.Second); !a.nextRefresh.Equal(want) {
		t.Errorf("nextRefresh = %v, want %v", a.nextRefresh, want)
	}
	if !a.refreshing {
		t.Error("the automatic tick should refresh like r does")
	}
}

func TestRefresh_SpinnerStopsWhenTheRefreshDoes(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})

	// Each frame re-arms while the refresh is running...
	if _, cmd := a.Update(spinMsg{}); cmd == nil {
		t.Fatal("the spinner should keep ticking while a refresh is in flight")
	}
	if a.spinFrame == 0 {
		t.Error("the spinner should have advanced a frame")
	}

	// ...and the chain ends once the data is in, rather than ticking forever.
	a.Update(views.CommitsLoadedMsg{})
	if _, cmd := a.Update(spinMsg{}); cmd != nil {
		t.Error("the spinner should stop after the refresh finishes")
	}
	if a.spinning {
		t.Error("no spinner chain should be left running")
	}
}

func TestRefresh_NoteSharesTheBarWithALongStatus(t *testing.T) {
	// An API error in the status must not push the note off the row, and neither
	// may the row wrap onto the tabs beneath it.
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.lastRefresh = now
	a.nextRefresh = now.Add(30 * time.Second)
	a.setStatus(strings.Repeat("Error loading pipelines: connection refused ", 6), true)

	bar := renderContextBar(a.width, a.ctx, a.statusText, a.statusIsErr,
		a.refreshNote(now.Add(5*time.Second)))
	if strings.Contains(bar, "\n") {
		t.Error("the context bar must stay one row")
	}
	if plain := ansi.Strip(bar); !strings.Contains(plain, "next 25s") {
		t.Errorf("bar = %q, want the refresh note kept at the right", plain)
	}
}

func TestRefresh_PausesWhileTheTerminalIsUnfocused(t *testing.T) {
	// Polling GitLab every thirty seconds for a window nobody is looking at is
	// pure waste, and the note must not promise a countdown it will not honour.
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.lastRefresh = now.Add(-time.Minute)

	a.Update(tea.BlurMsg{})
	if _, cmd := a.Update(tickMsg{}); cmd != nil {
		// The tick still re-arms itself, so a command is expected — but it must not
		// be a refresh.
		if a.refreshing {
			t.Error("the tick refreshed while the terminal was in the background")
		}
	}
	if got := note(a, now); !strings.Contains(got, "paused") {
		t.Errorf("note = %q, want it to say the refresh is paused", got)
	}

	// Coming back to stale data refreshes at once, rather than waiting out the tick.
	_, cmd := a.Update(tea.FocusMsg{})
	if cmd == nil || !a.refreshing {
		t.Error("regaining focus with stale data should refresh")
	}
}

func TestRefresh_FocusWithFreshDataDoesNotRefetch(t *testing.T) {
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := refreshApp(t, now)
	a.lastRefresh = now.Add(-2 * time.Second)

	a.Update(tea.BlurMsg{})
	if _, cmd := a.Update(tea.FocusMsg{}); cmd != nil {
		t.Error("a glance away and back must not cost a request")
	}
}

func TestRefresh_SwitchingViewsReusesWhatWasJustFetched(t *testing.T) {
	// Flipping through the tabs used to refetch every one of them on the way past.
	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	a := NewApp(Options{
		Clients:         map[string]*gitlab.Client{"h": nil},
		HostNames:       []string{"h"},
		DetectedHost:    "h",
		ViewIDs:         []views.ViewID{views.ViewOverview, views.ViewIssues},
		RefreshInterval: 30 * time.Second,
	})
	a.width, a.height = 120, 40
	a.now = func() time.Time { return now }
	a.ctx.Project = &gitlab.Project{ID: 1, PathWithNamespace: "g/p"}
	a.ctx.Client = &gitlab.Client{}

	// Both views have just been looked at.
	a.Update(views.CommitsLoadedMsg{})
	a.Update(tea.KeyPressMsg{Code: 'L', Text: "L"})
	a.Update(views.IssuesLoadedMsg{})

	if _, cmd := a.Update(tea.KeyPressMsg{Code: 'H', Text: "H"}); cmd != nil {
		t.Error("going back to a view fetched a moment ago should reuse it")
	}

	// r is an explicit ask and always reaches GitLab.
	if _, cmd := a.Update(tea.KeyPressMsg{Code: 'r', Text: "r"}); cmd == nil {
		t.Error("r must always refresh, however fresh the data looks")
	}

	// And once the window has passed, the tab refetches on its own.
	now = now.Add(viewFreshFor + time.Second)
	if _, cmd := a.Update(tea.KeyPressMsg{Code: 'L', Text: "L"}); cmd == nil {
		t.Error("a view whose data has aged past the window should refetch")
	}
}
