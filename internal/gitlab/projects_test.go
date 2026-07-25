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

func TestGetProjectByPath_success(t *testing.T) {
	mux := http.NewServeMux()
	// The SDK URL-escapes the path into a single path segment.
	mux.HandleFunc("/api/v4/projects/my-group%2Fmy-project", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 42,
			"name": "my-project",
			"name_with_namespace": "My Group / my-project",
			"path_with_namespace": "my-group/my-project",
			"web_url": "https://gitlab.com/my-group/my-project",
			"default_branch": "main"
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	p, err := client.GetProjectByPath("my-group/my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 42 {
		t.Errorf("want ID 42, got %d", p.ID)
	}
	if p.NameWithNamespace != "My Group / my-project" {
		t.Errorf("want NameWithNamespace 'My Group / my-project', got %q", p.NameWithNamespace)
	}
	if p.DefaultBranch != "main" {
		t.Errorf("want DefaultBranch main, got %q", p.DefaultBranch)
	}
}

func TestGetProjectByPath_notFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Project Not Found"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	if _, err := client.GetProjectByPath("nope/nope"); err == nil {
		t.Fatal("expected an error for a missing project")
	}
}

func TestListProjects_paginatesAllPages(t *testing.T) {
	// A single page would hide the rest of the user's projects from the switcher,
	// so every page must be followed.
	var requestedPages []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		requestedPages = append(requestedPages, page)
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("want per_page=100, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "", "1":
			w.Header().Set("X-Next-Page", "2")
			_, _ = w.Write([]byte(`[{"id":1,"path_with_namespace":"g/one"}]`))
		case "2":
			w.Header().Set("X-Next-Page", "3")
			_, _ = w.Write([]byte(`[{"id":2,"path_with_namespace":"g/two"}]`))
		default:
			// Last page: no X-Next-Page header.
			_, _ = w.Write([]byte(`[{"id":3,"path_with_namespace":"g/three"}]`))
		}
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	projects, err := client.ListProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("want 3 projects across pages, got %d", len(projects))
	}
	if projects[2].PathWithNamespace != "g/three" {
		t.Errorf("want the last page's project, got %q", projects[2].PathWithNamespace)
	}
	if len(requestedPages) != 3 {
		t.Errorf("want 3 requests, got %d (%v)", len(requestedPages), requestedPages)
	}
}

func TestListProjects_stopsAtPageCap(t *testing.T) {
	// A server that always advertises another page must not loop forever.
	requests := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Next-Page", "99")
		_, _ = w.Write([]byte(`[{"id":1,"path_with_namespace":"g/p"}]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	if _, err := client.ListProjects(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != maxProjectPages {
		t.Errorf("want the fetch capped at %d requests, got %d", maxProjectPages, requests)
	}
}
