package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// savedProject records what a stub SaveLastProjectFunc was asked to persist.
type savedProject struct {
	host, path string
	calls      int
	err        error
}

func newSessionApp(t *testing.T, lastProject, detectedPath string, saved *savedProject) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		DetectedPath: detectedPath,
		ViewIDs:      []views.ViewID{views.ViewOverview},
		LastProject:  lastProject,
		SaveLastProject: func(host, path string) error {
			saved.host = host
			saved.path = path
			saved.calls++
			return saved.err
		},
	})
	a.width, a.height = 100, 40
	a.projects = []gitlab.Project{
		{ID: 7, NameWithNamespace: "Group / resumed", PathWithNamespace: "group/resumed"},
	}
	return a
}

func TestSession_SelectingProjectIsRemembered(t *testing.T) {
	var saved savedProject
	a := newSessionApp(t, "", "", &saved)

	_, cmd := a.Update(views.ProjectSelectedMsg{Project: a.projects[0]})
	if cmd == nil {
		t.Fatal("expected commands after selecting a project")
	}
	cmd() // runs Focus + the persist command

	if saved.calls == 0 {
		t.Fatal("expected the project to be recorded")
	}
	if saved.host != "h" || saved.path != "group/resumed" {
		t.Errorf("recorded host=%q path=%q", saved.host, saved.path)
	}
}

func TestSession_ReselectingSameProjectWritesOnce(t *testing.T) {
	var saved savedProject
	a := newSessionApp(t, "group/resumed", "", &saved)

	// The project is already the remembered one, so no write is needed.
	_, cmd := a.Update(views.ProjectSelectedMsg{Project: a.projects[0]})
	if cmd != nil {
		cmd()
	}
	if saved.calls != 0 {
		t.Errorf("SaveLastProject calls = %d, want 0 for an unchanged project", saved.calls)
	}
}

func TestSession_FailedWriteIsReported(t *testing.T) {
	saved := savedProject{err: errors.New("read-only fs")}
	a := newSessionApp(t, "", "", &saved)

	a.Update(lastProjectSavedMsg{err: saved.err})
	if !a.statusIsErr || !strings.Contains(a.statusText, "read-only fs") {
		t.Errorf("expected the failure in the status bar, got %q (isErr=%v)", a.statusText, a.statusIsErr)
	}
}

func TestSession_InitRestoresLastProject(t *testing.T) {
	var saved savedProject
	a := newSessionApp(t, "group/resumed", "", &saved)

	cmd := a.Init()
	if cmd == nil {
		t.Fatal("Init should return commands")
	}
	// The remembered project resolves from the loaded list without an API call.
	if !producesProjectSelection(cmd, "group/resumed") {
		t.Error("Init should restore the remembered project")
	}
}

func TestSession_GitRemoteDetectionWinsOverRemembered(t *testing.T) {
	// Being inside a repo is the stronger signal about what the user wants.
	var saved savedProject
	a := newSessionApp(t, "group/resumed", "group/detected", &saved)

	if producesProjectSelection(a.Init(), "group/resumed") {
		t.Error("a detected project must take precedence over the remembered one")
	}
}

func TestSession_NoRememberedProjectDoesNothing(t *testing.T) {
	var saved savedProject
	a := newSessionApp(t, "", "", &saved)
	if producesProjectSelection(a.Init(), "group/resumed") {
		t.Error("nothing should be restored without a remembered project")
	}
}

// producesProjectSelection runs cmd — typically a tea.Batch, whose members may
// themselves be batches — and reports whether any of it selects the given path.
func producesProjectSelection(cmd tea.Cmd, path string) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			if producesProjectSelection(c, path) {
				return true
			}
		}
		return false
	case views.ProjectSelectedMsg:
		return msg.Project.PathWithNamespace == path
	default:
		return false
	}
}
