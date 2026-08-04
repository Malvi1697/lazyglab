package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// updateFoundMsg carries the result of the startup version check.
type updateFoundMsg struct{ version string }

// updateCheckCmd asks GitHub for the newest release, off the render path.
func (a *App) updateCheckCmd() tea.Cmd {
	if a.checkUpdate == nil {
		return nil
	}
	check := a.checkUpdate
	return func() tea.Msg { return updateFoundMsg{version: check()} }
}

// updateNote is the notice at the right of the tabs row: which version is out and the
// command that installs it.
func (a *App) updateNote() string {
	if a.updateVersion == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(components.ColorWarning).Render("▲ v"+a.updateVersion+" available") +
		components.MutedStyle.Render(" · lazyglab update")
}
