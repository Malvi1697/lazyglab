package tui

import (
	"net/url"
	"os/exec"
	"runtime"
)

// openBrowserCmd returns an exec.Cmd to open a URL in the default browser.
// Returns nil if the URL is invalid or uses an unsafe scheme.
func openBrowserCmd(rawURL string) *exec.Cmd {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}

	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", parsed.String())
	case "linux":
		return exec.Command("xdg-open", parsed.String())
	default:
		return exec.Command("open", parsed.String())
	}
}
