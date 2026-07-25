package components

// ScrollMargin is how many rows stay visible past the cursor before a list
// starts scrolling — vim's scrolloff. It means the row you are on is never the
// very edge of the window, so there is always context in the direction you are
// heading.
const ScrollMargin = 3

// ScrollOffset returns the index of the first visible row for a list of total
// items shown in a window of height rows, given the previous offset and the
// cursor's position.
//
// The previous offset is honoured whenever the cursor is comfortably inside the
// window: the viewport moves only when the cursor comes within ScrollMargin of
// an edge. Deriving the offset from the cursor alone (offset = cursor-height+1)
// pins the cursor to the last row once the list is scrolled, so moving back up
// scrolls immediately instead of walking the cursor up through the window.
func ScrollOffset(offset, cursor, total, height int) int {
	if height <= 0 || total <= 0 {
		return 0
	}
	// Everything fits: nothing to scroll.
	if total <= height {
		return 0
	}

	// A window too short for margins on both sides keeps the cursor centred-ish
	// instead of refusing to scroll.
	margin := ScrollMargin
	if maxMargin := (height - 1) / 2; margin > maxMargin {
		margin = maxMargin
	}

	if cursor-margin < offset {
		offset = cursor - margin
	}
	if cursor+margin > offset+height-1 {
		offset = cursor + margin - height + 1
	}

	// Never scroll past the ends: the first and last rows must both be reachable.
	if maxOffset := total - height; offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return offset
}
