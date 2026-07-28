package util

import (
	"fmt"
	"strings"
	"time"
)

// TimeAgoShort returns a compact relative time string padded to a fixed width
// of 4 (right-aligned) so it forms an aligned column, e.g. " <1m", " 26m",
// "  3h", "  5d", " 3mo".
func TimeAgoShort(t time.Time) string {
	d := time.Since(t)

	var s string
	switch {
	case d < time.Minute:
		s = "<1m"
	case d < time.Hour:
		s = fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		s = fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		s = fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		s = fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	}
	return fmt.Sprintf("%4s", s)
}

// TimeAgo returns a human-readable relative time string.
func TimeAgo(t time.Time) string {
	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		months := int(d.Hours() / 24 / 30)
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

// CommitTime formats a commit's timestamp for a list column, padded to a fixed
// width so the column stays a column.
//
// A relative age is the wrong unit here: a day's work shows as "1d" on every row,
// which says nothing about order or when anything happened. So today's commits get
// the clock — that is what tells them apart — and older ones the date, which is
// what tells those apart. Neither ever needs both, so the column is eight columns
// wide rather than twelve.
func CommitTime(t time.Time) string { return commitTimeAt(t, time.Now()) }

// commitStampWidth is the width of the column: "30.12.25" at its longest.
const commitStampWidth = 8

// commitTimeAt is CommitTime with an explicit "now", for tests.
func commitTimeAt(t, now time.Time) string {
	if t.IsZero() {
		return strings.Repeat(" ", commitStampWidth)
	}

	var s string
	switch {
	case sameDay(t, now):
		s = t.Format("15:04")
	case t.Year() == now.Year():
		s = fmt.Sprintf("%d.%d.", t.Day(), int(t.Month()))
	default:
		// Another year: the year earns its place, the minute does not.
		s = fmt.Sprintf("%d.%d.%02d", t.Day(), int(t.Month()), t.Year()%100)
	}
	// Right-aligned, so the dots and the times line up down the list.
	return fmt.Sprintf("%*s", commitStampWidth, s)
}

// sameDay reports whether two times fall on the same calendar day.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
