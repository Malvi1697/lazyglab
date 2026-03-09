package tui

import (
	"os/exec"
	"runtime"
)

// openBrowserCmd returns an exec.Cmd to open a URL in the default browser.
func openBrowserCmd(url string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url)
	case "linux":
		return exec.Command("xdg-open", url)
	default:
		return exec.Command("open", url)
	}
}
