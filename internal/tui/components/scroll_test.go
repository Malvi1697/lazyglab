package components

import "testing"

func TestScrollOffset_ShortListNeverScrolls(t *testing.T) {
	for cursor := 0; cursor < 5; cursor++ {
		if got := ScrollOffset(0, cursor, 5, 10); got != 0 {
			t.Errorf("cursor %d: offset = %d, want 0 when everything fits", cursor, got)
		}
	}
}

func TestScrollOffset_TopOfListStaysPinned(t *testing.T) {
	// Moving down inside the first window must not scroll while the cursor is
	// still more than a margin away from the bottom edge.
	height, total := 10, 50
	for cursor := 0; cursor <= height-1-ScrollMargin; cursor++ {
		if got := ScrollOffset(0, cursor, total, height); got != 0 {
			t.Errorf("cursor %d: offset = %d, want 0", cursor, got)
		}
	}
}

func TestScrollOffset_ScrollsBeforeReachingTheEdge(t *testing.T) {
	height, total := 10, 50
	// One row past the margin the window starts moving, one row at a time.
	if got := ScrollOffset(0, height-ScrollMargin, total, height); got != 1 {
		t.Errorf("offset = %d, want 1 (scroll starts %d rows early)", got, ScrollMargin)
	}
	if got := ScrollOffset(1, height-ScrollMargin+1, total, height); got != 2 {
		t.Errorf("offset = %d, want 2", got)
	}
}

// TestScrollOffset_CursorIsNotPinnedToTheEdge is the reported bug: after
// scrolling down, moving back up walked the whole viewport instead of moving the
// cursor inside it.
func TestScrollOffset_CursorIsNotPinnedToTheEdge(t *testing.T) {
	height, total := 10, 50
	offset := 20 // rows 20..29 visible, cursor at 26: a margin clear of the edges

	// Moving up walks the cursor through the window; the viewport only follows
	// once the cursor comes within a margin of the top edge.
	for _, step := range []struct{ cursor, wantOffset int }{
		{25, 20},
		{24, 20},
		{23, 20}, // exactly ScrollMargin from the top edge
		{22, 19}, // one row further and the window follows, by one row
		{21, 18},
	} {
		offset = ScrollOffset(offset, step.cursor, total, height)
		if offset != step.wantOffset {
			t.Errorf("cursor %d: offset = %d, want %d", step.cursor, offset, step.wantOffset)
		}
	}
}

// TestScrollOffset_CursorKeepsContextAhead walks the whole list down one row at a
// time and checks the cursor never lands on the last visible row — the symptom
// of the old cursor-derived offset — until the list itself ends.
func TestScrollOffset_CursorKeepsContextAhead(t *testing.T) {
	height, total := 10, 50
	offset := 0
	for cursor := 0; cursor < total; cursor++ {
		offset = ScrollOffset(offset, cursor, total, height)
		lastVisible := offset + height - 1

		if cursor < offset || cursor > lastVisible {
			t.Fatalf("cursor %d outside the window [%d,%d]", cursor, offset, lastVisible)
		}
		// Near the end of the list there is nothing left to keep in view.
		if lastVisible == total-1 {
			continue
		}
		if got := lastVisible - cursor; got < ScrollMargin {
			t.Errorf("cursor %d: only %d rows of context below, want >= %d", cursor, got, ScrollMargin)
		}
	}
	if want := total - height; offset != want {
		t.Errorf("final offset = %d, want %d", offset, want)
	}
}

func TestScrollOffset_LastRowIsReachable(t *testing.T) {
	height, total := 10, 50
	offset := ScrollOffset(0, total-1, total, height)
	if want := total - height; offset != want {
		t.Errorf("offset = %d, want %d so the final row is visible", offset, want)
	}
	// And the cursor really is on the last visible row, not beyond it.
	if last := offset + height - 1; last != total-1 {
		t.Errorf("last visible row = %d, want %d", last, total-1)
	}
}

func TestScrollOffset_FirstRowIsReachable(t *testing.T) {
	if got := ScrollOffset(20, 0, 50, 10); got != 0 {
		t.Errorf("offset = %d, want 0 when the cursor returns to the top", got)
	}
}

func TestScrollOffset_NeverScrollsPastTheEnd(t *testing.T) {
	// A stale offset from a longer list must be clamped, not trusted.
	if got := ScrollOffset(45, 49, 50, 10); got != 40 {
		t.Errorf("offset = %d, want 40 (clamped to total-height)", got)
	}
}

func TestScrollOffset_TinyWindow(t *testing.T) {
	// With less room than the margins need, the cursor must still be visible.
	for _, height := range []int{1, 2, 3, 4} {
		for _, cursor := range []int{0, 5, 49} {
			offset := ScrollOffset(0, cursor, 50, height)
			if cursor < offset || cursor > offset+height-1 {
				t.Errorf("height %d cursor %d: cursor outside window [%d,%d]",
					height, cursor, offset, offset+height-1)
			}
		}
	}
}

func TestScrollOffset_DegenerateInputs(t *testing.T) {
	if got := ScrollOffset(5, 0, 0, 10); got != 0 {
		t.Errorf("empty list: offset = %d, want 0", got)
	}
	if got := ScrollOffset(5, 3, 50, 0); got != 0 {
		t.Errorf("zero height: offset = %d, want 0", got)
	}
}
