package tui

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// newTestApp builds a minimal shell with a stub reconfigure function.
func newTestApp(t *testing.T, reconfigure ReconfigureFunc) *App {
	t.Helper()
	a := NewApp(Options{
		Clients:      map[string]*gitlab.Client{"gitlab.example.com": nil},
		HostNames:    []string{"gitlab.example.com"},
		DetectedHost: "gitlab.example.com",
		ViewIDs:      []views.ViewID{views.ViewOverview},
		Reconfigure:  reconfigure,
	})
	a.width, a.height = 100, 40
	return a
}

// press feeds a key to the model the way Bubble Tea would.
func press(a *App, key string) tea.Cmd {
	var msg tea.KeyMsg
	switch key {
	case "enter", "esc", "tab", "shift+tab", "backspace", "up", "down", "ctrl+u":
		msg = tea.KeyPressMsg{Code: keyCodeFor(key), Mod: modFor(key)}
	default:
		msg = tea.KeyPressMsg{Code: rune(key[0]), Text: key}
	}
	_, cmd := a.Update(msg)
	return cmd
}

func keyCodeFor(key string) rune {
	switch key {
	case "enter":
		return tea.KeyEnter
	case "esc":
		return tea.KeyEscape
	case "tab", "shift+tab":
		return tea.KeyTab
	case "backspace":
		return tea.KeyBackspace
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "ctrl+u":
		return 'u'
	}
	return 0
}

func modFor(key string) tea.KeyMod {
	switch key {
	case "shift+tab":
		return tea.ModShift
	case "ctrl+u":
		return tea.ModCtrl
	}
	return 0
}

// authErr mirrors what the API client produces for a revoked token, including
// the Request that ErrorResponse.Error() reads when formatting itself.
func authErr() error {
	u, _ := url.Parse("https://gitlab.example.com/api/v4/projects")
	return &gogitlab.ErrorResponse{
		Response: &http.Response{
			StatusCode: http.StatusUnauthorized,
			Request:    &http.Request{Method: http.MethodGet, URL: u},
		},
		Message: "{error: invalid_token}, {error_description: Token was revoked. You have to re-authorize from the user.}",
		Body:    []byte(`{"error":"invalid_token","error_description":"Token was revoked."}`),
	}
}

func TestAuthError_OpensReconfigOverlay(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })

	a.Update(views.ProjectsLoadedMsg{Err: authErr()})

	if a.overlay != overlayReconfig {
		t.Fatalf("expected the reconfig overlay to open, got overlay %v", a.overlay)
	}
	if a.reconfig == nil {
		t.Fatal("expected reconfig state to exist")
	}
	if a.reconfig.host != "gitlab.example.com" {
		t.Errorf("host should be prefilled with the active host, got %q", a.reconfig.host)
	}
	if a.reconfig.field != fieldToken {
		t.Error("focus should start on the token field when the host is known")
	}
	if a.reconfig.reason == "" {
		t.Error("expected the auth failure reason to be shown")
	}
}

func TestAuthError_FromAnyViewOpensOverlay(t *testing.T) {
	// A pipeline load failing must prompt too, not just the shell's own load.
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	a.Update(views.PipelinesLoadedMsg{Err: authErr()})
	if a.overlay != overlayReconfig {
		t.Fatalf("expected overlay from a view's failed load, got %v", a.overlay)
	}
}

func TestNonAuthError_DoesNotOpenOverlay(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	a.Update(views.ProjectsLoadedMsg{Err: errors.New("connection refused")})
	if a.overlay != overlayNone {
		t.Errorf("a network error must not prompt for re-authentication, got overlay %v", a.overlay)
	}
}

func TestDismissedPrompt_DoesNotReopenOnNextFailure(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	a.Update(views.ProjectsLoadedMsg{Err: authErr()})
	press(a, "esc")

	if a.overlay != overlayNone {
		t.Fatalf("Esc should close the overlay, got %v", a.overlay)
	}

	// An auto-refresh keeps failing; the overlay must stay closed.
	a.Update(views.ProjectsLoadedMsg{Err: authErr()})
	if a.overlay != overlayNone {
		t.Error("a dismissed prompt must not reopen on the next failed load")
	}

	// ...but asking for it explicitly works.
	press(a, "A")
	if a.overlay != overlayReconfig {
		t.Error("A should reopen the reconfig overlay")
	}
}

func TestReconfigForm_TypingAndFieldSwitching(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	press(a, "A")

	// Focus starts on the token field.
	press(a, "g")
	press(a, "l")
	press(a, "p")
	if a.reconfig.token != "glp" {
		t.Errorf("token = %q, want %q", a.reconfig.token, "glp")
	}

	press(a, "backspace")
	if a.reconfig.token != "gl" {
		t.Errorf("after backspace token = %q, want %q", a.reconfig.token, "gl")
	}

	// Tab moves to the host field and typing edits that instead.
	press(a, "tab")
	if a.reconfig.field != fieldHost {
		t.Fatal("Tab should move focus to the host field")
	}
	press(a, "ctrl+u")
	if a.reconfig.host != "" {
		t.Errorf("ctrl+u should clear the field, got %q", a.reconfig.host)
	}
	press(a, "x")
	if a.reconfig.host != "x" || a.reconfig.token != "gl" {
		t.Errorf("host = %q, token = %q; typing must only touch the focused field", a.reconfig.host, a.reconfig.token)
	}
}

func TestReconfigForm_AcceptsPaste(t *testing.T) {
	// Tokens are pasted, and bracketed paste arrives as PasteMsg, not key presses.
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	press(a, "A")

	a.Update(tea.PasteMsg{Content: "glpat-pasted-token"})
	if a.reconfig.token != "glpat-pasted-token" {
		t.Errorf("token = %q, want the pasted value", a.reconfig.token)
	}

	// A copied token often carries a trailing newline; it must not survive.
	press(a, "ctrl+u")
	a.Update(tea.PasteMsg{Content: "  glpat-trailing\n"})
	if a.reconfig.token != "glpat-trailing" {
		t.Errorf("token = %q, want whitespace stripped", a.reconfig.token)
	}

	// Paste lands in whichever field has focus.
	press(a, "tab")
	press(a, "ctrl+u")
	a.Update(tea.PasteMsg{Content: "gitlab.pasted.cz"})
	if a.reconfig.host != "gitlab.pasted.cz" {
		t.Errorf("host = %q, want the pasted value", a.reconfig.host)
	}
	if a.reconfig.token != "glpat-trailing" {
		t.Errorf("token = %q, paste must not touch the unfocused field", a.reconfig.token)
	}
}

func TestReconfigForm_PasteIgnoredWhenBusy(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	press(a, "A")
	a.reconfig.token = "glpat-x"
	press(a, "enter") // starts validating

	a.Update(tea.PasteMsg{Content: "late-paste"})
	if a.reconfig.token != "glpat-x" {
		t.Errorf("token = %q, want it unchanged while validating", a.reconfig.token)
	}
}

func TestReconfigForm_QuitKeyIsTypedNotObeyed(t *testing.T) {
	// "q" quits globally; inside the form it must be a character.
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	press(a, "A")
	if cmd := press(a, "q"); cmd != nil {
		t.Error("q must not trigger a command (like Quit) while the form is open")
	}
	if a.reconfig.token != "q" {
		t.Errorf("token = %q, want %q", a.reconfig.token, "q")
	}
}

func TestReconfigSubmit_RequiresBothFields(t *testing.T) {
	called := false
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) {
		called = true
		return nil, "", nil
	})
	press(a, "A")

	// Enter with an empty token moves to the token field instead of submitting.
	a.reconfig.field = fieldHost
	press(a, "enter")
	if called {
		t.Fatal("submitting with an empty token must not call reconfigure")
	}
	if a.reconfig.field != fieldToken {
		t.Error("Enter on the host field should move focus to the token")
	}

	// An empty host is reported inline.
	a.reconfig.host = ""
	a.reconfig.token = "glpat-x"
	press(a, "enter")
	if called {
		t.Fatal("submitting with an empty host must not call reconfigure")
	}
	if a.reconfig.err == "" {
		t.Error("expected an inline error about the missing host")
	}
}

func TestReconfigSubmit_SuccessSwapsClientAndClosesOverlay(t *testing.T) {
	client, err := gitlab.NewClient("t", "https://gitlab.example.com/api/v4", "gitlab.example.com")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	a := newTestApp(t, func(host, token string) (*gitlab.Client, string, error) {
		if host != "gitlab.example.com" || token != "glpat-new" {
			t.Errorf("reconfigure got host=%q token=%q", host, token)
		}
		return client, "jan", nil
	})
	press(a, "A")
	a.reconfig.token = "glpat-new"

	cmd := press(a, "enter")
	if cmd == nil {
		t.Fatal("expected a command performing the reconfiguration")
	}
	if !a.reconfig.busy {
		t.Error("the form should be marked busy while validating")
	}

	// Run the command and feed its message back, as the runtime would.
	a.Update(cmd())

	if a.overlay != overlayNone {
		t.Errorf("a successful reconfiguration should close the overlay, got %v", a.overlay)
	}
	if a.ctx.Client != client {
		t.Error("the shared context should use the new client")
	}
	if a.clients["gitlab.example.com"] != client {
		t.Error("the client map should be updated for the host")
	}
	if a.statusIsErr {
		t.Errorf("status should not be an error after success: %q", a.statusText)
	}
}

func TestReconfigSubmit_FailureKeepsFormOpenWithError(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) {
		return nil, "", errors.New("authentication failed: invalid or expired token")
	})
	press(a, "A")
	a.reconfig.token = "glpat-wrong"

	cmd := press(a, "enter")
	a.Update(cmd())

	if a.overlay != overlayReconfig {
		t.Fatalf("a failed attempt must keep the form open, got overlay %v", a.overlay)
	}
	if a.reconfig.busy {
		t.Error("busy should be cleared after the attempt finished")
	}
	if a.reconfig.err == "" {
		t.Error("expected the failure to be shown inline")
	}
}

func TestRenderReconfig_MasksTokenAndFits(t *testing.T) {
	a := newTestApp(t, func(string, string) (*gitlab.Client, string, error) { return nil, "", nil })
	press(a, "A")
	a.reconfig.token = "glpat-supersecret"

	out := a.renderReconfig()
	if out == "" {
		t.Fatal("expected the overlay to render")
	}
	if strings.Contains(out, "glpat-supersecret") {
		t.Error("the token must never be rendered in clear text")
	}
	if !strings.Contains(out, "•") {
		t.Error("expected the token to be masked with bullets")
	}
	if !strings.Contains(out, "gitlab.example.com") {
		t.Error("expected the prefilled host to be visible")
	}
}
