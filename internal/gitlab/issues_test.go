package gitlab

import (
	"net/http"
	"testing"
)

func TestListIssues_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/issues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("state"); got != "opened" {
			t.Errorf("want state=opened, got %q", got)
		}
		if got := r.URL.Query().Get("order_by"); got != "updated_at" {
			t.Errorf("want order_by=updated_at, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 1001,
				"iid": 89,
				"title": "Bug in login",
				"author": {"username": "alice"},
				"state": "opened",
				"labels": ["bug", "critical"],
				"assignees": [
					{"username": "bob"},
					{"username": "carol"}
				],
				"web_url": "https://gitlab.com/p/-/issues/89",
				"description": "Login page crashes",
				"created_at": "2026-01-05T08:00:00Z",
				"updated_at": "2026-01-06T09:00:00Z"
			},
			{
				"id": 1002,
				"iid": 90,
				"title": "Feature request",
				"author": {"username": ""},
				"state": "opened",
				"labels": [],
				"assignees": [],
				"web_url": "https://gitlab.com/p/-/issues/90",
				"description": "",
				"created_at": "2026-01-07T10:00:00Z",
				"updated_at": "2026-01-07T10:00:00Z"
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	issues, err := client.ListIssues(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 {
		t.Fatalf("want 2 issues, got %d", len(issues))
	}

	// First issue — full data
	issue := issues[0]
	if issue.IID != 89 {
		t.Errorf("want IID 89, got %d", issue.IID)
	}
	if issue.Title != "Bug in login" {
		t.Errorf("want title 'Bug in login', got %q", issue.Title)
	}
	if issue.Author != "alice" {
		t.Errorf("want author alice, got %q", issue.Author)
	}
	if issue.State != "opened" {
		t.Errorf("want state opened, got %q", issue.State)
	}
	if issue.Description != "Login page crashes" {
		t.Errorf("want description 'Login page crashes', got %q", issue.Description)
	}
	if issue.CreatedAt.IsZero() {
		t.Error("want non-zero CreatedAt")
	}
	if issue.UpdatedAt.IsZero() {
		t.Error("want non-zero UpdatedAt")
	}

	// Labels
	if len(issue.Labels) != 2 {
		t.Fatalf("want 2 labels, got %d", len(issue.Labels))
	}
	if issue.Labels[0] != "bug" {
		t.Errorf("want first label 'bug', got %q", issue.Labels[0])
	}
	if issue.Labels[1] != "critical" {
		t.Errorf("want second label 'critical', got %q", issue.Labels[1])
	}

	// Assignees
	if len(issue.Assignees) != 2 {
		t.Fatalf("want 2 assignees, got %d", len(issue.Assignees))
	}
	if issue.Assignees[0] != "bob" {
		t.Errorf("want first assignee bob, got %q", issue.Assignees[0])
	}
	if issue.Assignees[1] != "carol" {
		t.Errorf("want second assignee carol, got %q", issue.Assignees[1])
	}

	// Second issue — null author, empty labels/assignees
	issue2 := issues[1]
	if issue2.Author != "" {
		t.Errorf("want empty author for null, got %q", issue2.Author)
	}
	if len(issue2.Labels) != 0 {
		t.Errorf("want 0 labels, got %d", len(issue2.Labels))
	}
	if len(issue2.Assignees) != 0 {
		t.Errorf("want 0 assignees, got %d", len(issue2.Assignees))
	}
}

func TestListIssues_emptyResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	issues, err := client.ListIssues(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("want 0 issues, got %d", len(issues))
	}
}

func TestListIssues_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/issues", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListIssues(42)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestListIssues_stripANSI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/issues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": 2001,
			"iid": 1,
			"title": "\u001b[31mred title\u001b[0m",
			"author": {"username": "\u001b[32mhacker\u001b[0m"},
			"state": "opened",
			"labels": ["\u001b[34mblue-label\u001b[0m"],
			"assignees": [{"username": "\u001b[33mevil\u001b[0m"}],
			"web_url": "https://gitlab.com/p/-/issues/1",
			"description": "clean",
			"created_at": "2026-01-01T00:00:00Z",
			"updated_at": "2026-01-01T00:00:00Z"
		}]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	issues, err := client.ListIssues(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues[0].Title != "red title" {
		t.Errorf("ANSI not stripped from title: got %q", issues[0].Title)
	}
	if issues[0].Author != "hacker" {
		t.Errorf("ANSI not stripped from author: got %q", issues[0].Author)
	}
	if issues[0].Labels[0] != "blue-label" {
		t.Errorf("ANSI not stripped from label: got %q", issues[0].Labels[0])
	}
	if issues[0].Assignees[0] != "evil" {
		t.Errorf("ANSI not stripped from assignee: got %q", issues[0].Assignees[0])
	}
}
