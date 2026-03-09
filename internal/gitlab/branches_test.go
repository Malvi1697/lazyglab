package gitlab

import (
	"net/http"
	"testing"
)

func TestListBranches_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"name": "feature-branch",
				"protected": false,
				"merged": false,
				"default": false,
				"web_url": "https://gitlab.com/p/-/tree/feature-branch",
				"commit": {
					"committed_date": "2026-01-14T12:00:00Z"
				}
			},
			{
				"name": "main",
				"protected": true,
				"merged": false,
				"default": true,
				"web_url": "https://gitlab.com/p/-/tree/main",
				"commit": {
					"committed_date": "2026-01-10T08:00:00Z"
				}
			},
			{
				"name": "old-branch",
				"protected": false,
				"merged": true,
				"default": false,
				"web_url": "https://gitlab.com/p/-/tree/old-branch",
				"commit": null
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	branches, err := client.ListBranches(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 3 {
		t.Fatalf("want 3 branches, got %d", len(branches))
	}

	// Default branch should be sorted to top
	if branches[0].Name != "main" {
		t.Errorf("want default branch 'main' first, got %q", branches[0].Name)
	}
	if !branches[0].Default {
		t.Error("want Default=true for first branch")
	}
	if !branches[0].Protected {
		t.Error("want Protected=true for main")
	}

	// After the default, branches should be sorted by LastActivity descending
	if branches[1].Name != "feature-branch" {
		t.Errorf("want 'feature-branch' second (more recent), got %q", branches[1].Name)
	}
	if branches[1].LastActivity.IsZero() {
		t.Error("want non-zero LastActivity for feature-branch")
	}

	// old-branch with null commit should have zero LastActivity
	if branches[2].Name != "old-branch" {
		t.Errorf("want 'old-branch' third, got %q", branches[2].Name)
	}
	if !branches[2].Merged {
		t.Error("want Merged=true for old-branch")
	}
	if !branches[2].LastActivity.IsZero() {
		t.Error("want zero LastActivity for branch with null commit")
	}
}

func TestListBranches_emptyResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	branches, err := client.ListBranches(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 0 {
		t.Errorf("want 0 branches, got %d", len(branches))
	}
}

func TestListBranches_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListBranches(42)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestListBranches_sortOrder(t *testing.T) {
	// Verify sorting: default branch always first, then by LastActivity descending
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/repository/branches", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"name": "oldest",
				"protected": false,
				"merged": false,
				"default": false,
				"web_url": "",
				"commit": {"committed_date": "2025-01-01T00:00:00Z"}
			},
			{
				"name": "newest",
				"protected": false,
				"merged": false,
				"default": false,
				"web_url": "",
				"commit": {"committed_date": "2026-06-01T00:00:00Z"}
			},
			{
				"name": "default-branch",
				"protected": true,
				"merged": false,
				"default": true,
				"web_url": "",
				"commit": {"committed_date": "2025-06-01T00:00:00Z"}
			},
			{
				"name": "middle",
				"protected": false,
				"merged": false,
				"default": false,
				"web_url": "",
				"commit": {"committed_date": "2026-01-01T00:00:00Z"}
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	branches, err := client.ListBranches(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"default-branch", "newest", "middle", "oldest"}
	for i, name := range expected {
		if branches[i].Name != name {
			t.Errorf("position %d: want %q, got %q", i, name, branches[i].Name)
		}
	}
}
