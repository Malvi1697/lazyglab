package gitlab

import (
	"net/http"
	"testing"
)

func TestListMergeRequests_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("state"); got != "opened" {
			t.Errorf("want state=opened, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"iid": 123,
				"title": "Fix authentication bug",
				"author": {"username": "alice"},
				"source_branch": "fix/auth",
				"target_branch": "main",
				"state": "opened",
				"draft": false,
				"web_url": "https://gitlab.com/p/-/merge_requests/123",
				"description": "Fixes the auth bug",
				"created_at": "2026-01-10T09:00:00Z",
				"updated_at": "2026-01-12T14:00:00Z"
			},
			{
				"iid": 124,
				"title": "Draft: Add tests",
				"author": null,
				"source_branch": "feature/tests",
				"target_branch": "main",
				"state": "opened",
				"draft": true,
				"web_url": "https://gitlab.com/p/-/merge_requests/124",
				"description": "",
				"created_at": "2026-01-11T10:00:00Z",
				"updated_at": null
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	mrs, err := client.ListMergeRequests(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mrs) != 2 {
		t.Fatalf("want 2 MRs, got %d", len(mrs))
	}

	// First MR — normal with author
	mr := mrs[0]
	if mr.IID != 123 {
		t.Errorf("want IID 123, got %d", mr.IID)
	}
	if mr.Title != "Fix authentication bug" {
		t.Errorf("want title 'Fix authentication bug', got %q", mr.Title)
	}
	if mr.Author != "alice" {
		t.Errorf("want author alice, got %q", mr.Author)
	}
	if mr.SourceBranch != "fix/auth" {
		t.Errorf("want source fix/auth, got %q", mr.SourceBranch)
	}
	if mr.TargetBranch != "main" {
		t.Errorf("want target main, got %q", mr.TargetBranch)
	}
	if mr.State != "opened" {
		t.Errorf("want state opened, got %q", mr.State)
	}
	if mr.Draft {
		t.Error("want draft=false")
	}
	if mr.Description != "Fixes the auth bug" {
		t.Errorf("want description 'Fixes the auth bug', got %q", mr.Description)
	}
	if mr.CreatedAt.IsZero() {
		t.Error("want non-zero CreatedAt")
	}
	if mr.UpdatedAt.IsZero() {
		t.Error("want non-zero UpdatedAt")
	}
	// ListMergeRequests uses BasicMergeRequest — Pipeline should be nil
	if mr.Pipeline != nil {
		t.Error("want nil Pipeline from list endpoint")
	}

	// Second MR — null author, draft
	mr2 := mrs[1]
	if mr2.Author != "" {
		t.Errorf("want empty author for null, got %q", mr2.Author)
	}
	if !mr2.Draft {
		t.Error("want draft=true")
	}
	if !mr2.UpdatedAt.IsZero() {
		t.Error("want zero UpdatedAt for null value")
	}
}

func TestListMergeRequests_emptyResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	mrs, err := client.ListMergeRequests(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mrs) != 0 {
		t.Errorf("want 0 MRs, got %d", len(mrs))
	}
}

func TestListMergeRequests_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListMergeRequests(42)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestGetMergeRequest_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests/123", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"iid": 123,
			"title": "Fix authentication bug",
			"author": {"username": "alice"},
			"source_branch": "fix/auth",
			"target_branch": "main",
			"state": "merged",
			"draft": false,
			"web_url": "https://gitlab.com/p/-/merge_requests/123",
			"description": "Full description here",
			"created_at": "2026-01-10T09:00:00Z",
			"updated_at": "2026-01-15T16:00:00Z",
			"pipeline": {
				"id": 500,
				"status": "success",
				"web_url": "https://gitlab.com/p/-/pipelines/500"
			}
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	mr, err := client.GetMergeRequest(42, 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.IID != 123 {
		t.Errorf("want IID 123, got %d", mr.IID)
	}
	if mr.Title != "Fix authentication bug" {
		t.Errorf("want title 'Fix authentication bug', got %q", mr.Title)
	}
	if mr.Author != "alice" {
		t.Errorf("want author alice, got %q", mr.Author)
	}
	if mr.State != "merged" {
		t.Errorf("want state merged, got %q", mr.State)
	}
	if mr.Description != "Full description here" {
		t.Errorf("want description 'Full description here', got %q", mr.Description)
	}

	// GetMergeRequest should include pipeline info
	if mr.Pipeline == nil {
		t.Fatal("want non-nil Pipeline")
	}
	if mr.Pipeline.ID != 500 {
		t.Errorf("want pipeline ID 500, got %d", mr.Pipeline.ID)
	}
	if mr.Pipeline.Status != "success" {
		t.Errorf("want pipeline status success, got %q", mr.Pipeline.Status)
	}
	if mr.Pipeline.WebURL != "https://gitlab.com/p/-/pipelines/500" {
		t.Errorf("want pipeline WebURL, got %q", mr.Pipeline.WebURL)
	}
}

func TestGetMergeRequest_noPipeline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests/124", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"iid": 124,
			"title": "Simple MR",
			"author": {"username": "bob"},
			"source_branch": "feat",
			"target_branch": "main",
			"state": "opened",
			"draft": false,
			"web_url": "https://gitlab.com/p/-/merge_requests/124",
			"description": "",
			"created_at": "2026-01-10T09:00:00Z",
			"updated_at": "2026-01-10T09:00:00Z",
			"pipeline": null
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	mr, err := client.GetMergeRequest(42, 124)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.Pipeline != nil {
		t.Error("want nil Pipeline for null pipeline")
	}
}

func TestGetMergeRequest_notFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests/999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.GetMergeRequest(42, 999)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestGetMergeRequest_nullAuthor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/merge_requests/125", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"iid": 125,
			"title": "MR with no author",
			"author": null,
			"source_branch": "feat",
			"target_branch": "main",
			"state": "opened",
			"draft": false,
			"web_url": "https://gitlab.com/p/-/merge_requests/125",
			"description": "",
			"created_at": "2026-01-10T09:00:00Z",
			"updated_at": "2026-01-10T09:00:00Z"
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	mr, err := client.GetMergeRequest(42, 125)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.Author != "" {
		t.Errorf("want empty author for null, got %q", mr.Author)
	}
}
