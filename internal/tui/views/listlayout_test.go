package views

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// The width of a terminal that is not especially wide, where the difference
// between "the list has the page" and "the list has half the page" is the
// difference between reading a title and guessing at it.
const testWidth = 100

func TestLists_TheTitleFitsBecauseTheListHasTheWholePage(t *testing.T) {
	// Every list used to give half its width to a panel restating the highlighted
	// row, and the titles paid for it: "feat: split registration into two..." with
	// the rest of the sentence nowhere on screen.
	const title = "feat(cart): split registration into two steps"

	pages := map[string]string{
		"merge requests": func() string {
			v := NewMRsView(&Context{})
			v.mrs = []gitlab.MergeRequest{{IID: 16, Title: title, Author: "jiri.kucera",
				SourceBranch: "split-registration", UpdatedAt: time.Now()}}
			return plain(v.Body(testWidth, 20))
		}(),
		"issues": func() string {
			v := NewIssuesView(&Context{})
			v.issues = []gitlab.Issue{{IID: 7, Title: title, Author: "alice", UpdatedAt: time.Now()}}
			return plain(v.Body(testWidth, 20))
		}(),
		"pipelines": func() string {
			v := NewPipelinesView(&Context{})
			v.pipelines = []gitlab.Pipeline{{ID: 1, Status: "success", CommitTitle: title,
				Ref: "split-registration", CreatedAt: time.Now()}}
			return plain(v.Body(testWidth, 20))
		}(),
		"todos": func() string {
			v := NewTodosView(&Context{})
			v.loaded = true
			v.todos = []gitlab.Todo{{ID: 1, Action: "review_requested", Reference: "!16",
				Title: title, ProjectPath: "group/api", CreatedAt: time.Now()}}
			return plain(v.Body(testWidth, 20))
		}(),
		"dashboard": func() string {
			v := NewDashboardView(&Context{})
			v.commits = []gitlab.Commit{{ShortID: "abc1234", Title: title,
				AuthorName: "jiri.kucera", CreatedAt: time.Now()}}
			return plain(v.Body(testWidth, 20))
		}(),
	}

	for name, body := range pages {
		// The conventional prefix moves to its own column, so what has to survive
		// intact is the sentence after it.
		if !strings.Contains(body, "split registration into two steps") {
			t.Errorf("%s: the title is cut off in\n%s", name, body)
		}
		for _, line := range strings.Split(body, "\n") {
			if lipgloss.Width(line) > testWidth {
				t.Errorf("%s: a line is %d columns wide, want at most %d: %q",
					name, lipgloss.Width(line), testWidth, line)
			}
		}
	}
}

func TestLists_TheColumnsMeanTheSameThingEverywhere(t *testing.T) {
	// Read any two rows from different views and the metadata is in the same places:
	// what it is called, what kind of thing it is, then who/where/when at the right.
	now := time.Date(time.Now().Year(), 7, 27, 9, 12, 0, 0, time.Local)

	mr := plain(renderRow(mrRow(gitlab.MergeRequest{
		IID: 42, Title: "feat(cart): promote", Author: "jiri.kucera",
		SourceBranch: "promo-cap", UpdatedAt: now,
	}), testWidth))
	issue := plain(renderRow(issueRow(gitlab.Issue{
		IID: 7, Title: "fix(api): crash", Author: "alice", UpdatedAt: now,
	}), testWidth))

	for _, tc := range []struct {
		name, row string
		order     []string
	}{
		{"merge request", mr, []string{"!42", "feat:", "promote", "jiri.kucera", "promo-cap", "27.7."}},
		{"issue", issue, []string{"#7", "fix:", "crash", "alice", "27.7."}},
	} {
		for i := 1; i < len(tc.order); i++ {
			at, prev := strings.Index(tc.row, tc.order[i]), strings.Index(tc.row, tc.order[i-1])
			if prev < 0 || at < 0 {
				t.Fatalf("%s row = %q, want it to carry %q and %q", tc.name, tc.row, tc.order[i-1], tc.order[i])
			}
			if prev >= at {
				t.Errorf("%s row = %q, want %q before %q", tc.name, tc.row, tc.order[i-1], tc.order[i])
			}
		}
	}
}

func TestDashboard_TShowsAndFoldsTheReadme(t *testing.T) {
	// In a small window half the rows spent on the README is half the commits you
	// cannot see, which is the whole reason the key exists.
	v := NewDashboardView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "abc1234", Title: "feat: one", AuthorName: "jan"}}
	v.setReadme("README.md", "# Idiskgolf\n\nThis is the frontend.")

	// It starts folded: the commits are what the page is opened for.
	body := plain(v.Body(testWidth, 20))
	if strings.Contains(body, "This is the frontend") {
		t.Errorf("the README should start folded away:\n%s", body)
	}
	if !strings.Contains(body, "feat:") || !strings.Contains(body, "one") {
		t.Errorf("the commits should have the whole page:\n%s", body)
	}
	// The hint has to say which way the key goes, or it promises to hide something
	// that is already hidden.
	if !hasHint(v.KeyHints(), "t", "Show readme") {
		t.Errorf("hints = %v, want t offering to show it", v.KeyHints())
	}
	if hasHint(v.KeyHints(), "Tab", "Readme") {
		t.Errorf("hints = %v, want no Tab to a box that is not on screen", v.KeyHints())
	}

	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	if body := plain(v.Body(testWidth, 20)); !strings.Contains(body, "This is the frontend") {
		t.Errorf("t should have brought the README up:\n%s", body)
	}
	if !hasHint(v.KeyHints(), "t", "Hide readme") {
		t.Errorf("hints = %v, want t offering to fold it away again", v.KeyHints())
	}

	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	if body := plain(v.Body(testWidth, 20)); strings.Contains(body, "This is the frontend") {
		t.Errorf("t should have folded it away again:\n%s", body)
	}
}

func TestDashboard_FoldingTheReadmeTakesTheKeysBack(t *testing.T) {
	// Folding it away while it had the keys would leave j/k scrolling something
	// nobody can see.
	v := NewDashboardView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "abc1234", Title: "feat: one"}}
	v.setReadme("README.md", "words")
	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"}) // bring it up first

	v.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if v.focus != focusNotes {
		t.Fatalf("focus = %v, want the README to have the keys", v.focus)
	}

	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	if v.focus != focusPage {
		t.Errorf("focus = %v, want the keys back with the commits", v.focus)
	}
}

func TestTodos_TFoldsTheDetailAway(t *testing.T) {
	v := todosView()
	v.loaded = true

	if body := plain(v.Body(testWidth, 24)); !strings.Contains(body, "your review was requested") {
		t.Fatalf("the detail should be there to begin with:\n%s", body)
	}

	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	body := plain(v.Body(testWidth, 24))
	if strings.Contains(body, "your review was requested") {
		t.Errorf("t should have folded the detail away:\n%s", body)
	}
	if !strings.Contains(body, "Fix the cart") {
		t.Errorf("the list should still be there:\n%s", body)
	}
	if !hasHint(v.KeyHints(), "t", "Show detail") {
		t.Errorf("hints = %v, want t offering to show it again", v.KeyHints())
	}
}

// hasHint reports whether the footer offers this key with this description.
func hasHint(hints []KeyHint, key, desc string) bool {
	for _, h := range hints {
		if h.Key == key && h.Desc == desc {
			return true
		}
	}
	return false
}
