package views

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// keySearch starts the incremental search, as in lazygit.
const keySearch = "/"

// TextCapturer is implemented by views that may be receiving typed text — a "/" search
// in progress.
type TextCapturer interface{ CapturingText() bool }

// listSearch is the "/" search a list view owns: the shared filter plus the cursor
// bookkeeping every view repeats around it.
type listSearch struct {
	filter components.Filter
}

// handleKey applies a key press to the search and reports whether it was consumed.
func (s *listSearch) handleKey(msg tea.KeyMsg, cursor *int) bool {
	if consumed, changed := s.filter.HandleKey(msg); consumed {
		if changed {
			*cursor = 0
		}
		return true
	}

	switch msg.String() {
	case keySearch:
		// Resume editing an applied query rather than starting over.
		s.filter.Active = true
		if !s.filter.Applied() {
			*cursor = 0
		}
		return true
	case keyEscape:
		// Esc clears the search first; only then does it mean "back".
		if s.filter.Applied() {
			s.filter.Reset()
			*cursor = 0
			return true
		}
	}
	return false
}

// paste takes pasted text into an open search, as the pickers do.
func (s *listSearch) paste(content string, cursor *int) {
	if s.filter.Paste(content) {
		*cursor = 0
	}
}

// capturing reports whether typed characters are going into the query.
func (s listSearch) capturing() bool { return s.filter.Active }

// on reports whether the search is narrowing the list.
func (s listSearch) on() bool { return s.filter.On() }

// title composes a list heading: the item count, the matched-of-total count while
// searching.
func (s listSearch) title(name string, visible, total int) string {
	if !s.filter.On() {
		return fmt.Sprintf("%s (%d)", name, total)
	}
	return fmt.Sprintf("%s (%d/%d)  %s", name, visible, total,
		components.TitleStyle.Render(s.filter.Hint()))
}

// hint is the footer hint for the search, wording it by stage so the footer says what
// Esc will do next.
func (s listSearch) hint() KeyHint {
	if s.filter.Applied() {
		return KeyHint{"Esc", "Clear search"}
	}
	return KeyHint{"/", "Search"}
}

// filtered returns the entries whose label matches the search, in list order.
func filtered[T any](items []T, f components.Filter, label func(T) string) []T {
	if !f.On() {
		return items
	}
	out := make([]T, 0, len(items))
	for _, it := range items {
		if f.Matches(label(it)) {
			out = append(out, it)
		}
	}
	return out
}

// clampCursor keeps a cursor inside a list of n items.
func clampCursor(cursor, n int) int {
	if cursor >= n {
		cursor = n - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	return cursor
}
