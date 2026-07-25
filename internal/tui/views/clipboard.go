package views

import (
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"
)

// copyToClipboard puts text on the system clipboard.
//
// OSC 52 is the portable route — it works over SSH and inside tmux, needs no
// external binary — but not every terminal honours it, so a local clipboard
// command is also used when one exists. Copying twice is harmless; reporting a
// copy that silently did nothing would not be.
func copyToClipboard(text string) tea.Cmd {
	cmds := []tea.Cmd{tea.SetClipboard(text)}

	if bin, args := clipboardCommand(); bin != "" {
		if path, err := exec.LookPath(bin); err == nil {
			cmds = append(cmds, func() tea.Msg {
				cmd := exec.Command(path, args...)
				stdin, err := cmd.StdinPipe()
				if err != nil {
					return nil
				}
				if err := cmd.Start(); err != nil {
					return nil
				}
				_, _ = stdin.Write([]byte(text))
				_ = stdin.Close()
				_ = cmd.Wait()
				return nil
			})
		}
	}
	return tea.Batch(cmds...)
}

// clipboardCommand returns the platform's clipboard binary and its arguments.
func clipboardCommand() (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		return "pbcopy", nil
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return "wl-copy", nil
		}
		return "xclip", []string{"-selection", "clipboard"}
	}
	return "", nil
}
