package gitlab

import (
	"net/http"
	"testing"
)

func TestListProjects_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		// Verify query params forwarded by the SDK
		if got := r.URL.Query().Get("membership"); got != "true" {
			t.Errorf("want membership=true, got %q", got)
		}
		if got := r.URL.Query().Get("order_by"); got != "last_activity_at" {
			t.Errorf("want order_by=last_activity_at, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 42,
				"name": "my-project",
				"name_with_namespace": "My Group / my-project",
				"path_with_namespace": "my-group/my-project",
				"web_url": "https://gitlab.com/my-group/my-project",
				"default_branch": "main"
			},
			{
				"id": 99,
				"name": "other-project",
				"name_with_namespace": "Other / other-project",
				"path_with_namespace": "other/other-project",
				"web_url": "https://gitlab.com/other/other-project",
				"default_branch": "master"
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("want 2 projects, got %d", len(projects))
	}

	// Verify int64 -> int conversion and field mapping
	p := projects[0]
	if p.ID != 42 {
		t.Errorf("want ID 42, got %d", p.ID)
	}
	if p.Name != "my-project" {
		t.Errorf("want name my-project, got %q", p.Name)
	}
	if p.NameWithNamespace != "My Group / my-project" {
		t.Errorf("want NameWithNamespace 'My Group / my-project', got %q", p.NameWithNamespace)
	}
	if p.PathWithNamespace != "my-group/my-project" {
		t.Errorf("want PathWithNamespace my-group/my-project, got %q", p.PathWithNamespace)
	}
	if p.WebURL != "https://gitlab.com/my-group/my-project" {
		t.Errorf("want WebURL https://gitlab.com/my-group/my-project, got %q", p.WebURL)
	}
	if p.DefaultBranch != "main" {
		t.Errorf("want DefaultBranch main, got %q", p.DefaultBranch)
	}

	// Second project
	if projects[1].ID != 99 {
		t.Errorf("want second project ID 99, got %d", projects[1].ID)
	}
}

func TestListProjects_emptyResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("want 0 projects, got %d", len(projects))
	}
}

func TestListProjects_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListProjects()
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestListProjects_stripANSI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Project name contains an ANSI escape sequence
		_, _ = w.Write([]byte(`[{
			"id": 1,
			"name": "\u001b[31mmalicious\u001b[0m",
			"name_with_namespace": "ns",
			"path_with_namespace": "ns/p",
			"web_url": "https://gitlab.com/ns/p",
			"default_branch": "main"
		}]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects[0].Name != "malicious" {
		t.Errorf("ANSI not stripped: got %q", projects[0].Name)
	}
}
