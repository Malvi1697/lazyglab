package views

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestCommitStatus(t *testing.T) {
	pipelines := []gitlab.Pipeline{
		{SHA: "f138bae6729508b923de684d5a8e4f8a72eda3f2", Status: "success"},
	}

	if got := commitStatus("f138bae6", pipelines); got != "success" {
		t.Errorf(`commitStatus("f138bae6", pipelines) = %q, want "success"`, got)
	}
	if got := commitStatus("deadbeef", pipelines); got != "" {
		t.Errorf(`commitStatus("deadbeef", pipelines) = %q, want ""`, got)
	}
	if got := commitStatus("", pipelines); got != "" {
		t.Errorf(`commitStatus("", pipelines) = %q, want ""`, got)
	}
}

// TestOverviewView_Navigation covers the gap that made j/k feel broken: Overview
// is the default view, and it used to handle no keys at all.
func TestOverviewView_Navigation(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.height = 20
	v.commits = []gitlab.Commit{
		{ShortID: "aaa1111", Title: "first"},
		{ShortID: "bbb2222", Title: "second"},
		{ShortID: "ccc3333", Title: "third"},
	}

	navDown := tea.KeyPressMsg{Code: 'j', Text: "j"}
	navUp := tea.KeyPressMsg{Code: 'k', Text: "k"}

	v.Update(navDown)
	if v.cursor != 1 {
		t.Errorf("after j, cursor = %d, want 1", v.cursor)
	}
	v.Update(navDown)
	v.Update(navDown) // past the end
	if v.cursor != 2 {
		t.Errorf("cursor = %d, want it clamped to 2", v.cursor)
	}
	v.Update(navUp)
	if v.cursor != 1 {
		t.Errorf("after k, cursor = %d, want 1", v.cursor)
	}

	v.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if v.cursor != 2 {
		t.Errorf("after G, cursor = %d, want the last commit", v.cursor)
	}
	v.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if v.cursor != 0 {
		t.Errorf("after g, cursor = %d, want 0", v.cursor)
	}

	// Arrows must work as well as the vim keys.
	v.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if v.cursor != 1 {
		t.Errorf("after Down, cursor = %d, want 1", v.cursor)
	}
}

func TestOverviewView_CursorClampedWhenCommitsShrink(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "a"}, {ShortID: "b"}, {ShortID: "c"}}
	v.cursor = 2

	// A refresh on a different branch can return fewer commits.
	v.Update(CommitsLoadedMsg{Commits: []gitlab.Commit{{ShortID: "a"}}})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped into the shorter list", v.cursor)
	}
}

func TestOverviewView_EmptyListNavigationIsSafe(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	v.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 with no commits", v.cursor)
	}
	if cmd := v.Update(tea.KeyPressMsg{Code: 'o', Text: "o"}); cmd != nil {
		t.Error("o with no commits must do nothing")
	}
}

func TestOverviewLayout_SummariesTakeOnlyWhatTheyNeed(t *testing.T) {
	// Splitting the body in half stretched three short lists down half a tall
	// terminal while the commit list — the thing you read — was cut off.
	v := NewOverviewView(&Context{})
	for i := 0; i < 40; i++ {
		v.commits = append(v.commits, gitlab.Commit{ShortID: "c", Title: "commit", AuthorName: "A"})
	}
	v.pipelines = []gitlab.Pipeline{{ID: 1, Status: "success", Ref: "main"}}
	v.mrs = []gitlab.MergeRequest{{IID: 1, Title: "mr"}}

	const height = 40
	rows := strings.Split(v.Body(120, height), "\n")
	if len(rows) != height {
		t.Fatalf("body is %d rows, want %d", len(rows), height)
	}

	// Find where the summaries start: the row carrying their headings.
	summaryRow := -1
	for i, r := range rows {
		if strings.Contains(ansi.Strip(r), "Pipelines (") {
			summaryRow = i
			break
		}
	}
	if summaryRow == -1 {
		t.Fatal("the summary row was not rendered")
	}

	// With one pipeline and one MR, the summaries need a heading plus a few rows,
	// so they must sit near the bottom rather than halfway up.
	if summaryRow < height*2/3 {
		t.Errorf("summaries start at row %d of %d; they should hug the bottom", summaryRow, height)
	}
	// And the row above them is the blank separator.
	if strings.TrimSpace(ansi.Strip(rows[summaryRow-1])) != "" {
		t.Errorf("expected a blank row above the summaries, got %q", rows[summaryRow-1])
	}
}

func TestOverviewLayout_SummariesNeverTakeMoreThanHalf(t *testing.T) {
	v := NewOverviewView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "c", Title: "t"}}
	// More summary content than a short terminal can give room to.
	for i := 0; i < 20; i++ {
		v.pipelines = append(v.pipelines, gitlab.Pipeline{ID: i, Status: "success"})
	}

	const height = 14
	rows := strings.Split(v.Body(120, height), "\n")
	if len(rows) != height {
		t.Fatalf("body is %d rows, want %d", len(rows), height)
	}
	summaryRow := -1
	for i, r := range rows {
		if strings.Contains(ansi.Strip(r), "Pipelines (") {
			summaryRow = i
			break
		}
	}
	if summaryRow < height/2-1 {
		t.Errorf("summaries start at row %d of %d, taking more than half", summaryRow, height)
	}
}

func TestOverviewRows_ShowClockOrDateNotAnAge(t *testing.T) {
	// Every row reading "1d" told you nothing; a clock time or a date does.
	v := NewOverviewView(&Context{})
	// Noon today, not "two hours ago": run this a little after midnight and two
	// hours ago is yesterday, which is a date rather than a clock.
	now := time.Now()
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	v.commits = []gitlab.Commit{{
		ShortID: "a", Title: "today", AuthorName: "A", CreatedAt: noon,
	}}

	row := ansi.Strip(v.commitRow(v.visible()[0]))
	if strings.Contains(row, "1d") || strings.Contains(row, "2h") {
		t.Errorf("row = %q, should not show a relative age", row)
	}
	if !strings.Contains(row, ":") {
		t.Errorf("row = %q, want a clock time for a commit from today", row)
	}
}
