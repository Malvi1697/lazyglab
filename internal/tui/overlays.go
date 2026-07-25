package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// overlayKind identifies which modal overlay (if any) is currently active.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayProject
	overlayBranch
	overlayHelp
	overlayConfirm
)

// confirmAction holds state for a pending confirmation dialog. The action runs
// only if the user confirms.
type confirmAction struct {
	prompt string
	action tea.Cmd
}

// confirm opens a confirmation dialog for a destructive action.
func (a *App) confirm(prompt string, action tea.Cmd) {
	a.pendingConfirm = &confirmAction{prompt: prompt, action: action}
	a.overlay = overlayConfirm
}

// mergeOverlay composites a centered overlay box on top of the frame,
// line by line, so the frame shows through where the overlay is blank.
func (a *App) mergeOverlay(frame, box string) string {
	placed := lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
	bgLines := strings.Split(frame, "\n")
	ovLines := strings.Split(placed, "\n")
	for i, ovLine := range ovLines {
		if strings.TrimRight(ovLine, " ") != "" && i < len(bgLines) {
			bgLines[i] = ovLine
		}
	}
	return strings.Join(bgLines, "\n")
}

// ============================================================================
// Confirm dialog
// ============================================================================

func (a *App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", KeyEnter:
		var action tea.Cmd
		if a.pendingConfirm != nil {
			action = a.pendingConfirm.action
		}
		a.pendingConfirm = nil
		a.overlay = overlayNone
		return a, action
	default:
		a.pendingConfirm = nil
		a.overlay = overlayNone
		a.statusText = "Canceled"
		a.statusIsErr = false
		return a, nil
	}
}

func (a *App) renderConfirm() string {
	if a.pendingConfirm == nil {
		return ""
	}
	prompt := a.pendingConfirm.prompt
	hint := "y/Enter: confirm  n/Esc: cancel"

	innerWidth := len(prompt)
	if len(hint) > innerWidth {
		innerWidth = len(hint)
	}
	innerWidth += 4
	boxWidth := innerWidth + 4

	lines := []string{
		"",
		"  " + lipgloss.NewStyle().Bold(true).Foreground(components.ColorWarning).Render(prompt),
		"",
		"  " + components.HelpDescStyle.Render(hint),
		"",
	}
	return components.RenderBox("Confirm", lines, boxWidth, len(lines)+2, components.ColorWarning, components.ColorWarning)
}

// ============================================================================
// Branch picker
// ============================================================================

func (a *App) handleBranchPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch {
	case key == KeyEscape || key == KeyQuit:
		a.overlay = overlayNone
		return a, nil
	case isNavigateUp(msg):
		if a.branchCursor > 0 {
			a.branchCursor--
		}
		return a, nil
	case isNavigateDown(msg):
		if a.branchCursor < len(a.branches)-1 {
			a.branchCursor++
		}
		return a, nil
	case key == KeyTop:
		a.branchCursor = 0
		return a, nil
	case key == KeyBottom:
		a.branchCursor = len(a.branches) - 1
		if a.branchCursor < 0 {
			a.branchCursor = 0
		}
		return a, nil
	case key == KeyEnter:
		if a.branchCursor >= 0 && a.branchCursor < len(a.branches) {
			branch := a.branches[a.branchCursor]
			a.overlay = overlayNone
			return a, func() tea.Msg { return views.BranchSelectedMsg{Branch: branch} }
		}
		return a, nil
	}
	return a, nil
}

func (a *App) renderBranchPicker() string {
	boxWidth, boxHeight := a.overlayBoxSize()
	maxVisible := boxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}

	var lines []string
	if len(a.branches) == 0 {
		lines = append(lines, "No branches found")
	} else {
		scrollOffset := 0
		if a.branchCursor >= maxVisible {
			scrollOffset = a.branchCursor - maxVisible + 1
		}
		for i := scrollOffset; i < len(a.branches) && len(lines) < maxVisible; i++ {
			b := a.branches[i]
			marker := "  "
			if b.Default {
				marker = "* "
			} else if b.Protected {
				marker = "P "
			}
			activity := ""
			if !b.LastActivity.IsZero() {
				activity = components.HelpDescStyle.Render(" " + util.TimeAgo(b.LastActivity))
			}
			if i == a.branchCursor {
				line := components.SelectedItemStyle.Render(marker + b.Name)
				if activity != "" {
					line += activity
				}
				lines = append(lines, line)
			} else {
				lines = append(lines, fmt.Sprintf("%s%s%s", marker, b.Name, activity))
			}
		}
	}
	lines = append(lines, "", components.HelpDescStyle.Render("Enter: select  Esc: cancel  j/k: navigate"))

	return components.RenderBox("Select Branch", lines, boxWidth, boxHeight, components.ColorPrimary, components.ColorPrimary)
}

// ============================================================================
// Project picker
// ============================================================================

func (a *App) handleProjectPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch {
	case key == KeyEscape || key == KeyQuit:
		a.overlay = overlayNone
		return a, nil
	case isNavigateUp(msg):
		if a.projectCursor > 0 {
			a.projectCursor--
		}
		return a, nil
	case isNavigateDown(msg):
		if a.projectCursor < len(a.projects)-1 {
			a.projectCursor++
		}
		return a, nil
	case key == KeyTop:
		a.projectCursor = 0
		return a, nil
	case key == KeyBottom:
		a.projectCursor = len(a.projects) - 1
		if a.projectCursor < 0 {
			a.projectCursor = 0
		}
		return a, nil
	case key == KeyEnter:
		if a.projectCursor >= 0 && a.projectCursor < len(a.projects) {
			proj := a.projects[a.projectCursor]
			a.overlay = overlayNone
			return a, func() tea.Msg { return views.ProjectSelectedMsg{Project: proj} }
		}
		return a, nil
	}
	return a, nil
}

func (a *App) renderProjectPicker() string {
	boxWidth, boxHeight := a.overlayBoxSize()
	maxVisible := boxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}
	innerWidth := boxWidth - 4

	var lines []string
	if len(a.projects) == 0 {
		lines = append(lines, "No projects found")
	} else {
		scrollOffset := 0
		if a.projectCursor >= maxVisible {
			scrollOffset = a.projectCursor - maxVisible + 1
		}
		for i := scrollOffset; i < len(a.projects) && len(lines) < maxVisible; i++ {
			p := a.projects[i]
			marker := "  "
			if a.ctx != nil && a.ctx.Project != nil && a.ctx.Project.ID == p.ID {
				marker = "* "
			}
			label := components.Truncate(marker+p.NameWithNamespace, innerWidth)
			if i == a.projectCursor {
				label = components.SelectedItemStyle.Render(components.PadRight(label, innerWidth))
			}
			lines = append(lines, label)
		}
	}
	lines = append(lines, "", components.HelpDescStyle.Render("Enter: select  Esc: cancel  j/k: navigate"))

	return components.RenderBox("Select Project", lines, boxWidth, boxHeight, components.ColorPrimary, components.ColorPrimary)
}

// overlayBoxSize returns a reasonable centered-box size for list overlays.
func (a *App) overlayBoxSize() (int, int) {
	boxWidth := a.width * 6 / 10
	if boxWidth > 70 {
		boxWidth = 70
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	if boxWidth > a.width-2 {
		boxWidth = a.width - 2
	}
	boxHeight := a.height * 7 / 10
	if boxHeight < 6 {
		boxHeight = 6
	}
	if boxHeight > a.height-2 {
		boxHeight = a.height - 2
	}
	return boxWidth, boxHeight
}

// ============================================================================
// Help overlay
// ============================================================================

func (a *App) renderHelp() string {
	help := []struct{ key, desc string }{
		{"q / Ctrl+c", "Quit"},
		{"?", "Toggle help"},
		{"1-9", "Switch view"},
		{"Tab / S-Tab", "Next / prev view"},
		{"p", "Project switcher"},
		{"b", "Branch filter"},
		{"r", "Refresh view"},
		{"j / k", "Navigate down / up"},
		{"g / G", "Top / bottom"},
		{"Ctrl+d/u", "Half page down / up"},
		{"Enter", "Select / drill in"},
		{"Esc", "Back / cancel"},
		{"o", "Open in browser"},
		{"", ""},
		{"--- Pipelines ---", ""},
		{"Enter", "View jobs / log"},
		{"R", "Retry"},
		{"C", "Cancel"},
		{"", ""},
		{"--- Merge Requests ---", ""},
		{"a", "Approve"},
		{"m", "Merge"},
		{"", ""},
		{"--- Issues ---", ""},
		{"c", "Close / reopen"},
	}

	var lines []string
	lines = append(lines, components.TitleStyle.Render("Keybindings"))
	lines = append(lines, "")
	for _, h := range help {
		switch {
		case h.key == "":
			lines = append(lines, "")
		case strings.HasPrefix(h.key, "---"):
			lines = append(lines, components.HelpDescStyle.Render(h.key))
		default:
			lines = append(lines, fmt.Sprintf("  %s  %s",
				components.HelpKeyStyle.Width(14).Render(h.key),
				components.HelpDescStyle.Render(h.desc),
			))
		}
	}
	lines = append(lines, "")
	lines = append(lines, components.HelpDescStyle.Render("Press any key to close"))

	// Left-align the block as a unit, then let View center it.
	return lipgloss.NewStyle().Align(lipgloss.Left).Render(strings.Join(lines, "\n"))
}
