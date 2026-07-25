package components

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestWrapLine(t *testing.T) {
	t.Run("wraps on spaces within width", func(t *testing.T) {
		lines := WrapLine("the quick brown fox", 9)
		if len(lines) < 2 {
			t.Fatalf("want >=2 lines, got %d (%v)", len(lines), lines)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 9 {
				t.Errorf("line %d (%q) has width %d, want <=9", i, l, w)
			}
		}
		// No content should be lost or reordered.
		if joined := strings.Join(lines, " "); joined != "the quick brown fox" {
			t.Errorf("want reassembled %q, got %q", "the quick brown fox", joined)
		}
	})

	t.Run("hard-breaks an overlong word", func(t *testing.T) {
		lines := WrapLine("supercalifragilistic", 5)
		if len(lines) < 4 {
			t.Fatalf("want >=4 chunks, got %d (%v)", len(lines), lines)
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 5 {
				t.Errorf("chunk %d (%q) has width %d, want <=5", i, l, w)
			}
		}
	})

	t.Run("short line returned unchanged", func(t *testing.T) {
		lines := WrapLine("short", 20)
		if len(lines) != 1 || lines[0] != "short" {
			t.Errorf("want [\"short\"], got %v", lines)
		}
	})
}

func TestTruncate(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		if got := Truncate("hello", 10); got != "hello" {
			t.Errorf(`Truncate("hello", 10) = %q, want "hello"`, got)
		}
	})

	t.Run("long string truncated with ellipsis", func(t *testing.T) {
		got := Truncate("this is a long string", 10)
		if lipgloss.Width(got) != 10 {
			t.Errorf("want width 10, got %d (%q)", lipgloss.Width(got), got)
		}
		if !strings.HasSuffix(got, "...") {
			t.Errorf("want ellipsis suffix, got %q", got)
		}
	})

	t.Run("width <=3 has no ellipsis", func(t *testing.T) {
		got := Truncate("this is a long string", 3)
		if strings.Contains(got, "...") {
			t.Errorf("want no ellipsis at width<=3, got %q", got)
		}
		if lipgloss.Width(got) > 3 {
			t.Errorf("want width <=3, got %d (%q)", lipgloss.Width(got), got)
		}
	})

	t.Run("width 0 returns empty", func(t *testing.T) {
		if got := Truncate("hello", 0); got != "" {
			t.Errorf(`Truncate("hello", 0) = %q, want ""`, got)
		}
	})
}

func TestPadRight(t *testing.T) {
	t.Run("pads short string to width", func(t *testing.T) {
		got := PadRight("hi", 5)
		if got != "hi   " {
			t.Errorf(`PadRight("hi", 5) = %q, want "hi   "`, got)
		}
	})

	t.Run("does not shrink a longer string", func(t *testing.T) {
		got := PadRight("hello world", 5)
		if got != "hello world" {
			t.Errorf(`PadRight("hello world", 5) = %q, want unchanged`, got)
		}
	})

	t.Run("exact width unchanged", func(t *testing.T) {
		got := PadRight("hello", 5)
		if got != "hello" {
			t.Errorf(`PadRight("hello", 5) = %q, want "hello"`, got)
		}
	})
}
