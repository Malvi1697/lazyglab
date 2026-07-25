package tui

// Key constants for consistent keybinding references.
const (
	KeyQuit     = "q"
	KeyHelp     = "?"
	KeySearch   = "/"
	KeyRefresh  = "r"
	KeyEnter    = "enter"
	KeyEscape   = "esc"
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
	KeyUp       = "up"
	KeyDown     = "down"
	KeyVimLeft  = "h"
	KeyVimRight = "l"
	KeyHalfDown = "ctrl+d"
	KeyHalfUp   = "ctrl+u"
	KeyBranch   = "b"
	KeyReauth   = "A" // reconnect: change host / replace token
	KeyFavorite = "f" // favorites picker; inside a picker: star/unstar
	KeyNextTab  = "]" // next view, as in lazygit
	KeyPrevTab  = "[" // previous view, as in lazygit
)
