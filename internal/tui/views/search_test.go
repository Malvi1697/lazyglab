package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// typeKeys sends each rune of s as its own key press, the way a terminal does.
func typeKeys(v View, s string) {
	for _, r := range s {
		v.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func searchIssues() *IssuesView {
	v := NewIssuesView(&Context{})
	v.height = 20
	v.items = []gitlab.Issue{
		{IID: 1, Title: "Login crashes", Author: "alice"},
		{IID: 2, Title: "Export to CSV", Author: "bob"},
		{IID: 3, Title: "Login is slow", Author: "carol"},
	}
	return v
}

func TestSearch_NarrowsTheListAndResetsTheCursor(t *testing.T) {
	v := searchIssues()
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if v.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 before searching", v.cursor)
	}

	typeKeys(v, "/login")

	if got := len(v.visible()); got != 2 {
		t.Errorf("visible = %d issues, want the 2 matching \"login\"", got)
	}
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want it back at the top of the narrowed list", v.cursor)
	}
	if !v.CapturingText() {
		t.Error("the view should be capturing text while the query is being typed")
	}
	if got := v.selected(); got == nil || got.IID != 1 {
		t.Errorf("selected = %v, want the first match", got)
	}
}

func TestSearch_TypedLettersAreNotCommands(t *testing.T) {
	// "c" closes an issue and "o" opens a browser. Inside a search they are
	// letters, or searching for "close" would fire two actions on the way.
	v := searchIssues()
	typeKeys(v, "/csv")

	if v.search.filter.Query != "csv" {
		t.Errorf("query = %q, want %q", v.search.filter.Query, "csv")
	}
	if got := len(v.visible()); got != 1 {
		t.Errorf("visible = %d issues, want 1", got)
	}
}

func TestSearch_EnterAppliesThenTheActionKeysWorkAgain(t *testing.T) {
	v := searchIssues()
	typeKeys(v, "/csv")
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if v.CapturingText() {
		t.Error("Enter should stop the text entry")
	}
	if !v.search.filter.Applied() {
		t.Error("Enter should keep the list narrowed")
	}
	// The narrowed row is now actionable: "c" asks to close the searched-for issue.
	cmd := v.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if cmd == nil {
		t.Fatal("expected c to act on the search result")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected a confirmation, got %T", cmd())
	}
	if !strings.Contains(msg.Prompt, "#2") {
		t.Errorf("confirmation = %q, want it to name the searched-for issue #2", msg.Prompt)
	}
}

func TestSearch_EscClearsBeforeItMeansBack(t *testing.T) {
	v := searchIssues()
	typeKeys(v, "/login")
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	v.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v.search.on() {
		t.Error("Esc should clear an applied search")
	}
	if got := len(v.visible()); got != 3 {
		t.Errorf("visible = %d issues, want the full list back", got)
	}
}

func TestSearch_TitleShowsTheQueryAndTheCount(t *testing.T) {
	v := searchIssues()
	typeKeys(v, "/login")

	title := plain(v.search.title("Issues", len(v.visible()), len(v.items)))
	if !strings.Contains(title, "(2/3)") {
		t.Errorf("title = %q, want it to say how many of how many matched", title)
	}
	if !strings.Contains(title, "/login") {
		t.Errorf("title = %q, want the query visible where the narrowing happens", title)
	}
}

func TestSearch_PastedTextExtendsTheQuery(t *testing.T) {
	v := searchIssues()
	typeKeys(v, "/")
	v.Update(tea.PasteMsg{Content: "csv\n"})

	if v.search.filter.Query != "csv" {
		t.Errorf("query = %q, want the pasted text without the newline", v.search.filter.Query)
	}
}

func TestSearch_CommitPageStepsWithinTheSearchResults(t *testing.T) {
	// The page was opened from the narrowed list, so → walks that list rather than
	// jumping to a commit the search excluded.
	v := NewDashboardView(&Context{})
	v.height = 20
	v.items = []gitlab.Commit{
		{ShortID: "aaa1111", Title: "feat: one"},
		{ShortID: "bbb2222", Title: "chore: skip me"},
		{ShortID: "ccc3333", Title: "feat: two"},
	}

	typeKeys(v, "/feat")
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // applies the search
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // opens the commit page

	if !v.detail.active {
		t.Fatal("Enter should open the commit page")
	}
	if v.detail.total != 2 {
		t.Errorf("page says 1/%d, want the 2 searched commits", v.detail.total)
	}

	v.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if v.detail.commit == nil || v.detail.commit.ShortID != "ccc3333" {
		t.Errorf("→ landed on %v, want the next match rather than the skipped commit", v.detail.commit)
	}
}

func TestSearch_IsNotOfferedInsideTheCommitPage(t *testing.T) {
	// The page has no list of its own to narrow, and "/" there would swallow keys.
	v := NewDashboardView(&Context{})
	v.height = 20
	v.items = []gitlab.Commit{{ShortID: "aaa1111", Title: "one"}}
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	typeKeys(v, "/")
	if v.CapturingText() {
		t.Error("the commit page must not capture text for a list search")
	}
}
