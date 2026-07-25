package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// listFilter is the incremental "/" search shared by the list overlays. While it
// is active, printable keys extend the query instead of acting as commands, so
// typing "j" searches rather than moving the cursor.
type listFilter struct {
	active bool
	query  string
}

// matches reports whether a candidate satisfies the query (case-insensitive
// substring). An empty query matches everything.
func (f listFilter) matches(s string) bool {
	if f.query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(f.query))
}

// on reports whether filtering is currently narrowing the list.
func (f listFilter) on() bool { return f.active || f.query != "" }

// reset clears the query and leaves filter mode.
func (f *listFilter) reset() {
	f.active = false
	f.query = ""
}

// handleKey applies a key press to the filter while it is active. It reports
// whether the key was consumed, and whether the query changed (so the caller can
// re-clamp its cursor). Enter, the arrow keys and half-page scrolling are
// deliberately not consumed: selecting and navigating keep working while
// filtering. Esc leaves the search, so backspace is the way to edit the query.
func (f *listFilter) handleKey(msg tea.KeyMsg) (consumed, changed bool) {
	if !f.active {
		return false, false
	}

	switch msg.String() {
	case KeyEnter, KeyUp, KeyDown, KeyHalfDown, KeyHalfUp:
		return false, false
	case KeyEscape:
		// Leave the search but keep the picker open; a second Esc closes it.
		f.reset()
		return true, true
	case "backspace":
		f.query = trimLastRune(f.query)
		return true, true
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.Text != "" {
		f.query += kp.Text
		return true, true
	}
	return false, false
}

// hint renders the search prompt shown in place of the picker's key hints.
func (f listFilter) hint() string {
	if !f.active {
		return ""
	}
	return "/" + f.query + "█"
}
