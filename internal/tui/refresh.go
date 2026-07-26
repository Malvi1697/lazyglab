package tui

import (
	"fmt"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// spinnerFrames is the braille spinner shown while a refresh is in flight.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// justRefreshed is how long a finished refresh is called out in the success
// colour. It answers the question "did pressing r do anything?" even when the
// data came back identical, which is the usual case and used to look like nothing
// had happened at all.
const justRefreshed = 3 * time.Second

// refreshNote is the note at the far right of the context bar: the spinner while
// data is being fetched, a tick just after it lands, then how stale the data is
// and how long until the next automatic fetch.
func (a *App) refreshNote(now time.Time) string {
	if a.refreshing {
		frame := spinnerFrames[a.spinFrame%len(spinnerFrames)]
		return components.TitleStyle.Render(frame + " refreshing")
	}
	if a.lastRefresh.IsZero() {
		return ""
	}

	since := now.Sub(a.lastRefresh)
	if since < justRefreshed {
		return lipgloss.NewStyle().Foreground(components.ColorSuccess).Bold(true).
			Render("✓ updated now")
	}

	note := "updated " + shortDuration(since) + " ago"
	if a.refreshInterval > 0 && !a.nextRefresh.IsZero() {
		if next := a.nextRefresh.Sub(now); next > 0 {
			note += " · next " + shortDuration(next)
		}
	}
	return components.MutedStyle.Render(note)
}

// shortDuration renders a duration in one unit, rounded down, for a note that has
// to stay the same width as it counts.
func shortDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
}
