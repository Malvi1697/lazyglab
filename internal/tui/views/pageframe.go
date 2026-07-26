package views

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// pageFrame is a drill-down page's place in the list it was opened from: which of
// how many, and the arrows in the margins for stepping to the neighbours.
//
// Shared by the commit page and the merge-request page, because "the third of
// fifty, with somewhere to go on either side" is the same idea in both.
type pageFrame struct {
	index, total int
	// pageWidth is the last width the page was rendered at, so a selected row can
	// span it and the right arrow sits in the margin rather than against the text.
	pageWidth int
}

// arrowGutter is the width of the margins that carry the ‹ › step arrows.
const arrowGutter = 3

// placeIn records where in a list of total items this page sits.
func (f *pageFrame) placeIn(index, total int) { f.index, f.total = index, total }

// hasPrev and hasNext report whether the page can step to a neighbour.
func (f *pageFrame) hasPrev() bool { return f.total > 0 && f.index > 0 }
func (f *pageFrame) hasNext() bool { return f.total > 0 && f.index < f.total-1 }

// counter is the "3/50" a page's heading carries, or "" when it was not opened
// from a list.
func (f *pageFrame) counter() string {
	if f.total <= 0 {
		return ""
	}
	return fmt.Sprintf("  %d/%d", f.index+1, f.total)
}

// withArrows frames the page with ‹ and › in the left and right margins, level
// with the middle of the page. They are the shape of the keys that move between
// items, put where the movement happens rather than only named in the footer; an
// arrow with nowhere to go is faint.
func (f *pageFrame) withArrows(page string, prev, next bool) string {
	lines := strings.Split(page, "\n")
	middle := len(lines) / 2

	style := func(available bool) lipgloss.Style {
		if available {
			return components.TitleStyle
		}
		return components.FaintStyle
	}

	pad := strings.Repeat(" ", arrowGutter)
	out := make([]string, len(lines))
	for i, line := range lines {
		left, right := pad, pad
		if i == middle {
			left = " " + style(prev).Render("‹") + " "
			right = " " + style(next).Render("›") + " "
		}
		// Pad to the page width, or the right arrow trails the text instead of
		// sitting in the margin.
		out[i] = left + components.PadRight(line, f.pageWidth) + right
	}
	return strings.Join(out, "\n")
}

// stepKey maps a key to a move between the items of a list — the arrows, plus h/l
// for hands that stay on the home row. Lowercase, because this is movement within
// what is open; the uppercase pair moves between the views, which is a bigger step.
func stepKey(key string) (int, bool) {
	switch key {
	case "left", "h":
		return -1, true
	case "right", "l":
		return 1, true
	}
	return 0, false
}

// pageFocus is which box of a drill-down page the keys drive.
type pageFocus int

const (
	focusPage pageFocus = iota
	focusFiles
	focusJobs
	focusNotes
)

// cycleFocus moves the focus between a page's boxes: the text at the top, the
// changed files and the jobs, in that order, wrapping in the given direction.
// Boxes with nothing in them are skipped, so Tab never lands somewhere empty.
func cycleFocus(current pageFocus, step int, hasFiles, hasJobs, hasNotes bool) pageFocus {
	order := []pageFocus{focusPage}
	if hasFiles {
		order = append(order, focusFiles)
	}
	if hasJobs {
		order = append(order, focusJobs)
	}
	if hasNotes {
		order = append(order, focusNotes)
	}
	if len(order) == 1 {
		return current // only the text; nothing to cycle to
	}

	at := 0
	for i, f := range order {
		if f == current {
			at = i
			break
		}
	}
	return order[(at+step+len(order))%len(order)]
}
