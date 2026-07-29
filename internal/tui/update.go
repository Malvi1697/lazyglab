package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// updateFoundMsg carries the result of the startup version check. An empty
// version means there is nothing newer, which is the usual answer.
type updateFoundMsg struct{ version string }

// updateCheckCmd asks GitHub for the newest release, off the render path. It runs
// once per session: a release that appears while the app is open can wait for the
// next launch, and a request per refresh tick to learn the same thing would be
// waste of exactly the kind the refresh budget exists to prevent.
func (a *App) updateCheckCmd() tea.Cmd {
	if a.checkUpdate == nil {
		return nil
	}
	check := a.checkUpdate
	return func() tea.Msg { return updateFoundMsg{version: check()} }
}

// updateNote is the notice at the right of the tabs row: which version is out and
// the command that installs it. It names the command because a notice that only
// says "update available" leaves the reader to go looking.
func (a *App) updateNote() string {
	if a.updateVersion == "" {
		return ""
	}
	return lipgloss.NewStyle().Foreground(components.ColorWarning).Render("▲ v"+a.updateVersion+" available") +
		components.MutedStyle.Render(" · lazyglab update")
}
