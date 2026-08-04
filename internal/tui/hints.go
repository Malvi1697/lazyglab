package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// One renderer for the footer and for every overlay, so the bar reads the same
// wherever it appears: what it does, then the key that does it.
//
//	Commit page: Enter | Show readme: t | Copy SHA/link: y/Y | Keybindings: ?
const hintSep = " | "

// hintBar lays hints out in the order given and drops the ones that do not fit
// rather than cutting one mid-word; everything is in the help overlay anyway.
func hintBar(width int, hints ...views.KeyHint) string {
	return hintBarKeeping(width, hints, nil)
}

// hintBarKeeping is hintBar with hints that survive however narrow the terminal is —
// the way out of wherever you are.
func hintBarKeeping(width int, hints, always []views.KeyHint) string {
	tail := join(always)
	room := width
	if tail != "" {
		room -= lipgloss.Width(tail) + lipgloss.Width(hintSep)
	}

	parts := make([]string, 0, len(hints))
	used := 0
	for _, h := range hints {
		part := renderHint(h)
		if part == "" {
			continue
		}
		next := lipgloss.Width(part)
		if len(parts) > 0 {
			next += lipgloss.Width(hintSep)
		}
		if width > 0 && used+next > room {
			break
		}
		used += next
		parts = append(parts, part)
	}
	bar := strings.Join(parts, components.HelpSepStyle.Render(hintSep))
	if tail == "" {
		return bar
	}
	if bar == "" {
		return tail
	}
	return bar + components.HelpSepStyle.Render(hintSep) + tail
}

func renderHint(h views.KeyHint) string {
	if h.Key == "" || h.Desc == "" {
		return ""
	}
	return components.HelpDescStyle.Render(h.Desc+": ") + components.HelpKeyStyle.Render(h.Key)
}

func join(hints []views.KeyHint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		if p := renderHint(h); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, components.HelpSepStyle.Render(hintSep))
}

// prefixedHints puts a search query in front of the hints, for a picker whose "/" is
// open.
func prefixedHints(query string, width int, hints ...views.KeyHint) string {
	if query == "" {
		return hintBar(width, hints...)
	}
	return query + "  " + hintBar(width-lipgloss.Width(query)-2, hints...)
}
