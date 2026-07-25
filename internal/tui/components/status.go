package components

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// StatusColor returns the appropriate color for a pipeline status.
func StatusColor(status string) color.Color {
	switch status {
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
func StatusIcon(status string) string {
	switch status {
	case "success":
		return lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
	case "failed":
		return lipgloss.NewStyle().Foreground(ColorError).Render("✗")
	case "running":
		return lipgloss.NewStyle().Foreground(ColorRunning).Render("◉")
	case "pending":
		return lipgloss.NewStyle().Foreground(ColorPending).Render("○")
	case "canceled":
		return lipgloss.NewStyle().Foreground(ColorCanceled).Render("⊘")
	case "skipped":
		return lipgloss.NewStyle().Foreground(ColorCanceled).Render("⊘")
	case "manual":
		return lipgloss.NewStyle().Foreground(ColorManual).Render("❚❚")
	default:
		return lipgloss.NewStyle().Foreground(ColorSecondary).Render("?")
	}
}

// StatusIconPadded returns StatusIcon padded to 2 display cells so 1- and
// 2-cell glyphs align in a column.
func StatusIconPadded(status string) string {
	icon := StatusIcon(status)
	if status == "manual" { // the only 2-cell glyph
		return icon
	}
	return icon + " "
}
