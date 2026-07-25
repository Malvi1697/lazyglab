package components

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// SelectionGutter is the width of the marker column every list row carries, so
// the current row can be pointed at without shifting the text beside it.
const SelectionGutter = 2

// RenderPanel draws a section of the body: a heading, a thin rule that runs to
// the edge, then the content.
//
// Panels deliberately have no frame. Four bordered rectangles on one screen read
// as chrome, and the borders carry no information that the heading does not — so
// only the floating overlays keep a box, where the border is what separates the
// dialog from the screen behind it. Focus is shown by colouring the heading and
// its rule with the accent instead.
func RenderPanel(title string, lines []string, width, height int, focused bool) string {
	if width < 1 {
		width = 1
	}
	if height < 2 {
		height = 2
	}

	// Headings are bold, not accented. The accent marks one thing — where you are
	// — and spending it on every heading as well leaves three coloured, bold rows
	// stacked at the top of the screen, all shouting equally.
	titleStyle, ruleStyle := MutedTitleStyle, FaintStyle
	if focused {
		titleStyle = lipgloss.NewStyle().Bold(true)
	}

	head := titleStyle.Render(title)
	if fill := width - lipgloss.Width(head) - 1; fill > 0 {
		head += " " + ruleStyle.Render(strings.Repeat("─", fill))
	}

	out := make([]string, 0, height)
	out = append(out, head)

	contentRows := height - 1
	for i := 0; i < contentRows; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out = append(out, lipgloss.NewStyle().MaxWidth(width).Render(line))
	}
	return strings.Join(out, "\n")
}

// VRule is a vertical separator for side-by-side panels, one column wide.
func VRule(height int) string {
	if height < 1 {
		height = 1
	}
	rule := FaintStyle.Render("│")
	lines := make([]string, height)
	for i := range lines {
		lines[i] = rule
	}
	return strings.Join(lines, "\n")
}

// SelectRow renders one list row: the current row gets the accent bar in the
// gutter and a tinted background, everything else keeps the gutter empty so the
// text stays in the same column.
func SelectRow(text string, width int, selected bool) string {
	gutter := strings.Repeat(" ", SelectionGutter)
	if selected {
		gutter = TitleStyle.Render("▌") + strings.Repeat(" ", SelectionGutter-1)
	}

	textWidth := width - SelectionGutter
	if textWidth < 1 {
		textWidth = 1
	}
	body := Truncate(text, textWidth)
	if selected {
		// The row is repainted as one span, so styling inside it cannot fight the
		// background.
		body = SelectedItemStyle.Render(PadRight(StripStyles(body), textWidth))
	}
	return gutter + body
}
