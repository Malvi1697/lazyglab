package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// pickerApp opens the project picker on two projects.
func pickerApp(t *testing.T) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})
	a.width, a.height = 120, 30
	a.projects = []gitlab.Project{
		{
			ID: 1, Name: "Website", NameWithNamespace: "ACME / Website",
			PathWithNamespace: "acme/website",
			SSHCloneURL:       "git@gitlab.example.com:acme/website.git",
			HTTPCloneURL:      "https://gitlab.example.com/acme/website.git",
		},
		{ID: 2, Name: "billing", PathWithNamespace: "contoso/billing"},
	}
	a.overlay = overlayProject
	return a
}

func TestProjectPicker_YCopiesTheCloneURLs(t *testing.T) {
	// Cloning a project you are looking at meant leaving the app to find its URL.
	a := pickerApp(t)

	if cmd := a.pressKey(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd == nil {
		t.Error("y should have copied something")
	}
	if !strings.Contains(a.pickerStatus, "git@gitlab.example.com:acme/website.git") {
		t.Errorf("note = %q, want the SSH URL", a.pickerStatus)
	}
	// The note is shown where you are looking, inside the picker.
	if box := ansi.Strip(a.renderProjectPicker()); !strings.Contains(box, "git@gitlab.example.com") {
		t.Errorf("picker should say what was copied:\n%s", box)
	}

	a.pressKey(tea.KeyPressMsg{Code: 'Y', Text: "Y", Mod: tea.ModShift})
	if !strings.Contains(a.pickerStatus, "https://gitlab.example.com/acme/website.git") {
		t.Errorf("note = %q, want the HTTPS URL", a.pickerStatus)
	}
}

func TestProjectPicker_ACloneURLTheAPIDidNotSend(t *testing.T) {
	// Saying "copied" when nothing was copied is worse than saying nothing.
	a := pickerApp(t)
	a.projectCursor = 1 // the project without URLs

	if cmd := a.pressKey(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Error("nothing should have been copied")
	}
	if !strings.Contains(a.pickerStatus, "No SSH clone URL") {
		t.Errorf("note = %q, want it to say there is none", a.pickerStatus)
	}
}

func TestProjectPicker_TheCopyKeysDoNotSelectOrClose(t *testing.T) {
	// y and Y are actions on the highlighted row, not a way out of the picker.
	a := pickerApp(t)
	a.pressKey(tea.KeyPressMsg{Code: 'y', Text: "y"})

	if a.overlay != overlayProject {
		t.Error("the picker should stay open")
	}
	if a.ctx.Project != nil {
		t.Error("copying must not select the project")
	}
}

func TestProjectPicker_AStaleNoteIsNotShownNextTime(t *testing.T) {
	// "Copied git@…" left over from the last visit reads as something that just happened.
	a := pickerApp(t)
	a.pressKey(tea.KeyPressMsg{Code: 'y', Text: "y"})
	a.overlay = overlayNone

	a.pressKey(tea.KeyPressMsg{Code: 'P', Text: "P", Mod: tea.ModShift})
	if a.pickerStatus != "" {
		t.Errorf("note = %q, want it cleared when the picker opens", a.pickerStatus)
	}
}

// pressKey routes a key press the way the running program does.
func (a *App) pressKey(msg tea.KeyPressMsg) tea.Cmd {
	_, cmd := a.Update(msg)
	return cmd
}

func TestProjectPicker_OpensOnTheProjectYouAreIn(t *testing.T) {
	// It is the row you most often want something from — its clone URL, or just to see
	// where you are — and it used to mean hunting through a list of hundreds.
	a := pickerApp(t)
	a.overlay = overlayNone
	current := a.projects[1]
	a.ctx.Project = &current

	a.pressKey(tea.KeyPressMsg{Code: 'P', Text: "P", Mod: tea.ModShift})

	if a.overlay != overlayProject {
		t.Fatal("P should open the picker")
	}
	if got := a.visibleProjects()[a.projectCursor].PathWithNamespace; got != "contoso/billing" {
		t.Errorf("cursor is on %q, want the active project", got)
	}
}
