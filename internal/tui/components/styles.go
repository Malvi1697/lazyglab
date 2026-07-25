package components

import (
	"charm.land/lipgloss/v2"
)

// Color palette.
var (
	ColorPrimary   = lipgloss.Color("#7B68EE") // medium slate blue
	ColorSecondary = lipgloss.Color("#A0A0A0")
	ColorSuccess   = lipgloss.Color("#50C878")
	ColorError     = lipgloss.Color("#FF6B6B")
	ColorWarning   = lipgloss.Color("#FFD93D")
	ColorRunning   = lipgloss.Color("#6CB4EE")
	ColorPending   = lipgloss.Color("#FFD93D")
	ColorCanceled  = lipgloss.Color("#808080")
	ColorManual    = lipgloss.Color("#00CED1")
	ColorDraft     = lipgloss.Color("#808080")
)

// Panel styles.
var (
	ActiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	InactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSecondary)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ColorPrimary)

	NormalItemStyle = lipgloss.NewStyle()

	StatusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#333333")).
			Foreground(lipgloss.Color("#CCCCCC"))

	HelpKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00CFCF")) // cyan, like lazygit

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	HelpSepStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)
)
