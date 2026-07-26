package views

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func discussion() []gitlab.Note {
	return []gitlab.Note{
		{ID: 1, Author: "alice", Body: "Looks good, but the cart total is off by one.",
			CreatedAt: time.Now().Add(-3 * time.Hour)},
		{ID: 2, Author: "bob", Body: "added 3 commits", System: true,
			CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: 3, Author: "carol", Body: "Fixed.\n\nSee the second commit — the rounding\nwas the culprit.",
			Resolvable: true, Resolved: true, OnPath: "cart.py", OnLine: 42,
			CreatedAt: time.Now().Add(-time.Hour)},
	}
}

func TestNotes_HeadingSeparatesPeopleFromBookkeeping(t *testing.T) {
	// A thread of system notes is not a discussion, and a count that pretends
	// otherwise makes a quiet merge request look busy.
	var b notesBox
	b.setNotes(discussion())

	if got := b.notesTitle(); !strings.Contains(got, "2 of 3") {
		t.Errorf("title = %q, want it to say how many a person wrote", got)
	}

	b.setNotes([]gitlab.Note{{ID: 1, Author: "a", Body: "hi"}})
	if got := b.notesTitle(); got != "Discussion (1)" {
		t.Errorf("title = %q, want a plain count when nothing is a system note", got)
	}
}

func TestNotes_RowsSayWhoAndWhen(t *testing.T) {
	var b notesBox
	b.setNotes(discussion())

	row := plain(b.noteRow(0))
	for _, want := range []string{"3h", "alice", "cart total is off"} {
		if !strings.Contains(row, want) {
			t.Errorf("row = %q, want %q", row, want)
		}
	}
	// A multi-line comment is one row: the first line of it. Row 1 is carol's,
	// because the system note between them is not shown by default.
	if row := plain(b.noteRow(1)); strings.Contains(row, "rounding") {
		t.Errorf("row = %q, want only the opening line", row)
	}
}

func TestNotes_BookkeepingIsHiddenUntilAskedFor(t *testing.T) {
	// An issue can carry a hundred "changed the description" notes and no
	// conversation at all; the box is for the conversation.
	var b notesBox
	b.setNotes(discussion())

	if got := len(b.shown()); got != 2 {
		t.Errorf("showing %d notes, want the 2 people wrote", got)
	}
	if got := b.notesTitle(); !strings.Contains(got, "2 of 3") {
		t.Errorf("title = %q, want it to say some are left out", got)
	}

	b.toggleSystem()
	if got := len(b.shown()); got != 3 {
		t.Errorf("showing %d notes after s, want all 3", got)
	}
	if got := b.notesTitle(); got != "Discussion (3)" {
		t.Errorf("title = %q, want a plain count when nothing is hidden", got)
	}

	// A thread that is only bookkeeping shows it: an empty box would just puzzle.
	var only notesBox
	only.setNotes([]gitlab.Note{{ID: 1, Author: "bot", Body: "changed the description", System: true}})
	if got := len(only.shown()); got != 1 {
		t.Errorf("showing %d notes, want the record when there is nothing else", got)
	}
	if cmd := only.toggleSystem(); cmd != nil {
		if msg, ok := cmd().(StatusMsg); ok && !strings.Contains(msg.Text, "only GitLab's own record") {
			t.Errorf("s reported %q, want it to explain there is nothing to hide", msg.Text)
		}
	}
}

func TestNotes_ThreadReadsAsProse(t *testing.T) {
	var b notesBox
	b.setNotes(discussion())
	b.threadOpen = true
	b.showSystem = true // the record is kept too, a weight quieter

	body := plain(b.threadView(80, 40))
	for _, want := range []string{
		"alice", "cart total is off by one",
		"added 3 commits",
		"carol", "the rounding", // the whole body, not just its first line
		"on cart.py:42", // a comment on a line says which
		"resolved",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("thread does not contain %q:\n%s", want, body)
		}
	}
}

func TestNotes_ThreadIsCachedPerWidth(t *testing.T) {
	var b notesBox
	b.setNotes(discussion())

	first := b.thread(80)
	if got := b.thread(80); &got[0] != &first[0] {
		t.Error("a second render at the same width should reuse the rendered thread")
	}
	b.thread(40)
	if b.threadWidth != 40 {
		t.Errorf("threadWidth = %d, want the new width", b.threadWidth)
	}
	// New comments must not be read through a stale render.
	b.setNotes(append(discussion(), gitlab.Note{ID: 4, Author: "dave", Body: "one more"}))
	if b.threadLines != nil {
		t.Error("setting notes should drop the rendered thread")
	}
}

func TestNotes_EmptyDiscussionInvitesOne(t *testing.T) {
	var b notesBox
	panel := plain(b.notesPanel(60, 10, false, false))
	if !strings.Contains(panel, "No comments yet") {
		t.Errorf("panel = %q, want it to say the thread is empty", panel)
	}
	if loading := plain(b.notesPanel(60, 10, false, true)); !strings.Contains(loading, "Loading") {
		t.Errorf("panel = %q, want loading to be said before emptiness", loading)
	}
}

func TestComment_StripsTheTemplateAndKeepsTheProse(t *testing.T) {
	got := stripCommentLines("Looks good.\n\nOne question though.\n" +
		"# Write your comment above, in Markdown.\n# Lines starting with '#' are ignored.\n")
	want := "Looks good.\n\nOne question though."
	if got != want {
		t.Errorf("stripped = %q, want %q", got, want)
	}
	if got := stripCommentLines("\n#only comments\n"); got != "" {
		t.Errorf("stripped = %q, want an abandoned comment to come back empty", got)
	}
}

func TestComment_EditorFollowsGitsPreference(t *testing.T) {
	t.Setenv("GIT_EDITOR", "")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if editor, args := editorCommand(); editor != "vi" || len(args) != 0 {
		t.Errorf("editor = %q %v, want vi when nothing is set", editor, args)
	}

	t.Setenv("EDITOR", "nvim -u NONE")
	editor, args := editorCommand()
	if editor != "nvim" || strings.Join(args, " ") != "-u NONE" {
		t.Errorf("editor = %q %v, want the flags kept with the command", editor, args)
	}

	// git's order: GIT_EDITOR wins over VISUAL, which wins over EDITOR.
	t.Setenv("VISUAL", "code -w")
	t.Setenv("GIT_EDITOR", "hx")
	if editor, _ := editorCommand(); editor != "hx" {
		t.Errorf("editor = %q, want GIT_EDITOR to win", editor)
	}
}

func TestComment_TemplateExplainsItselfAndIsCleanedUp(t *testing.T) {
	path, err := commentFile("!42 Fix the cart")
	if err != nil {
		t.Fatalf("writing the scratch file: %v", err)
	}
	defer func() { _ = os.Remove(path) }()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	body := string(raw)
	if !strings.Contains(body, "!42 Fix the cart") {
		t.Errorf("template = %q, want it to name what is being commented on", body)
	}
	if stripCommentLines(body) != "" {
		t.Errorf("the template itself must strip to nothing, got %q", stripCommentLines(body))
	}
}

func TestComment_EmptyCommentIsNotPosted(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)

	cmd := v.detail.update(commentWrittenMsg{body: ""})
	if cmd == nil {
		t.Fatal("an abandoned comment should say so")
	}
	msg, ok := cmd().(StatusMsg)
	if !ok || !strings.Contains(msg.Text, "nothing posted") {
		t.Errorf("reported %v, want it to say nothing was posted", cmd())
	}
}

func TestComment_TheDiscussionIsInTheTabCycleEvenWhenEmpty(t *testing.T) {
	// c is how a discussion starts, so there has to be somewhere to stand.
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v.detail.update(MRDetailLoadedMsg{IID: 42,
		MR: &gitlab.MergeRequest{IID: 42, Title: "t", TargetBranch: "main"}})

	v.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if v.detail.focus != focusNotes {
		t.Errorf("focus = %v, want the discussion with no other box to visit", v.detail.focus)
	}
}

func TestComment_ThreadTakesTheBodyAndKeepsTheArrowsToItself(t *testing.T) {
	v := mrsWithPage(t)
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	loadedPage(v)
	v.detail.setNotes(discussion())
	v.detail.focus = focusNotes

	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !v.detail.threadOpen {
		t.Fatal("Enter in the discussion should open the thread")
	}
	if !v.detail.readingBody() {
		t.Error("an open thread must own the arrows, or h would swap it for another MR")
	}
	if body := plain(v.detail.body(160, 45)); !strings.Contains(body, "cart total is off by one") {
		t.Errorf("the thread should take the body:\n%s", body)
	}

	v.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if v.detail.threadOpen || v.detail.focus != focusNotes {
		t.Error("Esc should come back to the discussion box")
	}
}

func TestNotes_TheToggleIsOnlyOfferedWhenItWouldChangeSomething(t *testing.T) {
	// Offering to hide the record when the record is all there is sends people
	// pressing a key that can only answer "there is nothing to hide".
	var mixed notesBox
	mixed.setNotes(discussion())
	if got := hintsFor(mixed.boxHints()); !strings.Contains(got, "s ") {
		t.Errorf("hints = %q, want s offered when some notes are hidden", got)
	}

	var onlyRecord notesBox
	onlyRecord.setNotes([]gitlab.Note{{ID: 1, Author: "bot", Body: "changed the description", System: true}})
	if got := hintsFor(onlyRecord.boxHints()); strings.Contains(got, "s ") {
		t.Errorf("hints = %q, want no toggle when there is nothing to hide", got)
	}

	var onlyPeople notesBox
	onlyPeople.setNotes([]gitlab.Note{{ID: 1, Author: "alice", Body: "hello"}})
	if got := hintsFor(onlyPeople.boxHints()); strings.Contains(got, "s ") {
		t.Errorf("hints = %q, want no toggle when nothing is hidden", got)
	}
}
