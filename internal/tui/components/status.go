package components

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// StatusWarning is the pseudo-status for a pipeline that succeeded while an
// allowed-to-fail job failed — GitLab calls it "passed with warnings" and shows
// it in orange rather than green, because a plain ✓ would hide a real failure.
const StatusWarning = "success-with-warnings"

// StatusColor returns the appropriate color for a pipeline status.
func StatusColor(status string) color.Color {
	switch status {
	case StatusWarning:
		return ColorWarning
	case "success":
		return ColorSuccess
	case "failed":
		return ColorError
	case "running":
		return ColorRunning
	case "pending", "waiting_for_resource":
		return ColorPending
	case "canceled", "skipped":
		return ColorCanceled
	case "manual":
		return ColorManual
	default:
		return ColorSecondary
	}
}

// StatusIcon returns a colored icon for a pipeline status.
//
// The glyphs are solid rather than line-drawn, and bold, because at terminal font
// sizes a thin ✓ or ✗ disappears into the text beside it — the icon is the thing
// you scan a list by, so it has to carry. Shape distinguishes the states as well
// as colour: a filled circle finished, a half-filled one still running, a hollow
// one waiting, a triangle you can act on.
func StatusIcon(status string) string {
	icon := func(c color.Color, glyph string) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(glyph)
	}

	switch status {
	case StatusWarning:
		return icon(ColorWarning, "▲")
	case "success":
		return icon(ColorSuccess, "●")
	case "failed":
		return icon(ColorError, "●")
	case "running":
		return icon(ColorRunning, "◐")
	case "pending", "created", "waiting_for_resource", "preparing", "scheduled":
		return icon(ColorPending, "○")
	case "canceled", "canceling":
		return icon(ColorCanceled, "⊗")
	case "skipped":
		return icon(ColorCanceled, "◌")
	case "manual":
		return icon(ColorManual, "▶")
	default:
		return icon(ColorSecondary, "·")
	}
}

// StatusIconPadded returns StatusIcon padded to 2 display cells so 1- and
// 2-cell glyphs align in a column.
func StatusIconPadded(status string) string {
	// Every glyph is one cell wide now, so the column is a fixed two.
	return StatusIcon(status) + " "
}
