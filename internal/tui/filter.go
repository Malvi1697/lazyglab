package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// listFilter is the incremental "/" search shared by the list overlays. It has
// two stages, following lazygit:
//
//   - typing (active): printable keys extend the query, so "j" searches rather
//     than moving the cursor and "f" is a character rather than a command;
//   - applied (query set, not active): the list stays narrowed but every normal
//     picker key works again, so a searched-for item can be acted on.
//
// Enter moves from typing to applied, which is what makes "search, then star it"
// possible at all.
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

// applied reports whether a query is narrowing the list while text entry is off,
// so the picker's normal keys (star, navigate, select) are available again.
func (f listFilter) applied() bool { return !f.active && f.query != "" }

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
	case KeyEnter:
		// Stop typing but keep the list narrowed, so the picker's own keys (star,
		// navigate, select) apply to the search result.
		f.active = false
		return true, false
	case KeyUp, KeyDown, KeyHalfDown, KeyHalfUp:
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

// hint renders the search prompt: with a cursor while typing, plain once applied.
func (f listFilter) hint() string {
	switch {
	case f.active:
		return "/" + f.query + "█"
	case f.query != "":
		return "/" + f.query
	}
	return ""
}
