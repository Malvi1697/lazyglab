package gitlab

import (
	"net/http"
	"testing"
)

func TestListTodos_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/todos", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("state"); got != "pending" {
			t.Errorf("want state=pending, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 130,
				"project": {"path_with_namespace": "group/api"},
				"author": {"username": "alice"},
				"action_name": "review_requested",
				"target_type": "MergeRequest",
				"target": {"iid": 42, "title": "Fix the cart", "state": "opened"},
				"target_url": "https://gitlab.com/group/api/-/merge_requests/42",
				"body": "Fix the cart",
				"state": "pending",
				"created_at": "2026-07-20T08:00:00Z"
			},
			{
				"id": 131,
				"project": {"path_with_namespace": "group/web"},
				"author": {"username": "bob"},
				"action_name": "mentioned",
				"target_type": "Issue",
				"target": {"iid": 7, "title": "Crash on login", "state": "opened"},
				"target_url": "https://gitlab.com/group/web/-/issues/7",
				"body": "@carol can you look?",
				"state": "pending",
				"created_at": "2026-07-21T09:30:00Z"
			},
			{
				"id": 132,
				"project": {"path_with_namespace": "group/web"},
				"action_name": "build_failed",
				"target_type": "Commit",
				"target_url": "https://gitlab.com/group/web/-/commit/abc",
				"body": "The pipeline failed",
				"state": "pending"
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	todos, err := client.ListTodos()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(todos) != 3 {
		t.Fatalf("got %d todos, want 3", len(todos))
	}

	mr := todos[0]
	if mr.ID != 130 || mr.Action != "review_requested" || mr.Reference != "!42" {
		t.Errorf("merge request todo = %+v", mr)
	}
	if mr.Title != "Fix the cart" || mr.ProjectPath != "group/api" || mr.Author != "alice" {
		t.Errorf("merge request todo = %+v", mr)
	}
	if mr.CreatedAt.IsZero() {
		t.Error("want the creation time kept, for the age column")
	}

	if got := todos[1].Reference; got != "#7" {
		t.Errorf("issue reference = %q, want #7", got)
	}

	// A commit has no number GitLab would write, and no target title either — the body is
	// all there is to show, so the row must fall back to it.
	commit := todos[2]
	if commit.Reference != "" {
		t.Errorf("commit reference = %q, want none", commit.Reference)
	}
	if commit.Title != "The pipeline failed" {
		t.Errorf("commit title = %q, want the body as a fallback", commit.Title)
	}
}

func TestMarkTodoDone(t *testing.T) {
	var called string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/todos/130/mark_as_done", func(w http.ResponseWriter, r *http.Request) {
		called = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	if err := client.MarkTodoDone(130); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != http.MethodPost {
		t.Errorf("marked done with %q, want POST", called)
	}
}

func TestMarkAllTodosDone(t *testing.T) {
	var called string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/todos/mark_as_done", func(w http.ResponseWriter, r *http.Request) {
		called = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	if err := client.MarkAllTodosDone(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != http.MethodPost {
		t.Errorf("cleared the list with %q, want POST", called)
	}
}
