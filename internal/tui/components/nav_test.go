package components

import "testing"

func TestNavFor(t *testing.T) {
	cases := map[string]NavAction{
		"j": NavDown, "down": NavDown,
		"k": NavUp, "up": NavUp,
		"g": NavTop, "<": NavTop, "home": NavTop,
		"G": NavBottom, ">": NavBottom, "end": NavBottom,
		"ctrl+d": NavHalfDown, "ctrl+u": NavHalfUp,
		".": NavPageDown, "pgdown": NavPageDown,
		",": NavPageUp, "pgup": NavPageUp,
		// Not navigation.
		"enter": NavNone, "f": NavNone, "q": NavNone, "": NavNone,
	}
	for key, want := range cases {
		if got := NavFor(key); got != want {
			t.Errorf("NavFor(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestApplyNav_SingleSteps(t *testing.T) {
	if got := ApplyNav(NavDown, 0, 10, 5); got != 1 {
		t.Errorf("down from 0 = %d, want 1", got)
	}
	if got := ApplyNav(NavUp, 3, 10, 5); got != 2 {
		t.Errorf("up from 3 = %d, want 2", got)
	}
}

func TestApplyNav_Ends(t *testing.T) {
	if got := ApplyNav(NavBottom, 0, 10, 5); got != 9 {
		t.Errorf("bottom = %d, want 9", got)
	}
	if got := ApplyNav(NavTop, 7, 10, 5); got != 0 {
		t.Errorf("top = %d, want 0", got)
	}
}

func TestApplyNav_Pages(t *testing.T) {
	// A page is the window height; half a page is half of it.
	if got := ApplyNav(NavPageDown, 0, 100, 20); got != 20 {
		t.Errorf("page down = %d, want 20", got)
	}
	if got := ApplyNav(NavPageUp, 50, 100, 20); got != 30 {
		t.Errorf("page up = %d, want 30", got)
	}
	if got := ApplyNav(NavHalfDown, 0, 100, 20); got != 10 {
		t.Errorf("half page down = %d, want 10", got)
	}
	if got := ApplyNav(NavHalfUp, 50, 100, 20); got != 40 {
		t.Errorf("half page up = %d, want 40", got)
	}
}

func TestApplyNav_ClampsToList(t *testing.T) {
	if got := ApplyNav(NavUp, 0, 10, 5); got != 0 {
		t.Errorf("up from the top = %d, want 0", got)
	}
	if got := ApplyNav(NavDown, 9, 10, 5); got != 9 {
		t.Errorf("down from the bottom = %d, want 9", got)
	}
	if got := ApplyNav(NavPageDown, 8, 10, 20); got != 9 {
		t.Errorf("page down past the end = %d, want 9", got)
	}
	if got := ApplyNav(NavPageUp, 2, 10, 20); got != 0 {
		t.Errorf("page up past the start = %d, want 0", got)
	}
}

func TestApplyNav_EmptyListAndTinyWindow(t *testing.T) {
	if got := ApplyNav(NavDown, 0, 0, 10); got != 0 {
		t.Errorf("empty list = %d, want 0", got)
	}
	// A degenerate window must still move by at least one row.
	if got := ApplyNav(NavHalfDown, 0, 10, 0); got != 1 {
		t.Errorf("half page with no height = %d, want 1", got)
	}
	if got := ApplyNav(NavPageDown, 0, 10, 0); got != 1 {
		t.Errorf("page with no height = %d, want 1", got)
	}
}

func TestApplyNav_NoneIsANoOp(t *testing.T) {
	if got := ApplyNav(NavNone, 4, 10, 5); got != 4 {
		t.Errorf("NavNone moved the cursor to %d", got)
	}
}
