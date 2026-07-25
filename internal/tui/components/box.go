package components

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// RenderBox draws a bordered box with a title embedded in the top border line.
func RenderBox(title string, lines []string, totalWidth, totalHeight int, borderColor, titleColor color.Color) string {
	contentWidth := totalWidth - 4
	contentHeight := totalHeight - 2

	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	bs := lipgloss.NewStyle().Foreground(borderColor)
	ts := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	truncStyle := lipgloss.NewStyle().MaxWidth(contentWidth)

	// Top border: ╭─[1] Title──────────╮
	fill := totalWidth - len(title) - 3
	if fill < 0 {
		fill = 0
	}
	top := bs.Render("╭─") + ts.Render(title) + bs.Render(strings.Repeat("─", fill)+"╮")

	leftB := bs.Render("│")
	rightB := bs.Render("│")

	var result []string
	result = append(result, top)

	for i := 0; i < contentHeight; i++ {
		var line string
		if i < len(lines) {
			line = truncStyle.Render(lines[i])
		}
		visLen := lipgloss.Width(line)
		pad := contentWidth - visLen
		if pad < 0 {
			pad = 0
		}
		result = append(result, leftB+" "+line+strings.Repeat(" ", pad)+" "+rightB)
	}

	bottom := bs.Render("╰" + strings.Repeat("─", totalWidth-2) + "╯")
	result = append(result, bottom)

	return strings.Join(result, "\n")
}
