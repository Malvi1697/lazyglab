package views

import (
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// foldBox is a view's second box: the dashboard's README, a to-do's reason, a
// pipeline's stages. Three views had the same field, the same "t", and the same
// two-line hint; this is that, once.
//
// Folded is session state, not a preference: every launch starts on the list.
type foldBox struct {
	name   string // what the hint calls it, lower case: "readme", "stages"
	folded bool
}

func (b *foldBox) toggle() { b.folded = !b.folded }

func (b *foldBox) open() bool { return !b.folded }

// hint names the direction the key goes, so it never offers to hide something that
// is already hidden.
func (b *foldBox) hint() KeyHint {
	if b.folded {
		return KeyHint{"t", "Show " + b.name}
	}
	return KeyHint{"t", "Hide " + b.name}
}

// splitBody stacks a list above a box, with a blank row between them so the two
// halves do not run together. bottom is asked for its height first: it gets what it
// asks for, up to half the page, and the list keeps the rest. Below the floor for
// the list the box is dropped entirely — the rows are what the page is for.
func splitBody(width, height, wanted int, list, box func(width, height int) string) string {
	const (
		gap      = 1
		listMin  = 5
		boxFloor = 4
	)
	bottom := min(max(wanted, boxFloor), height/2)
	top := height - gap - bottom
	if top < listMin {
		return list(width, height)
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		list(width, top),
		"",
		box(width, bottom),
	)
}

// panelBox renders lines into a body panel, for the box half of splitBody.
func panelBox(title string, lines []string) func(width, height int) string {
	return func(width, height int) string {
		return components.RenderPanel(title, lines, width, height, false)
	}
}
