package components

// NavAction is a cursor movement requested by a key press.
type NavAction int

const (
	NavNone NavAction = iota
	NavUp
	NavDown
	NavTop
	NavBottom
	NavHalfUp
	NavHalfDown
	NavPageUp
	NavPageDown
)

// NavFor maps a key string to its navigation action, or NavNone.
func NavFor(key string) NavAction {
	switch key {
	case "k", "up":
		return NavUp
	case "j", "down":
		return NavDown
	case "g", "<", "home":
		return NavTop
	case "G", ">", "end":
		return NavBottom
	case "ctrl+u":
		return NavHalfUp
	case "ctrl+d":
		return NavHalfDown
	case ",", "pgup":
		return NavPageUp
	case ".", "pgdown":
		return NavPageDown
	}
	return NavNone
}

// ApplyNav returns the new cursor position for an action over a list of total items
// displayed in windowRows rows, clamped to the list.
func ApplyNav(act NavAction, cursor, total, windowRows int) int {
	if total <= 0 {
		return 0
	}

	page := windowRows
	if page < 1 {
		page = 1
	}
	half := page / 2
	if half < 1 {
		half = 1
	}

	switch act {
	case NavUp:
		cursor--
	case NavDown:
		cursor++
	case NavTop:
		cursor = 0
	case NavBottom:
		cursor = total - 1
	case NavHalfUp:
		cursor -= half
	case NavHalfDown:
		cursor += half
	case NavPageUp:
		cursor -= page
	case NavPageDown:
		cursor += page
	}

	if cursor >= total {
		cursor = total - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	return cursor
}
