package tui

import tea "charm.land/bubbletea/v2"

// Key constants for consistent keybinding references.
const (
	KeyQuit       = "q"
	KeyHelp       = "?"
	KeySearch     = "/"
	KeyRefresh    = "r"
	KeyEnter      = "enter"
	KeyEscape     = "esc"
	KeyTab        = "tab"
	KeyShiftTab   = "shift+tab"
	KeyUp         = "up"
	KeyDown       = "down"
	KeyVimUp      = "k"
	KeyVimDown    = "j"
	KeyTop        = "g"
	KeyBottom     = "G"
	KeyOpenBrowse = "o"

	// Panel selection
	KeyPanel1 = "1"
	KeyPanel2 = "2"
	KeyPanel3 = "3"
	KeyPanel4 = "4"

	// MR-specific
	KeyApprove = "a"
	KeyMerge   = "m"
	KeyComment = "c"

	// Pipeline-specific
	KeyRetry  = "R"
	KeyCancel = "C"
)

// isNavigateUp checks if the key is an up-navigation key.
func isNavigateUp(msg tea.KeyMsg) bool {
	return msg.String() == KeyUp || msg.String() == KeyVimUp
}

// isNavigateDown checks if the key is a down-navigation key.
func isNavigateDown(msg tea.KeyMsg) bool {
	return msg.String() == KeyDown || msg.String() == KeyVimDown
}
