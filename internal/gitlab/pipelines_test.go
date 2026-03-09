package gitlab

import (
	"fmt"
	"net/http"
	"testing"
)

func TestListPipelines_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		if got := r.URL.Query().Get("order_by"); got != "updated_at" {
			t.Errorf("want order_by=updated_at, got %q", got)
		}
		if got := r.URL.Query().Get("sort"); got != "desc" {
			t.Errorf("want sort=desc, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 100,
				"status": "success",
				"ref": "main",
				"sha": "abc123def456",
				"web_url": "https://gitlab.com/my-group/my-project/-/pipelines/100",
				"created_at": "2026-01-15T10:00:00Z",
				"updated_at": "2026-01-15T10:05:00Z"
			},
			{
				"id": 99,
				"status": "failed",
				"ref": "feature-branch",
				"sha": "def456abc789",
				"web_url": "https://gitlab.com/my-group/my-project/-/pipelines/99",
				"created_at": "2026-01-14T08:00:00Z",
				"updated_at": null
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	pipelines, err := client.ListPipelines(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 2 {
		t.Fatalf("want 2 pipelines, got %d", len(pipelines))
	}

	p := pipelines[0]
	if p.ID != 100 {
		t.Errorf("want ID 100, got %d", p.ID)
	}
	if p.Status != "success" {
		t.Errorf("want status success, got %q", p.Status)
	}
	if p.Ref != "main" {
		t.Errorf("want ref main, got %q", p.Ref)
	}
	if p.SHA != "abc123def456" {
		t.Errorf("want SHA abc123def456, got %q", p.SHA)
	}
	if p.CreatedAt.IsZero() {
		t.Error("want non-zero CreatedAt")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("want non-zero UpdatedAt")
	}

	// Second pipeline with null updated_at
	p2 := pipelines[1]
	if p2.ID != 99 {
		t.Errorf("want ID 99, got %d", p2.ID)
	}
	if !p2.UpdatedAt.IsZero() {
		t.Error("want zero UpdatedAt for null value")
	}
}

func TestListPipelines_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListPipelines(42)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestListPipelinesByRef_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ref"); got != "develop" {
			t.Errorf("want ref=develop, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 200,
				"status": "running",
				"ref": "develop",
				"sha": "aaa111",
				"web_url": "https://gitlab.com/p/-/pipelines/200",
				"created_at": "2026-02-01T12:00:00Z",
				"updated_at": "2026-02-01T12:01:00Z"
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	pipelines, err := client.ListPipelinesByRef(42, "develop")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 1 {
		t.Fatalf("want 1 pipeline, got %d", len(pipelines))
	}
	if pipelines[0].Ref != "develop" {
		t.Errorf("want ref develop, got %q", pipelines[0].Ref)
	}
}

func TestListPipelineJobs_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines/100/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("want GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": 1001,
				"name": "build",
				"stage": "build",
				"status": "success",
				"web_url": "https://gitlab.com/p/-/jobs/1001",
				"duration": 45.5,
				"created_at": "2026-01-15T10:00:00Z",
				"started_at": "2026-01-15T10:00:05Z"
			},
			{
				"id": 1002,
				"name": "test",
				"stage": "test",
				"status": "failed",
				"web_url": "https://gitlab.com/p/-/jobs/1002",
				"duration": 120.0,
				"created_at": "2026-01-15T10:01:00Z",
				"started_at": null
			}
		]`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	jobs, err := client.ListPipelineJobs(42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("want 2 jobs, got %d", len(jobs))
	}

	j := jobs[0]
	if j.ID != 1001 {
		t.Errorf("want ID 1001, got %d", j.ID)
	}
	if j.Name != "build" {
		t.Errorf("want name build, got %q", j.Name)
	}
	if j.Stage != "build" {
		t.Errorf("want stage build, got %q", j.Stage)
	}
	if j.Status != "success" {
		t.Errorf("want status success, got %q", j.Status)
	}
	if j.Duration != 45.5 {
		t.Errorf("want duration 45.5, got %f", j.Duration)
	}
	if j.CreatedAt.IsZero() {
		t.Error("want non-zero CreatedAt")
	}
	if j.StartedAt.IsZero() {
		t.Error("want non-zero StartedAt")
	}

	// Second job with null started_at
	j2 := jobs[1]
	if !j2.StartedAt.IsZero() {
		t.Error("want zero StartedAt for null value")
	}
}

func TestListPipelineJobs_serverError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines/100/jobs", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"400 Bad Request"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	_, err := client.ListPipelineJobs(42, 100)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestRetryPipeline_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines/100/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 100, "status": "pending"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	err := client.RetryPipeline(42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelPipeline_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipelines/100/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 100, "status": "canceled"}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	err := client.CancelPipeline(42, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPipeline_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/pipeline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 300,
			"status": "created",
			"ref": "main",
			"sha": "abc123",
			"web_url": "https://gitlab.com/p/-/pipelines/300",
			"created_at": "2026-03-01T12:00:00Z"
		}`))
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	p, err := client.RunPipeline(42, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != 300 {
		t.Errorf("want ID 300, got %d", p.ID)
	}
	if p.Status != "created" {
		t.Errorf("want status created, got %q", p.Status)
	}
	if p.Ref != "main" {
		t.Errorf("want ref main, got %q", p.Ref)
	}
}

func TestRetryJob_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/jobs/1001/retry", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id": 1001, "status": "pending"}`)
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	err := client.RetryJob(42, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCancelJob_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/jobs/1001/cancel", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id": 1001, "status": "canceled"}`)
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	err := client.CancelJob(42, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlayJob_success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/42/jobs/1001/play", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("want POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"id": 1001, "status": "pending"}`)
	})

	client, srv := setupTestClient(t, mux)
	defer srv.Close()

	err := client.PlayJob(42, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
