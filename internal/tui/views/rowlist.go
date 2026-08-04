package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// rowList is the state a list view keeps: the rows it holds, where the cursor is,
// where the visible window starts, and the search that narrows it.
//
// Views embed it, which leaves each view with only what makes it different: what it
// fetches, what its rows say, and what its keys do. The cursor always indexes the
// searched rows, never the full list.
type rowList[T any] struct {
	items  []T
	cursor int
	scroll int
	search listSearch

	// match is the text the search compares a row against.
	match func(T) string
}

func (l *rowList[T]) setItems(items []T) {
	l.items = items
	l.clamp()
}

func (l *rowList[T]) visible() []T {
	return filtered(l.items, l.search.filter, l.match)
}

func (l *rowList[T]) selected() *T {
	visible := l.visible()
	if l.cursor < 0 || l.cursor >= len(visible) {
		return nil
	}
	return &visible[l.cursor]
}

func (l *rowList[T]) clamp() {
	l.cursor = clampCursor(l.cursor, len(l.visible()))
}

func (l *rowList[T]) capturing() bool { return l.search.capturing() }

func (l *rowList[T]) paste(content string) { l.search.paste(content, &l.cursor) }

// navigate takes the keys that belong to the list itself — the search and the
// movement — and reports whether it used one, so a view's own keys only see what is
// left.
func (l *rowList[T]) navigate(msg tea.KeyMsg, height int) bool {
	if l.search.handleKey(msg, &l.cursor) {
		return true
	}
	if act := components.NavFor(msg.String()); act != components.NavNone {
		l.cursor = components.ApplyNav(act, l.cursor, len(l.visible()), listRows(height))
		return true
	}
	return false
}

// box renders the list as a body panel. Rows are described rather than drawn — one
// listRow per item — so every list in the app is laid out by the same columns,
// measured over the whole list.
func (l *rowList[T]) box(width, height int, name string, row func(T) listRow, focused bool) string {
	visible := l.visible()
	rows := make([]listRow, len(visible))
	for i, item := range visible {
		rows[i] = row(item)
	}
	rowWidth := width - components.SelectionGutter
	cols := measureColumns(rows, rowWidth)

	return renderRowsBox(width, height,
		l.search.title(name, len(visible), len(l.items)),
		len(visible),
		func(i int) string { return renderListRow(rows[i], cols, rowWidth) },
		cursorWhen(focused, l.cursor), &l.scroll)
}
