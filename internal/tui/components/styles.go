package components

import (
	"charm.land/lipgloss/v2"
)

// Color palette.
//
// The colours are the terminal's own 16, referenced by index, not fixed hex
// values: index 2 is whatever green your theme calls green. That way lazyglab
// looks like the rest of your terminal instead of imposing a palette on it —
// change your theme and it follows.
//
// Body text sets no colour at all, so it inherits the terminal's foreground.
//
// Two ideas hold the rest together. Text has three weights — default for what you
// read, grey for metadata you scan past, dimmed grey for structure — so the eye
// lands on content rather than on chrome. And the accent is reserved: it marks
// what has focus and nothing else, leaving the status colours as the only other
// saturated things on screen, so they read as information.
var (
	ColorPrimary   = lipgloss.Color("13") // accent: focus, headings, selection
	ColorText      = lipgloss.Color("7")  // light grey: legends and inactive tabs
	ColorSecondary = lipgloss.Color("8")  // muted text: metadata
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

	// FaintStyle is structure: rules, separators, gutters. Grey and dimmed, so it
	// sits a step behind metadata even though both use the same palette entry.
	FaintStyle = lipgloss.NewStyle().Foreground(ColorFaint).Faint(true)

	// SelectedItemStyle is the current row of a list: the accent as a solid band,
	// the way lazygit marks its selection. Grey behind bold text was tasteful and
	// unreadable across a room — where you are should be the first thing you see.
	SelectedItemStyle = lipgloss.NewStyle().Bold(true).
				Background(ColorPrimary).Foreground(ColorOnAccent)

	// AccentChipStyle is the accent as a background, for the tab you are on.
	AccentChipStyle = lipgloss.NewStyle().Bold(true).
			Background(ColorPrimary).Foreground(ColorOnAccent)

	HelpKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)

	// HelpDescStyle is the word beside a key. The legend is read, not scanned past,
	// so it takes the light grey rather than the metadata grey.
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColorText)

	HelpSepStyle = lipgloss.NewStyle().Foreground(ColorFaint)

	ErrorStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
)
