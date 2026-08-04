package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// newPickerApp builds a shell with a longer project list to search through.
func newPickerApp(t *testing.T) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})
	a.width, a.height = 100, 40
	a.projects = []gitlab.Project{
		{ID: 1, NameWithNamespace: "IDISCGOLF / Idiskgolf", PathWithNamespace: "idiscgolf/idiskgolf"},
		{ID: 2, NameWithNamespace: "OLC Systems / DevOps / Renovate", PathWithNamespace: "olc/devops/renovate"},
		{ID: 3, NameWithNamespace: "NEKO KLIMA / Neko IS", PathWithNamespace: "neko/neko-is"},
		{ID: 4, NameWithNamespace: "OLC Systems / DevOps / traefik", PathWithNamespace: "olc/devops/traefik"},
	}
	return a
}

func TestProjectFilter_SlashNarrowsList(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")

	press(a, "/")
	if !a.projectFilter.Active {
		t.Fatal("/ should start the search")
	}

	for _, r := range "devops" {
		press(a, string(r))
	}
	if a.projectFilter.Query != "devops" {
		t.Fatalf("query = %q, want %q", a.projectFilter.Query, "devops")
	}

	visible := a.visibleProjects()
	if len(visible) != 2 {
		t.Fatalf("visible = %d projects, want 2 (%v)", len(visible), visible)
	}
	for _, p := range visible {
		if !strings.Contains(p.PathWithNamespace, "devops") {
			t.Errorf("unexpected match %q", p.PathWithNamespace)
		}
	}
}

func TestProjectFilter_MatchesFriendlyNameToo(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "KLIMA" {
		press(a, string(r))
	}

	visible := a.visibleProjects()
	if len(visible) != 1 || visible[0].ID != 3 {
		t.Errorf("expected the NEKO KLIMA project by display name, got %v", visible)
	}
}

func TestProjectFilter_CaseInsensitive(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "renoVATE" {
		press(a, string(r))
	}
	if got := a.visibleProjects(); len(got) != 1 || got[0].ID != 2 {
		t.Errorf("expected a case-insensitive match, got %v", got)
	}
}

func TestProjectFilter_TypedKeysDoNotActAsCommands(t *testing.T) {
	// "f" stars and "j" moves; while searching both must be plain characters.
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	press(a, "j")
	press(a, "f")

	if a.projectFilter.Query != "jf" {
		t.Errorf("query = %q, want %q", a.projectFilter.Query, "jf")
	}
	if len(a.favorites) != 0 {
		t.Errorf("f must not star while searching, favorites = %v", a.favorites)
	}
	if a.projectCursor != 0 {
		t.Errorf("j must not move the cursor while searching, cursor = %d", a.projectCursor)
	}
}

func TestProjectFilter_EnterAppliesThenSelects(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "traefik" {
		press(a, string(r))
	}

	// First Enter leaves text entry but keeps the list narrowed.
	if cmd := press(a, "enter"); cmd != nil {
		t.Error("the first Enter should apply the search, not select")
	}
	if !a.projectFilter.Applied() {
		t.Fatal("expected the search to be applied")
	}
	if len(a.visibleProjects()) != 1 {
		t.Fatalf("the list should stay narrowed, got %d", len(a.visibleProjects()))
	}

	// Second Enter opens the match.
	cmd := press(a, "enter")
	if cmd == nil {
		t.Fatal("the second Enter should select the highlighted match")
	}
	msg, ok := cmd().(views.ProjectSelectedMsg)
	if !ok {
		t.Fatalf("expected ProjectSelectedMsg, got %T", cmd())
	}
	if msg.Project.ID != 4 {
		t.Errorf("selected ID = %d, want 4 (traefik)", msg.Project.ID)
	}
	if a.overlay != overlayNone {
		t.Error("selecting should close the picker")
	}
	if a.projectFilter.On() {
		t.Error("the search should be cleared after selecting")
	}
}

// TestProjectFilter_StarAfterApplying covers the flow that matters: search for a
// project, then star it.
func TestProjectFilter_StarAfterApplying(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "traefik" {
		press(a, string(r))
	}
	press(a, "enter") // apply

	press(a, "f")
	if !a.isFavorite("olc/devops/traefik") {
		t.Fatalf("expected the searched project to be starred, favorites = %v", a.favorites)
	}
	if a.overlay != overlayProject {
		t.Error("starring should keep the picker open")
	}
	if !a.projectFilter.Applied() {
		t.Error("starring should not drop the search")
	}
}

func TestProjectFilter_ArrowsNavigateWhileSearching(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "olc" {
		press(a, string(r))
	}
	press(a, "down")

	if a.projectCursor != 1 {
		t.Fatalf("cursor = %d, want 1 (arrows must still navigate)", a.projectCursor)
	}

	press(a, "enter") // apply, cursor kept
	cmd := press(a, "enter")
	msg := cmd().(views.ProjectSelectedMsg)
	if msg.Project.ID != 4 {
		t.Errorf("selected ID = %d, want 4 (second olc match)", msg.Project.ID)
	}
}

func TestProjectFilter_SlashResumesEditing(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "dev" {
		press(a, string(r))
	}
	press(a, "enter") // apply

	press(a, "/") // resume typing where we left off
	if !a.projectFilter.Active {
		t.Fatal("/ should re-enter text entry")
	}
	if a.projectFilter.Query != "dev" {
		t.Errorf("query = %q, want the previous query kept", a.projectFilter.Query)
	}
	for _, r := range "ops/tr" {
		press(a, string(r))
	}
	if got := a.visibleProjects(); len(got) != 1 || got[0].ID != 4 {
		t.Errorf("expected the narrowed match, got %v", got)
	}
}

func TestProjectFilter_EscWithAppliedSearchKeepsPickerOpen(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	press(a, "x")
	press(a, "enter") // apply a query matching nothing

	press(a, "esc")
	if a.projectFilter.On() {
		t.Error("Esc should clear an applied search")
	}
	if a.overlay != overlayProject {
		t.Fatalf("the picker should stay open, overlay = %v", a.overlay)
	}
	press(a, "esc")
	if a.overlay != overlayNone {
		t.Error("a second Esc should close the picker")
	}
}

func TestProjectFilter_BackspaceEditsAndEscLeavesSearch(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "devx" {
		press(a, string(r))
	}
	if len(a.visibleProjects()) != 0 {
		t.Fatal("expected no matches for devx")
	}

	press(a, "backspace")
	if a.projectFilter.Query != "dev" {
		t.Errorf("query = %q, want %q", a.projectFilter.Query, "dev")
	}
	if len(a.visibleProjects()) != 2 {
		t.Errorf("expected 2 matches after backspace, got %d", len(a.visibleProjects()))
	}

	// First Esc leaves the search but keeps the picker open.
	press(a, "esc")
	if a.projectFilter.On() {
		t.Error("Esc should clear the search")
	}
	if a.overlay != overlayProject {
		t.Fatalf("the picker should stay open, overlay = %v", a.overlay)
	}
	if len(a.visibleProjects()) != 4 {
		t.Errorf("the full list should be back, got %d", len(a.visibleProjects()))
	}

	// Second Esc closes it.
	press(a, "esc")
	if a.overlay != overlayNone {
		t.Error("a second Esc should close the picker")
	}
}

func TestProjectFilter_ResetWhenReopened(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	press(a, "x")
	press(a, "esc") // leaves search
	press(a, "esc") // closes picker
	press(a, "P")

	if a.projectFilter.On() {
		t.Error("reopening the picker must start unfiltered")
	}
}

func TestProjectFilter_NoMatchIsExplained(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	for _, r := range "zzz" {
		press(a, string(r))
	}

	out := a.renderProjectPicker()
	if !strings.Contains(out, "No project matches") {
		t.Error("expected the empty result to be explained")
	}
	if cmd := press(a, "enter"); cmd != nil {
		t.Error("Enter with no match must do nothing")
	}
}

func TestProjectFilter_TitleShowsCounts(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	if out := a.renderProjectPicker(); !strings.Contains(out, "Select Project (4)") {
		t.Error("expected the total project count in the title")
	}

	press(a, "/")
	for _, r := range "olc" {
		press(a, string(r))
	}
	if out := a.renderProjectPicker(); !strings.Contains(out, "(2/4)") {
		t.Error("expected matched/total counts in the title while searching")
	}
}

func TestProjectFilter_AcceptsPaste(t *testing.T) {
	a := newPickerApp(t)
	press(a, "P")
	press(a, "/")
	a.Update(tea.PasteMsg{Content: "renovate\n"})

	if a.projectFilter.Query != "renovate" {
		t.Errorf("query = %q, want the pasted text without the newline", a.projectFilter.Query)
	}
}

func TestBranchFilter_NarrowsList(t *testing.T) {
	a := newPickerApp(t)
	a.Update(views.BranchesLoadedMsg{Branches: []gitlab.Branch{
		{Name: "main", Default: true},
		{Name: "feature/login"},
		{Name: "feature/signup"},
		{Name: "hotfix/crash"},
	}})
	if a.overlay != overlayBranch {
		t.Fatalf("expected the branch picker, got %v", a.overlay)
	}

	press(a, "/")
	for _, r := range "feature" {
		press(a, string(r))
	}
	if got := a.visibleBranches(); len(got) != 2 {
		t.Fatalf("visible = %d branches, want 2 (%v)", len(got), got)
	}

	press(a, "enter") // apply
	cmd := press(a, "enter")
	msg, ok := cmd().(views.BranchSelectedMsg)
	if !ok {
		t.Fatalf("expected BranchSelectedMsg, got %T", cmd())
	}
	if msg.Branch.Name != "feature/login" {
		t.Errorf("selected %q, want feature/login", msg.Branch.Name)
	}
}
