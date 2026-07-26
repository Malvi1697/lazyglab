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
	KeyPrevView = "H" // previous view: the big tabs, so lowercase h stays free
	KeyNextView = "L" // next view
	KeyBranch   = "b"
	KeyReauth   = "A" // reconnect: change host / replace token
	KeyFavorite = "f" // favorites picker; inside a picker: star/unstar
	KeyNextTab  = "]" // next view, as in lazygit
	KeyPrevTab  = "[" // previous view, as in lazygit
)
