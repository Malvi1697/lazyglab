package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Truncate shortens a string to maxLen.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return ansi.Truncate(s, maxLen, "")
	}
	return ansi.Truncate(s, maxLen-3, "") + "..."
}

// PadRight pads s with spaces to the given display width (no-op if already wider).
func PadRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// WrapLine wraps a single (ANSI-stripped) line to width w, breaking on spaces
// when possible and falling back to a hard rune break for overlong words.
func WrapLine(line string, w int) []string {
	if w < 1 {
		w = 1
	}
	if lipgloss.Width(line) <= w {
		return []string{line}
	}
	var out []string
	var cur []rune
	curW := 0
	flush := func() {
		out = append(out, string(cur))
		cur = cur[:0]
		curW = 0
	}
	for _, word := range strings.Split(line, " ") {
		wl := len([]rune(word))
		// Hard-break a single word longer than the width.
		for wl > w {
			if curW > 0 {
				flush()
			}
			r := []rune(word)
			out = append(out, string(r[:w]))
			word = string(r[w:])
			wl = len([]rune(word))
		}
		space := 0
		if curW > 0 {
			space = 1
		}
		if curW+space+wl > w {
			flush()
			space = 0
		}
		if space == 1 {
			cur = append(cur, ' ')
			curW++
		}
		cur = append(cur, []rune(word)...)
		curW += wl
	}
	if curW > 0 || len(out) == 0 {
		flush()
	}
	return out
}
