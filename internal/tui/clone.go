package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// copyCloneURL copies what you would clone the highlighted project with: "y" the
// SSH URL you would type, "Y" the HTTPS one you would send someone — the same
// division of labour the two copy keys have everywhere else.
//
// GitLab sends both URLs with every project, so this costs no request. The note
// goes inside the picker rather than into the status bar behind it: that is where
// you are looking, and a clipboard write is otherwise invisible.
func (a *App) copyCloneURL(p gitlab.Project, https bool) tea.Cmd {
	url, kind := p.SSHCloneURL, "SSH"
	if https {
		url, kind = p.HTTPCloneURL, "HTTPS"
	}
	if url == "" {
		a.pickerStatus = "No " + kind + " clone URL for " + p.PathWithNamespace
		return nil
	}
	a.pickerStatus = "Copied " + url
	return views.CopyToClipboard(url)
}
