package util

import (
	"fmt"
	"testing"
	"time"
)

func TestTimeAgo(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		// Just now (< 1 minute)
		{
			name: "zero seconds ago",
			time: now,
			want: "just now",
		},
		{
			name: "30 seconds ago",
			time: now.Add(-30 * time.Second),
			want: "just now",
		},
		{
			name: "59 seconds ago",
			time: now.Add(-59 * time.Second),
			want: "just now",
		},

		// Minutes
		{
			name: "1 minute ago",
			time: now.Add(-1 * time.Minute),
			want: "1 minute ago",
		},
		{
			name: "30 minutes ago",
			time: now.Add(-30 * time.Minute),
			want: "30 minutes ago",
		},
		{
			name: "59 minutes ago",
			time: now.Add(-59 * time.Minute),
			want: "59 minutes ago",
		},

		// Hours
		{
			name: "1 hour ago",
			time: now.Add(-1 * time.Hour),
			want: "1 hour ago",
		},
		{
			name: "12 hours ago",
			time: now.Add(-12 * time.Hour),
			want: "12 hours ago",
		},
		{
			name: "23 hours ago",
			time: now.Add(-23 * time.Hour),
			want: "23 hours ago",
		},

		// Days
		{
			name: "1 day ago",
			time: now.Add(-24 * time.Hour),
			want: "1 day ago",
		},
		{
			name: "15 days ago",
			time: now.Add(-15 * 24 * time.Hour),
			want: "15 days ago",
		},
		{
			name: "29 days ago",
			time: now.Add(-29 * 24 * time.Hour),
			want: "29 days ago",
		},

		// Months
		{
			name: "1 month ago (30 days)",
			time: now.Add(-30 * 24 * time.Hour),
			want: "1 month ago",
		},
		{
			name: "6 months ago (180 days)",
			time: now.Add(-180 * 24 * time.Hour),
			want: "6 months ago",
		},
		{
			name: "11 months ago (330 days)",
			time: now.Add(-330 * 24 * time.Hour),
			want: "11 months ago",
		},

		// Years (reported as months since there is no year case)
		{
			name: "1 year ago (365 days)",
			time: now.Add(-365 * 24 * time.Hour),
			want: "12 months ago",
		},
		{
			name: "5 years ago (1825 days)",
			time: now.Add(-1825 * 24 * time.Hour),
			want: "60 months ago",
		},

		// Zero time (very far in the past)
		{
			name: "zero time",
			time: time.Time{},
			// zero time is year 0001; produces a large month count
			want: func() string {
				d := time.Since(time.Time{})
				months := int(d.Hours() / 24 / 30)
				if months == 1 {
					return "1 month ago"
				}
				return formatMonths(months)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TimeAgo(tt.time)
			if got != tt.want {
				t.Errorf("TimeAgo(%v) = %q, want %q", tt.time, got, tt.want)
			}
		})
	}
}

// formatMonths is a test helper that mirrors the default branch formatting.
func formatMonths(months int) string {
	return fmt.Sprintf("%d months ago", months)
}
