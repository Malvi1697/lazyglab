package components

import (
	"charm.land/lipgloss/v2"
)

// Color palette.
//
// Two ideas hold it together. Text has three weights — bright for what you read,
// muted for metadata you scan past, faint for structure — so the eye lands on
// content rather than on chrome. And the accent is reserved: it marks what has
// focus and nothing else, which leaves the status colours (green, red, amber) as
// the only other saturated things on screen, so they read as information.
var (
	ColorPrimary   = lipgloss.Color("#8B80F9") // accent: focus, headings, selection
	ColorSecondary = lipgloss.Color("#8B92A5") // muted text: metadata
	ColorFaint     = lipgloss.Color("#3E4352") // structure: rules, separators
	ColorText      = lipgloss.Color("#D7DAE0") // body text, softer than pure white
	ColorSuccess   = lipgloss.Color("#5FD68A")
	ColorError     = lipgloss.Color("#F0716F")
	ColorWarning   = lipgloss.Color("#E8B84B")
	ColorRunning   = lipgloss.Color("#66B2F0")
	ColorPending   = lipgloss.Color("#E8B84B")
	ColorCanceled  = lipgloss.Color("#6B7280")
	ColorManual    = lipgloss.Color("#4FD1C5")
	ColorDraft     = lipgloss.Color("#6B7280")

	// colorSelectedBg is the current row: a tint of the accent rather than the
	// accent itself, so a highlighted line does not shout across the width of a
	// terminal. The accent returns as a bar in the row's gutter.
	colorSelectedBg = lipgloss.Color("#2E2B4A")
)

// Text styles.
var (
	// TitleStyle is a section heading.
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// MutedTitleStyle is the heading of a section that does not have focus.
	MutedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	// BodyStyle is ordinary content.
	BodyStyle = lipgloss.NewStyle().Foreground(ColorText)

	// MutedStyle is metadata beside content: timestamps, authors, counts.
	MutedStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	// FaintStyle is structure: rules, separators, gutters.
	FaintStyle = lipgloss.NewStyle().Foreground(ColorFaint)

	// SelectedItemStyle is the current row of a list.
	SelectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F2F3F5")).
				Background(colorSelectedBg)

	NormalItemStyle = lipgloss.NewStyle().Foreground(ColorText)

	// StatusBarStyle is the context line at the top of the screen. No background:
	// a full-width grey band competes with the content under it.
	StatusBarStyle = lipgloss.NewStyle().Foreground(ColorText)

	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	HelpDescStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	HelpSepStyle = lipgloss.NewStyle().Foreground(ColorFaint)

	ErrorStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
)

// Border styles, used by the floating overlays. Panels inside the body use
// headings and rules instead — see RenderPanel.
var (
	ActiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	InactiveBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorFaint)
)
