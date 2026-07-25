package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// Filter is the incremental "/" search shared by every list in the app — the
// modal pickers and the list views alike. It has two stages, following lazygit:
//
//   - typing (Active): printable keys extend the query, so "j" searches rather
//     than moving the cursor and "f" is a character rather than a command;
//   - applied (Query set, not Active): the list stays narrowed but every normal
//     key works again, so a searched-for item can be acted on.
//
// Enter moves from typing to applied, which is what makes "search, then star it"
// (or "search, then open it") possible at all.
type Filter struct {
	Active bool
	Query  string
}

// Matches reports whether a candidate satisfies the query (case-insensitive
// substring). An empty query matches everything.
func (f Filter) Matches(s string) bool {
	if f.Query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(f.Query))
}

// On reports whether filtering is currently narrowing the list.
func (f Filter) On() bool { return f.Active || f.Query != "" }

// Applied reports whether a query is narrowing the list while text entry is off,
// so the list's normal keys (select, star, drill in) are available again.
func (f Filter) Applied() bool { return !f.Active && f.Query != "" }

// Reset clears the query and leaves filter mode.
func (f *Filter) Reset() {
	f.Active = false
	f.Query = ""
}

// HandleKey applies a key press to the filter while it is active. It reports
// whether the key was consumed, and whether the query changed (so the caller can
// re-clamp its cursor). Enter, the arrow keys and half-page scrolling are
// deliberately not consumed: selecting and navigating keep working while
// filtering. Esc leaves the search, so backspace is the way to edit the query.
func (f *Filter) HandleKey(msg tea.KeyMsg) (consumed, changed bool) {
	if !f.Active {
		return false, false
	}

	switch msg.String() {
	case "enter":
		// Stop typing but keep the list narrowed, so the list's own keys (star,
		// navigate, select) apply to the search result.
		f.Active = false
		return true, false
	case "up", "down", "ctrl+d", "ctrl+u":
		return false, false
	case "esc":
		// Leave the search but keep the list open; a second Esc closes it.
		f.Reset()
		return true, true
	case "backspace":
		f.Query = TrimLastRune(f.Query)
		return true, true
	}

	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.Text != "" {
		f.Query += kp.Text
		return true, true
	}
	return false, false
}

// Paste appends pasted text to the query, since with bracketed paste the content
// arrives as its own message rather than as key presses. It reports whether the
// paste was taken.
func (f *Filter) Paste(content string) bool {
	if !f.Active {
		return false
	}
	f.Query += strings.TrimSpace(content)
	return true
}

// Hint renders the search prompt: with a cursor while typing, plain once applied.
func (f Filter) Hint() string {
	switch {
	case f.Active:
		return "/" + f.Query + "█"
	case f.Query != "":
		return "/" + f.Query
	}
	return ""
}

// TrimLastRune removes the final rune, so multi-byte input deletes cleanly.
func TrimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}
