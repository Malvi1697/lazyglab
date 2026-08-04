package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// newLongPickerApp builds a shell with far more projects than fit on screen.
func newLongPickerApp(t *testing.T) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"h": nil},
		HostNames:    []string{"h"},
		DetectedHost: "h",
		ViewIDs:      []views.ViewID{views.ViewDashboard},
	})
	a.width, a.height = 100, 40
	for i := 0; i < 60; i++ {
		a.projects = append(a.projects, gitlab.Project{
			ID:                i + 1,
			NameWithNamespace: fmt.Sprintf("Group / project-%02d", i),
			PathWithNamespace: fmt.Sprintf("group/project-%02d", i),
		})
	}
	return a
}

// visibleProjectNumbers returns the project indices currently rendered.
func visibleProjectNumbers(t *testing.T, a *App) []int {
	t.Helper()
	var out []int
	for _, line := range strings.Split(a.renderProjectPicker(), "\n") {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(stripANSI(line)), "│ Group / project-%02d", &n); err == nil {
			out = append(out, n)
			continue
		}
		// The selected row is padded/styled differently; match on the name only.
		if idx := strings.Index(stripANSI(line), "project-"); idx >= 0 {
			if _, err := fmt.Sscanf(stripANSI(line)[idx:], "project-%02d", &n); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && r == 'm':
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// TestPickerScroll_CursorMovesInsideWindow is the reported bug: the cursor used to
// stick to the bottom row.
func TestPickerScroll_CursorMovesInsideWindow(t *testing.T) {
	a := newLongPickerApp(t)
	press(a, "P")

	// Walk down well past the first screenful.
	for i := 0; i < 30; i++ {
		press(a, "j")
		a.renderProjectPicker()
	}
	scrolledOffset := a.projectScroll
	if scrolledOffset == 0 {
		t.Fatal("expected the list to have scrolled")
	}

	// The cursor must not be on the last visible row: context has to remain below.
	rows := visibleProjectNumbers(t, a)
	if len(rows) == 0 {
		t.Fatal("no rows rendered")
	}
	lastVisible := rows[len(rows)-1]
	if a.projectCursor >= lastVisible {
		t.Errorf("cursor at %d is at/past the last visible row %d", a.projectCursor, lastVisible)
	}

	// Moving up a couple of rows must not move the viewport at all.
	press(a, "k")
	a.renderProjectPicker()
	press(a, "k")
	a.renderProjectPicker()
	if a.projectScroll != scrolledOffset {
		t.Errorf("viewport moved (%d -> %d) while the cursor still had room",
			scrolledOffset, a.projectScroll)
	}
	if got := a.projectCursor; got != 28 {
		t.Errorf("cursor = %d, want 28 (30 down, 2 back up)", got)
	}
}

func TestPickerScroll_KeepsMarginWalkingDown(t *testing.T) {
	a := newLongPickerApp(t)
	press(a, "P")

	for i := 0; i < len(a.projects)-1; i++ {
		press(a, "j")
		rows := visibleProjectNumbers(t, a)
		if len(rows) == 0 {
			t.Fatal("no rows rendered")
		}
		first, last := rows[0], rows[len(rows)-1]
		if a.projectCursor < first || a.projectCursor > last {
			t.Fatalf("cursor %d outside the rendered window [%d,%d]", a.projectCursor, first, last)
		}
		// Except at the very end of the list, keep context below the cursor.
		if last == len(a.projects)-1 {
			continue
		}
		if got := last - a.projectCursor; got < components.ScrollMargin {
			t.Fatalf("cursor %d: %d rows below, want >= %d", a.projectCursor, got, components.ScrollMargin)
		}
	}
}

func TestPickerScroll_TopAndBottomAreReachable(t *testing.T) {
	a := newLongPickerApp(t)
	press(a, "P")

	press(a, "G")
	visibleProjectNumbers(t, a) // render once so the offset follows the cursor
	rows := visibleProjectNumbers(t, a)
	if rows[len(rows)-1] != len(a.projects)-1 {
		t.Errorf("last row = %d, want the final project %d", rows[len(rows)-1], len(a.projects)-1)
	}

	press(a, "g")
	visibleProjectNumbers(t, a)
	rows = visibleProjectNumbers(t, a)
	if rows[0] != 0 {
		t.Errorf("first row = %d, want 0 after g", rows[0])
	}
	if a.projectScroll != 0 {
		t.Errorf("offset = %d, want 0 at the top", a.projectScroll)
	}
}

func TestFavoritesScroll_LongListScrolls(t *testing.T) {
	a := newLongPickerApp(t)
	for _, p := range a.projects {
		a.favorites = append(a.favorites, p.PathWithNamespace)
	}

	press(a, "f")
	for i := 0; i < 40; i++ {
		press(a, "j")
		a.renderFavorites()
	}
	if a.favoriteScroll == 0 {
		t.Error("expected the favorites picker to scroll for a long list")
	}
	out := a.renderFavorites()
	if !strings.Contains(stripANSI(out), fmt.Sprintf("project-%02d", a.favoriteCursor)) {
		t.Error("the highlighted favorite must be visible")
	}
}
