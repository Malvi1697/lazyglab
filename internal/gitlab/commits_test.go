package gitlab

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestListCommits_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": "f138bae6729508b923de684d5a8e4f8a72eda3f2",
				"short_id": "f138bae6",
				"title": "Fix the widget",
				"author_name": "Jan Novak",
				"created_at": "2026-07-20T10:30:00.000Z",
				"web_url": "https://gitlab.com/my-group/my-project/-/commit/f138bae6729508b923de684d5a8e4f8a72eda3f2"
			},
			{
				"id": "a1b2c3d4e5f6",
				"short_id": "a1b2c3d4",
				"title": "Add tests",
				"author_name": "Jana Novakova",
				"created_at": "2026-07-19T08:00:00.000Z",
				"web_url": "https://gitlab.com/my-group/my-project/-/commit/a1b2c3d4e5f6"
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	commits, err := client.ListCommits(1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("want 2 commits, got %d", len(commits))
	}

	c := commits[0]
	if c.ShortID != "f138bae6" {
		t.Errorf("want ShortID f138bae6, got %q", c.ShortID)
	}
	if c.Title != "Fix the widget" {
		t.Errorf("want Title 'Fix the widget', got %q", c.Title)
	}
	if c.AuthorName != "Jan Novak" {
		t.Errorf("want AuthorName 'Jan Novak', got %q", c.AuthorName)
	}
	wantTime := time.Date(2026, 7, 20, 10, 30, 0, 0, time.UTC)
	if !c.CreatedAt.Equal(wantTime) {
		t.Errorf("want CreatedAt %v, got %v", wantTime, c.CreatedAt)
	}
	if c.WebURL != "https://gitlab.com/my-group/my-project/-/commit/f138bae6729508b923de684d5a8e4f8a72eda3f2" {
		t.Errorf("WebURL mismatch, got %q", c.WebURL)
	}

	// Second commit preserves order.
	if commits[1].ShortID != "a1b2c3d4" {
		t.Errorf("want second commit ShortID a1b2c3d4, got %q", commits[1].ShortID)
	}
	if commits[1].Title != "Add tests" {
		t.Errorf("want second commit Title 'Add tests', got %q", commits[1].Title)
	}
}

func TestListCommits_withRef(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ref_name"); got != "feature-branch" {
			t.Errorf("want ref_name=feature-branch, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	commits, err := client.ListCommits(1, "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("want 0 commits, got %d", len(commits))
	}
}

func TestListCommits_emptyResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	commits, err := client.ListCommits(1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("want 0 commits, got %d", len(commits))
	}
}

func TestListCommits_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListCommits(1, "")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestListCommits_stripANSI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": "abc123",
			"short_id": "abc123",
			"title": "\u001b[31mmalicious\u001b[0m",
			"author_name": "eve",
			"created_at": "2026-07-20T10:30:00.000Z",
			"web_url": "https://gitlab.com/ns/p/-/commit/abc123"
		}]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	commits, err := client.ListCommits(1, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commits[0].Title != "malicious" {
		t.Errorf("ANSI not stripped: got %q", commits[0].Title)
	}
}

func TestGetCommit_includesFullMessage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/1/repository/commits/abc123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "abc123def456789",
			"short_id": "abc123d",
			"title": "fix: the thing",
			"message": "fix: the thing\n\nWith a longer explanation.\n",
			"author_name": "Jan",
			"web_url": "https://gitlab.example.com/g/p/-/commit/abc123def456789"
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	c, err := client.GetCommit(1, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.ID != "abc123def456789" {
		t.Errorf("ID = %q, want the full SHA", c.ID)
	}
	if !strings.Contains(c.Message, "longer explanation") {
		t.Errorf("Message = %q, want the full body", c.Message)
	}
}
