package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// savedFavorites records what a stub SaveFavoritesFunc was asked to persist.
type savedFavorites struct {
	host  string
	paths []string
	err   error
	calls int
}

// newFavApp builds a shell with projects loaded and a recording save function.
func newFavApp(t *testing.T, favorites []string, saved *savedFavorites) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"gitlab.example.com": nil},
		HostNames:    []string{"gitlab.example.com"},
		DetectedHost: "gitlab.example.com",
		ViewIDs:      []views.ViewID{views.ViewOverview, views.ViewPipelines, views.ViewCommits},
		Favorites:    favorites,
		SaveFavorites: func(host string, paths []string) error {
			saved.host = host
			saved.paths = paths
			saved.calls++
			return saved.err
		},
	})
	a.width, a.height = 100, 40
	a.projects = []gitlab.Project{
		{ID: 1, NameWithNamespace: "Group / alpha", PathWithNamespace: "group/alpha"},
		{ID: 2, NameWithNamespace: "Group / beta", PathWithNamespace: "group/beta"},
	}
	return a
}

func TestFavorites_StarFromProjectPickerPersists(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, nil, &saved)

	press(a, "P") // projects already loaded, so the picker opens at once
	if a.overlay != overlayProject {
		t.Fatalf("expected the project picker, got overlay %v", a.overlay)
	}

	cmd := press(a, "f")
	if !a.isFavorite("group/alpha") {
		t.Error("expected the highlighted project to be starred")
	}
	if cmd == nil {
		t.Fatal("expected a command persisting the change")
	}
	a.Update(cmd())

	if saved.calls != 1 {
		t.Errorf("SaveFavorites calls = %d, want 1", saved.calls)
	}
	if saved.host != "gitlab.example.com" {
		t.Errorf("saved host = %q", saved.host)
	}
	if len(saved.paths) != 1 || saved.paths[0] != "group/alpha" {
		t.Errorf("saved paths = %v, want [group/alpha]", saved.paths)
	}
	if a.statusIsErr {
		t.Errorf("a successful save must not set an error status: %q", a.statusText)
	}
}

func TestFavorites_StarTogglesOff(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, []string{"group/alpha"}, &saved)

	press(a, "P")
	press(a, "f") // alpha is highlighted and already starred

	if a.isFavorite("group/alpha") {
		t.Error("expected the second press to unstar the project")
	}
	if len(saved.paths) != 0 {
		t.Errorf("saved paths = %v, want empty", saved.paths)
	}
}

func TestFavorites_FailedSaveIsReported(t *testing.T) {
	saved := savedFavorites{err: errors.New("disk full")}
	a := newFavApp(t, nil, &saved)

	press(a, "P")
	cmd := press(a, "f")
	a.Update(cmd())

	if !a.statusIsErr || !strings.Contains(a.statusText, "disk full") {
		t.Errorf("expected the write failure in the status bar, got %q (isErr=%v)", a.statusText, a.statusIsErr)
	}
}

func TestFavorites_PickerOpensAndSelects(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, []string{"group/beta"}, &saved)

	press(a, "f")
	if a.overlay != overlayFavorites {
		t.Fatalf("expected the favorites picker, got overlay %v", a.overlay)
	}

	cmd := press(a, "enter")
	if a.overlay != overlayNone {
		t.Error("selecting a favorite should close the picker")
	}
	if cmd == nil {
		t.Fatal("expected a command selecting the project")
	}

	// beta is among the loaded projects, so it resolves without an API call.
	msg, ok := cmd().(views.ProjectSelectedMsg)
	if !ok {
		t.Fatalf("expected ProjectSelectedMsg, got %T", cmd())
	}
	if msg.Project.ID != 2 {
		t.Errorf("selected project ID = %d, want 2 (group/beta)", msg.Project.ID)
	}
}

func TestFavorites_PickerUnstarsWithF(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, []string{"group/alpha", "group/beta"}, &saved)

	press(a, "f") // open
	press(a, "j") // move to beta
	press(a, "f") // unstar it

	if a.isFavorite("group/beta") {
		t.Error("expected f inside the picker to unstar the highlighted favorite")
	}
	if a.overlay != overlayFavorites {
		t.Error("unstarring should keep the picker open")
	}
	if len(a.favorites) != 1 || a.favorites[0] != "group/alpha" {
		t.Errorf("favorites = %v, want [group/alpha]", a.favorites)
	}
}

func TestFavorites_EmptyPickerExplainsHowToStar(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, nil, &saved)

	press(a, "f")
	out := a.renderFavorites()
	if !strings.Contains(out, "No favorites yet") {
		t.Error("expected the empty picker to say so")
	}
	if !strings.Contains(out, "press f to star") {
		t.Error("expected the empty picker to explain how to add favorites")
	}
	if cmd := press(a, "enter"); cmd != nil {
		t.Error("Enter with no favorites must do nothing")
	}
}

func TestFavorites_PickerShowsFriendlyNameWhenLoaded(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, []string{"group/alpha", "other/unloaded"}, &saved)
	press(a, "f")

	out := a.renderFavorites()
	if !strings.Contains(out, "Group / alpha") {
		t.Error("expected the loaded project's friendly name")
	}
	// A favorite outside the 50 most recent projects still has to be listed.
	if !strings.Contains(out, "other/unloaded") {
		t.Error("expected an unloaded favorite to be listed by path")
	}
}

func TestFavorites_ProjectPickerMarksStarred(t *testing.T) {
	var saved savedFavorites
	a := newFavApp(t, []string{"group/beta"}, &saved)

	press(a, "P")
	out := a.renderProjectPicker()
	if !strings.Contains(out, "★") {
		t.Error("expected a star marker next to the favorited project")
	}
}

func TestFavorites_NoSaveFuncKeepsStarsInSession(t *testing.T) {
	// A nil SaveFavorites must not panic; stars just do not outlive the session.
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewOverview},
	})
	a.width, a.height = 100, 40
	a.projects = []gitlab.Project{{ID: 1, PathWithNamespace: "g/p"}}

	press(a, "P")
	if cmd := press(a, "f"); cmd != nil {
		t.Error("expected no persist command when SaveFavorites is nil")
	}
	if !a.isFavorite("g/p") {
		t.Error("the star should still apply in memory")
	}
}

func TestViewSwitching_VimKeys(t *testing.T) {
	// h/l move between views the way they moved between panels in v1.
	var saved savedFavorites
	a := newFavApp(t, nil, &saved) // 3 views, starting at index 0

	press(a, "l")
	if a.active != 1 {
		t.Errorf("after l, active = %d, want 1", a.active)
	}
	press(a, "l")
	if a.active != 2 {
		t.Errorf("after a second l, active = %d, want 2", a.active)
	}
	press(a, "l")
	if a.active != 0 {
		t.Errorf("l past the last view should wrap to 0, got %d", a.active)
	}
	press(a, "h")
	if a.active != 2 {
		t.Errorf("h from the first view should wrap to the last, got %d", a.active)
	}
	press(a, "h")
	if a.active != 1 {
		t.Errorf("after h, active = %d, want 1", a.active)
	}
}
