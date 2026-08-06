package components

import (
	"charm.land/lipgloss/v2"
)

// Color palette.
var (
	ColorPrimary   = lipgloss.Color("13") // accent: focus, headings, selection
	ColorText      = lipgloss.Color("7")  // light grey: everything you read past the content
	ColorSecondary = lipgloss.Color("7")  // metadata reads at the same weight as a legend
	ColorFaint     = lipgloss.Color("8")  // structure; dimmed further via Faint
	ColorOnAccent  = lipgloss.Color("0")  // text on top of the accent, always the darkest
	ColorSuccess   = lipgloss.Color("2")
	ColorError     = lipgloss.Color("1")
	ColorWarning   = lipgloss.Color("3")
	ColorRunning   = lipgloss.Color("4")
	ColorPending   = lipgloss.Color("3")
	ColorCanceled  = lipgloss.Color("8")
	ColorManual    = lipgloss.Color("6")
	ColorDraft     = lipgloss.Color("8")
)

// Text styles.
var (
	// TitleStyle is a section heading.
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// MutedTitleStyle is the heading of a section that does not have focus.
	MutedTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)

	// BodyStyle is ordinary content: the terminal's own foreground.
	BodyStyle = lipgloss.NewStyle()

	// MutedStyle is metadata beside content: timestamps, authors, counts.
	MutedStyle = lipgloss.NewStyle().Foreground(ColorSecondary)

	// FaintStyle is structure: rules, separators, gutters.
	FaintStyle = lipgloss.NewStyle().Foreground(ColorFaint).Faint(true)

	// SelectedItemStyle is the current row of a list: the accent as a solid band, the way
	// lazygit marks its selection.
	SelectedItemStyle = lipgloss.NewStyle().Bold(true).
				Background(ColorPrimary).Foreground(ColorOnAccent)

	// AccentChipStyle is the accent as a background, for the tab you are on.
	AccentChipStyle = lipgloss.NewStyle().Bold(true).
			Background(ColorPrimary).Foreground(ColorOnAccent)

	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// HelpDescStyle is the word beside a key.
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColorText)

	HelpSepStyle = lipgloss.NewStyle().Foreground(ColorFaint)

	ErrorStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
)
