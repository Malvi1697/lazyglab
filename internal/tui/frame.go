package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// renderContextBar renders the top bar: the active branch and project on the
// left, then the latest status message, and the refresh note at the far right.
// Full width.
func renderContextBar(width int, ctx *views.Context, status string, statusIsErr bool, refresh string) string {
	branch := ""
	project := "no project"
	if ctx != nil {
		if ctx.Project != nil {
			project = ctx.Project.NameWithNamespace
			branch = ctx.Project.DefaultBranch
		}
		if ctx.Branch != nil {
			branch = ctx.Branch.Name
		}
	}

	// The branch is what you check first, so it is bold; the project name is
	// context and stays quieter. Neither takes the accent — that belongs to the
	// active tab on the row below, which would otherwise have to compete with it.
	left := " "
	if branch != "" {
		left += lipgloss.NewStyle().Bold(true).Render(" "+branch) + "  "
	}
	left += components.MutedStyle.Render(project)

	// The refresh note keeps the far right; the status takes whatever is left
	// between it and the project name. A long status (an API error, typically) must
	// not wrap onto the tabs row, so it is truncated rather than allowed to grow.
	room := width - lipgloss.Width(left) - lipgloss.Width(refresh) - 4
	right := components.Truncate(status, room)
	if statusIsErr {
		right = components.ErrorStyle.Render(right)
	} else {
		right = components.MutedStyle.Render(right)
	}
	if refresh != "" {
		if right != "" {
			right += "  "
		}
		right += refresh
	}

	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + right + " "
	return lipgloss.NewStyle().Width(width).Render(bar)
}

// renderTabs renders the numbered view tabs, highlighting the active one. Tabs
// are numbered by position (1-based). Full width.
func renderTabs(width int, viewIDs []views.ViewID, active int, titles []string) string {
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(components.ColorPrimary)
	inactiveStyle := components.MutedStyle

	var parts []string
	for i := range viewIDs {
		title := ""
		if i < len(titles) {
			title = titles[i]
		}
		num := i + 1
		if i == active {
			parts = append(parts, activeStyle.Render(sprintTab("[", num, "] ", title)))
		} else {
			parts = append(parts, inactiveStyle.Render(sprintTab(" ", num, " ", title)))
		}
	}

	// A rule under the tabs separates the frame from the body, which no longer
	// has borders of its own.
	bar := " " + strings.Join(parts, components.HelpSepStyle.Render(" · "))
	return lipgloss.NewStyle().Width(width).Render(bar)
}

// sprintTab formats a tab label as pre + number + mid + title.
func sprintTab(pre string, num int, mid, title string) string {
	return pre + itoa(num) + mid + title
}

// itoa is a tiny int-to-string for small tab numbers (avoids fmt import churn).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// renderFooter renders the global key hints joined with the active view's hints,
// styled like the v1 keybind bar. Full width.
func renderFooter(width int, globalHints, viewHints []views.KeyHint) string {
	var parts []string
	appendHints := func(hints []views.KeyHint) {
		for _, h := range hints {
			parts = append(parts, components.HelpKeyStyle.Render(h.Key)+" "+components.HelpDescStyle.Render(h.Desc))
		}
	}
	appendHints(globalHints)
	appendHints(viewHints)

	sep := components.HelpSepStyle.Render(" · ")
	bar := " " + strings.Join(parts, sep)
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(bar)
}
