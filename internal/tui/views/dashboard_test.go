package views

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// TestDashboard_Navigation covers the gap that made j/k feel broken: the dashboard
// is the default view, and it used to handle no keys at all.
func TestDashboard_Navigation(t *testing.T) {
	v := NewDashboardView(&Context{})
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

func TestDashboard_CursorClampedWhenCommitsShrink(t *testing.T) {
	v := NewDashboardView(&Context{})
	v.commits = []gitlab.Commit{{ShortID: "a"}, {ShortID: "b"}, {ShortID: "c"}}
	v.cursor = 2

	// A refresh on a different branch can return fewer commits.
	v.Update(CommitsLoadedMsg{Commits: []gitlab.Commit{{ShortID: "a"}}})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped into the shorter list", v.cursor)
	}
}

func TestDashboard_EmptyListNavigationIsSafe(t *testing.T) {
	v := NewDashboardView(&Context{})
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	v.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want 0 with no commits", v.cursor)
	}
	if cmd := v.Update(tea.KeyPressMsg{Code: 'o', Text: "o"}); cmd != nil {
		t.Error("o with no commits must do nothing")
	}
}

func TestDashboard_ShowClockOrDateNotAnAge(t *testing.T) {
	// Every row reading "1d" told you nothing; a clock time or a date does.
	v := NewDashboardView(&Context{})
	// Noon today, not "two hours ago": run this a little after midnight and two
	// hours ago is yesterday, which is a date rather than a clock.
	now := time.Now()
	noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	v.commits = []gitlab.Commit{{
		ShortID: "a", Title: "today", AuthorName: "A", CreatedAt: noon,
	}}

	row := ansi.Strip(renderRow(v.commitRow(v.visible()[0]), 120))
	if strings.Contains(row, "1d") || strings.Contains(row, "2h") {
		t.Errorf("row = %q, should not show a relative age", row)
	}
	if !strings.Contains(row, ":") {
		t.Errorf("row = %q, want a clock time for a commit from today", row)
	}
}

func TestDashboard_CommitsAboveTheReadme(t *testing.T) {
	// GitLab's own front page: what has been happening, and what the project says
	// about itself. Both halves scroll, so both get real room.
	v := NewDashboardView(&Context{})
	for i := 0; i < 40; i++ {
		v.commits = append(v.commits, gitlab.Commit{ShortID: "c", Title: "commit", AuthorName: "A"})
	}
	v.setReadme("README.md", "# The Project\n\nWhat it does.\n")
	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"}) // it starts folded away

	const height = 40
	rows := strings.Split(v.Body(120, height), "\n")
	if len(rows) != height {
		t.Fatalf("body is %d rows, want %d", len(rows), height)
	}

	commitsRow, readmeRow := -1, -1
	for i, r := range rows {
		plainRow := ansi.Strip(r)
		if commitsRow == -1 && strings.Contains(plainRow, "Recent Commits") {
			commitsRow = i
		}
		if readmeRow == -1 && strings.Contains(plainRow, "README.md") {
			readmeRow = i
		}
	}
	if commitsRow == -1 || readmeRow == -1 {
		t.Fatalf("headings not found (commits=%d readme=%d)", commitsRow, readmeRow)
	}
	if commitsRow > readmeRow {
		t.Errorf("the README (row %d) is above the commits (row %d)", readmeRow, commitsRow)
	}
	// Roughly half each: the README is the point of the page, not a footnote.
	if readmeRow < height/3 || readmeRow > height*2/3 {
		t.Errorf("the README starts at row %d of %d, want it around halfway", readmeRow, height)
	}
	if strings.TrimSpace(ansi.Strip(rows[readmeRow-1])) != "" {
		t.Errorf("expected a blank row between the halves, got %q", rows[readmeRow-1])
	}
	if !strings.Contains(ansi.Strip(v.Body(120, height)), "What it does.") {
		t.Error("the README's text should be on screen")
	}
}

func TestDashboard_TabMovesBetweenTheHalves(t *testing.T) {
	v := NewDashboardView(&Context{})
	v.height = 40
	v.commits = []gitlab.Commit{{ShortID: "a", Title: "one"}, {ShortID: "b", Title: "two"}}
	v.setReadme("README.md", strings.Repeat("a line of the readme\n", 200))
	v.Update(tea.KeyPressMsg{Code: 't', Text: "t"}) // it starts folded away
	v.Body(120, 40)

	v.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if v.focus != focusNotes {
		t.Fatalf("focus = %v, want the README", v.focus)
	}

	// j scrolls the README rather than moving the commit cursor.
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if v.readmeBox.scroll == 0 { // the embedded box's own offset
		t.Error("j should scroll the README when it has the focus")
	}
	if v.cursor != 0 {
		t.Errorf("cursor = %d, want the commit list left alone", v.cursor)
	}

	// Esc hands the keys back.
	v.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v.focus != focusPage {
		t.Errorf("focus = %v, want the commits again", v.focus)
	}
	v.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if v.cursor != 1 {
		t.Errorf("cursor = %d, want j to move the commit cursor again", v.cursor)
	}
}

func TestDashboard_ReadmeIsFetchedOncePerProject(t *testing.T) {
	// It is the one thing on this page that does not change while you watch, and it
	// is a whole file: asking for it every thirty seconds would be waste.
	v := NewDashboardView(&Context{})
	if !v.wants("README.md") {
		t.Error("a project with a README should want it fetched")
	}
	v.setReadme("README.md", "# hi")
	if v.wants("README.md") {
		t.Error("the README we already hold must not be refetched")
	}
	if !v.wants("docs/README.md") {
		t.Error("another project's README is another file")
	}
	// A project without one is never asked about.
	if v.wants("") {
		t.Error("a project with no README must not be asked for one")
	}
}

func TestDashboard_MissingReadmeSaysSoRatherThanSpinning(t *testing.T) {
	v := NewDashboardView(&Context{})
	v.setReadme("", "")
	if got := ansi.Strip(v.readmePanel(80, 10, false)); !strings.Contains(got, "no README") {
		t.Errorf("panel = %q, want it to say the project has none", got)
	}
}

func TestDashboard_AnotherProjectDoesNotKeepTheLastReadme(t *testing.T) {
	// Switching to a project without a README would otherwise leave the previous
	// project's words sitting under the new project's commits.
	ctx := &Context{Client: &gitlab.Client{}, Project: &gitlab.Project{ID: 1, ReadmeFile: "README.md"}}
	v := NewDashboardView(ctx)
	v.syncReadme()
	v.setReadme("README.md", "# The first project")

	ctx.Project = &gitlab.Project{ID: 2} // no README at all
	v.syncReadme()

	if got := ansi.Strip(v.readmePanel(80, 10, false)); strings.Contains(got, "first project") {
		t.Errorf("panel = %q, want the previous project's README dropped", got)
	}
	if got := ansi.Strip(v.readmePanel(80, 10, false)); !strings.Contains(got, "no README") {
		t.Errorf("panel = %q, want it to say this one has none", got)
	}

	// And a project that has one is asked about again.
	ctx.Project = &gitlab.Project{ID: 3, ReadmeFile: "docs/README.md"}
	if cmd := v.syncReadme(); cmd == nil {
		t.Error("a new project with a README should be fetched")
	}
}

func TestDashboard_CommitColumnsLineUp(t *testing.T) {
	// The kind was inline, so every subject started somewhere else and the eye had
	// nothing to follow down the list.
	v := NewDashboardView(&Context{})
	now := time.Now()
	v.commits = []gitlab.Commit{
		{ShortID: "a", Title: "fix: short prefix", AuthorName: "Jan Všetíček", CreatedAt: now},
		{ShortID: "b", Title: "refactor(api): long prefix", AuthorName: "Someone Else", CreatedAt: now},
		{ShortID: "c", Title: "no conventional prefix at all", AuthorName: "Third Person", CreatedAt: now},
	}

	rows := strings.Split(ansi.Strip(v.commitsBox(120, 6)), "\n")[1:4]

	// Every subject starts at the same column — measured in display columns, not
	// bytes: the selection gutter and the author names are multibyte.
	at := func(row, text string) int {
		i := strings.Index(row, text)
		if i < 0 {
			return -1
		}
		return lipgloss.Width(row[:i])
	}
	first := at(rows[0], "short prefix")
	second := at(rows[1], "long prefix")
	third := at(rows[2], "no conventional prefix")
	if first < 0 || second < 0 || third < 0 {
		t.Fatalf("subjects not found in %q", rows)
	}
	if first != second || second != third {
		t.Errorf("subjects start at %d, %d and %d:\n%s", first, second, third, strings.Join(rows, "\n"))
	}

	// And the order is kind, subject, author, when — the timestamp last, past the
	// author, because it is looked up rather than scanned.
	row := rows[0]
	if at(row, "fix:") >= first || first >= at(row, "Jan Všetíček") {
		t.Errorf("row = %q, want kind, subject, then the author", row)
	}
	if when := at(row, now.Format("15:04")); when < at(row, "Jan Všetíček") {
		t.Errorf("row = %q, want the timestamp at the right, past the author", row)
	}
}

func TestDashboard_TheTimestampSitsAtTheRight(t *testing.T) {
	// Nineteen columns of metadata went past before the message started. The when
	// moved to the far right — past the author, where it is looked up rather than
	// scanned — and carries the date and the time together.
	v := NewDashboardView(&Context{})
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -7)
	v.commits = []gitlab.Commit{
		{ShortID: "a", Title: "chore: one", AuthorName: "A", CreatedAt: now},
		{ShortID: "b", Title: "chore: two", AuthorName: "A", CreatedAt: lastWeek},
	}

	rows := strings.Split(ansi.Strip(v.commitsBox(120, 5)), "\n")[1:3]

	// Nothing before the CI mark and the kind: the row opens with what it is. In
	// display columns, since the selection gutter is multibyte.
	for i, row := range rows {
		if strings.TrimSpace(row) == "" {
			t.Fatalf("row %d is blank", i)
		}
		at := strings.Index(row, "chore:")
		if at < 0 {
			t.Fatalf("row %d = %q, want the message in it", i, row)
		}
		if before := lipgloss.Width(row[:at]); before > 8 {
			t.Errorf("row = %q, want the message to start early, not %d columns in", row, before)
		}
	}

	for i, when := range []time.Time{now, lastWeek} {
		want := fmt.Sprintf("%d.%d. %02d:%02d", when.Day(), int(when.Month()), when.Hour(), when.Minute())
		if !strings.Contains(rows[i], want) {
			t.Errorf("row = %q, want it to end with %q", rows[i], want)
		}
		if strings.Index(rows[i], want) < strings.Index(rows[i], "chore:") {
			t.Errorf("row = %q, want the timestamp after the message", rows[i])
		}
	}
}

func TestDashboard_NarrowRowsKeepTheMessage(t *testing.T) {
	// The author and the timestamp come along only if they fit beside the message,
	// never instead of it.
	v := NewDashboardView(&Context{})
	v.commits = []gitlab.Commit{{
		ShortID: "a", Title: "feat: a reasonably long subject line here",
		AuthorName: "Jan Všetíček", CreatedAt: time.Now(),
	}}

	row := ansi.Strip(strings.Split(v.commitsBox(46, 3), "\n")[1])
	if !strings.Contains(row, "reasonably long") {
		t.Errorf("row = %q, want the message kept on a narrow terminal", row)
	}
	if lipgloss.Width(row) > 46 {
		t.Errorf("row is %d wide, want at most 46: %q", lipgloss.Width(row), row)
	}
}
