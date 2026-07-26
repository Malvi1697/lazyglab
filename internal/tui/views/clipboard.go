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

// copyRef and copyLink are the two halves of one rule, kept together so every
// view spells it the same way: lowercase y copies the identifier — the thing you
// would type into a commit message or a chat — and uppercase Y copies the link
// you would send someone. What was copied is said in the status bar, because a
// clipboard write is otherwise entirely invisible.
func copyRef(ref string) tea.Cmd {
	if ref == "" {
		return statusCmd("Nothing to copy here", true)
	}
	return tea.Batch(copyToClipboard(ref), statusCmd("Copied "+ref, false))
}

func copyLink(what, url string) tea.Cmd {
	if url == "" {
		return statusCmd("No link to copy here", true)
	}
	label := "the link"
	if what != "" {
		label = "the link to " + what
	}
	return tea.Batch(copyToClipboard(url), statusCmd("Copied "+label, false))
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
