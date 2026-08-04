package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// ReconfigureFunc validates a host/token pair, persists it and returns a ready client
// plus the authenticated username.
type ReconfigureFunc func(host, token string) (*gitlab.Client, string, error)

// reconfigField identifies which input of the re-authentication form has focus.
type reconfigField int

const (
	fieldHost reconfigField = iota
	fieldToken
)

// reconfigState is the re-authentication form's state.
type reconfigState struct {
	host  string
	token string
	field reconfigField
	// reason is the auth error that popped the overlay; empty when opened by hand.
	reason string
	// err is a validation or save failure to show inline, keeping the form open.
	err string
	// busy is true while a validate-and-save round trip is in flight.
	busy bool
}

// reconfigDoneMsg carries the result of a re-authentication attempt.
type reconfigDoneMsg struct {
	host     string
	client   *gitlab.Client
	username string
	err      error
}

// openReconfig opens the re-authentication overlay with the current host prefilled.
func (a *App) openReconfig(reason string) {
	host := a.activeHost
	if host == "" && len(a.hostNames) > 0 {
		host = a.hostNames[0]
	}
	r := &reconfigState{host: host, field: fieldToken, reason: reason}
	if host == "" {
		r.field = fieldHost
	}
	a.reconfig = r
	a.overlay = overlayReconfig
}

// closeReconfig dismisses the overlay.
func (a *App) closeReconfig() {
	a.reconfig = nil
	a.overlay = overlayNone
	a.authPromptDismissed = true
	a.setStatus("Re-authentication canceled — press A to retry", false)
}

// handleReconfigKey edits the form: printable keys type into the focused input,
// Tab/arrows move between them, Enter submits and Esc cancels.
func (a *App) handleReconfigKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	r := a.reconfig
	if r == nil {
		a.overlay = overlayNone
		return a, nil
	}
	// Ignore input while validating so a second Enter cannot submit twice.
	if r.busy {
		return a, nil
	}

	switch msg.String() {
	case KeyEscape:
		a.closeReconfig()
		return a, nil
	case KeyTab:
		if r.field == fieldHost {
			r.field = fieldToken
		} else {
			r.field = fieldHost
		}
		return a, nil
	case KeyDown:
		r.field = fieldToken
		return a, nil
	case KeyShiftTab, KeyUp:
		r.field = fieldHost
		return a, nil
	case KeyEnter:
		// Enter on the host field moves to the still-empty token instead of submitting a form
		// that cannot succeed.
		if r.field == fieldHost && strings.TrimSpace(r.token) == "" {
			r.field = fieldToken
			return a, nil
		}
		return a, a.submitReconfig()
	case "backspace":
		r.setField(components.TrimLastRune(r.currentField()))
		return a, nil
	case "ctrl+u":
		r.setField("")
		return a, nil
	}

	// Printable characters type into the focused input.
	if kp, ok := msg.(tea.KeyPressMsg); ok && kp.Text != "" {
		r.setField(r.currentField() + kp.Text)
	}
	return a, nil
}

// pasteIntoReconfig appends pasted text to the focused input.
func (a *App) pasteIntoReconfig(content string) {
	r := a.reconfig
	if r == nil || r.busy {
		return
	}
	cleaned := strings.Join(strings.Fields(content), "")
	if cleaned == "" {
		return
	}
	r.setField(r.currentField() + cleaned)
}

// currentField returns the focused input's value.
func (r *reconfigState) currentField() string {
	if r.field == fieldHost {
		return r.host
	}
	return r.token
}

// setField writes the focused input's value and clears any stale inline error.
func (r *reconfigState) setField(v string) {
	if r.field == fieldHost {
		r.host = v
	} else {
		r.token = v
	}
	r.err = ""
}

// submitReconfig validates the form and, if complete, runs the injected reconfigure
// function off the UI goroutine.
func (a *App) submitReconfig() tea.Cmd {
	r := a.reconfig
	host := strings.TrimSpace(r.host)
	token := strings.TrimSpace(r.token)

	switch {
	case host == "":
		r.err = "host is required"
		r.field = fieldHost
		return nil
	case token == "":
		r.err = "token is required"
		r.field = fieldToken
		return nil
	case a.reconfigure == nil:
		r.err = "re-authentication is unavailable"
		return nil
	}

	r.busy = true
	r.err = ""
	reconfigure := a.reconfigure
	return func() tea.Msg {
		client, username, err := reconfigure(host, token)
		return reconfigDoneMsg{host: host, client: client, username: username, err: err}
	}
}

// applyReconfig swaps in the client for the freshly authenticated host and closes the
// overlay.
func (a *App) applyReconfig(host string, client *gitlab.Client) {
	if a.clients == nil {
		a.clients = make(map[string]*gitlab.Client)
	}
	if _, known := a.clients[host]; !known {
		a.hostNames = append(a.hostNames, host)
	}
	a.clients[host] = client

	if host != a.activeHost {
		a.ctx.Project = nil
		a.ctx.Branch = nil
		a.projects = nil
		a.projectCursor = 0
	}
	a.activeHost = host
	a.ctx.Client = client

	a.reconfig = nil
	a.overlay = overlayNone
	// A new token can be revoked later too, so allow prompting again.
	a.authPromptDismissed = false
}

// renderReconfig draws the re-authentication form.
func (a *App) renderReconfig() string {
	r := a.reconfig
	if r == nil {
		return ""
	}

	boxWidth, _ := a.overlayBoxSize()
	if boxWidth < 44 {
		boxWidth = 44
	}
	if boxWidth > a.width-2 {
		boxWidth = a.width - 2
	}
	innerWidth := boxWidth - 4
	if innerWidth < 10 {
		innerWidth = 10
	}

	var lines []string

	if r.reason != "" {
		lines = append(lines, components.ErrorStyle.Render("Authentication failed"))
		for _, l := range components.WrapLine(r.reason, innerWidth) {
			lines = append(lines, components.HelpDescStyle.Render(l))
		}
		lines = append(lines, "")
	}

	lines = append(lines, components.HelpDescStyle.Render("GitLab host"))
	lines = append(lines, a.renderReconfigInput(r.host, innerWidth, r.field == fieldHost))
	lines = append(lines, "")
	lines = append(lines, components.HelpDescStyle.Render("Personal access token (scope: api)"))
	lines = append(lines, a.renderReconfigInput(strings.Repeat("•", len([]rune(r.token))), innerWidth, r.field == fieldToken))

	if host := strings.TrimSpace(r.host); host != "" {
		lines = append(lines, "")
		lines = append(lines, components.HelpDescStyle.Render(
			components.Truncate("Create one at https://"+host+"/-/user_settings/personal_access_tokens", innerWidth)))
	}

	lines = append(lines, "")
	switch {
	case r.busy:
		lines = append(lines, components.HelpKeyStyle.Render("Validating token…"))
	case r.err != "":
		for _, l := range components.WrapLine(r.err, innerWidth) {
			lines = append(lines, components.ErrorStyle.Render(l))
		}
	default:
		lines = append(lines, hintBar(innerWidth,
			views.KeyHint{Key: "Tab", Desc: "Next field"}, views.KeyHint{Key: "Enter", Desc: "Save"},
			views.KeyHint{Key: "Esc", Desc: "Cancel"}))
	}

	return components.RenderBox("Reconnect to GitLab", lines, boxWidth, len(lines)+2,
		components.ColorWarning, components.ColorWarning)
}

// renderReconfigInput draws one input row, marking the focused one with "›".
func (a *App) renderReconfigInput(value string, width int, focused bool) string {
	marker := "  "
	if focused {
		marker = components.HelpKeyStyle.Render("› ")
	}
	text := components.Truncate(value, width-4)
	if focused {
		text += "█"
	}
	field := components.PadRight(text, width-2)
	if focused {
		return marker + components.SelectedItemStyle.Render(field)
	}
	return marker + field
}

// authFailureReason condenses an auth error into one line for the overlay.
func authFailureReason(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// GitLab's body is the useful part; the "METHOD url: status" prefix is noise.
	if i := strings.Index(msg, "{error"); i >= 0 {
		msg = msg[i:]
	}
	msg = strings.NewReplacer("{", "", "}", "", "error_description: ", "", "error: ", "").Replace(msg)
	return strings.TrimSpace(msg)
}
