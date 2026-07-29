package views

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func todosView() *TodosView {
	v := NewTodosView(&Context{})
	v.height = 20
	v.width = 120
	v.todos = []gitlab.Todo{
		{
			ID: 1, Action: "review_requested", Target: "MergeRequest", Reference: "!42",
			Title: "Fix the cart", ProjectPath: "group/api", Author: "alice",
			CreatedAt: time.Now().Add(-2 * time.Hour),
		},
		{
			ID: 2, Action: "mentioned", Target: "Issue", Reference: "#7",
			Title: "Crash on login", ProjectPath: "group/web", Author: "bob",
			Body: "@carol can you look?", CreatedAt: time.Now().Add(-30 * time.Minute),
		},
		{
			ID: 3, Action: "build_failed", Target: "Commit",
			Title: "The pipeline failed", ProjectPath: "group/web",
		},
	}
	return v
}

func TestTodos_RowSaysWhyItIsThereAndWhere(t *testing.T) {
	v := todosView()
	row := plain(renderRow(todoRow(v.visible()[0]), 120))

	// The stamp is derived, not hardcoded: a test that says "2h" breaks whenever the
	// list is laid out in another unit.
	when := commitStamp(v.visible()[0].CreatedAt)
	for _, want := range []string{when, "review", "api", "!42", "Fix the cart"} {
		if !strings.Contains(row, want) {
			t.Errorf("row = %q, want it to contain %q", row, want)
		}
	}
	// The group repeats on every row and would push the title off the screen.
	if strings.Contains(row, "group/api") {
		t.Errorf("row = %q, want only the project name, not the whole path", row)
	}
}

func TestTodos_DetailSpellsOutTheReason(t *testing.T) {
	v := todosView()
	detail := plain(v.todoDetail())

	if !strings.Contains(detail, "your review was requested") {
		t.Errorf("detail = %q, want GitLab's action name spelled out", detail)
	}
	if !strings.Contains(detail, "group/api") {
		t.Errorf("detail = %q, want the full project path, which the row omits", detail)
	}
}

func TestTodos_EmptyListSaysSoOnlyOnceItIsKnown(t *testing.T) {
	v := NewTodosView(&Context{})
	if got := plain(v.todoDetail()); strings.Contains(got, "Nothing is waiting on you") {
		t.Errorf("detail = %q, want it not to claim a clear plate before the reply", got)
	}

	v.Update(TodosLoadedMsg{})
	if got := plain(v.todoDetail()); !strings.Contains(got, "Nothing is waiting on you") {
		t.Errorf("detail = %q, want an empty list to say so plainly", got)
	}
}

func TestTodos_MarkingOneDoneAsksFirst(t *testing.T) {
	v := todosView()
	cmd := v.Update(tea.KeyPressMsg{Code: 'd', Text: "d"})
	if cmd == nil {
		t.Fatal("d should ask to mark the todo done")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected a confirmation, got %T", cmd())
	}
	if !strings.Contains(msg.Prompt, "!42") {
		t.Errorf("prompt = %q, want it to name the todo", msg.Prompt)
	}
}

func TestTodos_ClearingTheListNamesTheCount(t *testing.T) {
	v := todosView()
	cmd := v.Update(tea.KeyPressMsg{Code: 'D', Text: "D"})
	if cmd == nil {
		t.Fatal("D should ask to clear the list")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected a confirmation, got %T", cmd())
	}
	if !strings.Contains(msg.Prompt, "3") {
		t.Errorf("prompt = %q, want it to say how many would be cleared", msg.Prompt)
	}
}

func TestTodos_ClearedTodoRefetchesTheList(t *testing.T) {
	// The row is gone on GitLab's side, so leaving it on screen would be a lie.
	v := todosView()
	v.ctx = &Context{Client: &gitlab.Client{}}

	cmd := v.Update(TodoActionDoneMsg{Text: "Done: !42 Fix the cart"})
	if cmd == nil {
		t.Fatal("clearing a todo should reload the list and report it")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a batch of reload + status, got %T", cmd())
	}
	if len(batch) != 2 {
		t.Errorf("batch has %d commands, want the reload and the status line", len(batch))
	}
}

func TestTodos_SearchNarrowsAcrossProjects(t *testing.T) {
	v := todosView()
	typeKeys(v, "/web")

	if got := len(v.visible()); got != 2 {
		t.Errorf("visible = %d todos, want the 2 from group/web", got)
	}
}

func TestTodos_NeedsNoProject(t *testing.T) {
	// Every other view is about the selected project; this one is about the user,
	// so it must fetch with no project chosen at all.
	v := NewTodosView(&Context{Client: &gitlab.Client{}})
	if v.Focus() == nil {
		t.Error("the todo list should load without a project selected")
	}
}
