package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// countingHandler answers a pipeline list and the lookups it provokes, counting each
// kind of request.
type countingHandler struct {
	mu      sync.Mutex
	list    int
	titles  int
	commits int
	details int
}

func (h *countingHandler) counts() (list, titles, commits int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.list, h.titles, h.commits
}

func (h *countingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	h.mu.Lock()
	switch {
	case strings.HasSuffix(path, "/pipelines"):
		h.list++
	case strings.Contains(path, "/repository/commits/"):
		h.titles++
	case strings.HasSuffix(path, "/repository/commits"):
		h.commits++
	default:
		h.details++ // /pipelines/:id, the detailed status
	}
	h.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(path, "/pipelines"):
		var items []string
		for i := 0; i < 10; i++ {
			items = append(items, fmt.Sprintf(
				`{"id":%d,"sha":"%040d","ref":"main","status":"failed"}`, 100+i, i))
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	case strings.HasSuffix(path, "/repository/commits"):
		var items []string
		for i := 0; i < 10; i++ {
			items = append(items, fmt.Sprintf(
				`{"id":"%040d","short_id":"%08d","title":"commit %d"}`, i, i, i))
		}
		_, _ = w.Write([]byte("[" + strings.Join(items, ",") + "]"))
	case strings.Contains(path, "/repository/commits/"):
		_, _ = w.Write([]byte(`{"id":"x","short_id":"x","title":"one commit"}`))
	default:
		_, _ = w.Write([]byte(`{"id":100,"status":"failed"}`))
	}
}

func TestListPipelines_CommitTitlesAreResolvedInBulkAndCached(t *testing.T) {
	// The list endpoint carries no commit title, so every row needs one.
	h := &countingHandler{}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	pipelines, err := client.ListPipelines(42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pipelines) != 10 {
		t.Fatalf("got %d pipelines, want 10", len(pipelines))
	}
	if pipelines[0].CommitTitle != "commit 0" {
		t.Errorf("first row title = %q, want it resolved from the commit page", pipelines[0].CommitTitle)
	}

	list, titles, commits := h.counts()
	if list != 1 {
		t.Errorf("pipeline list fetched %d times, want once", list)
	}
	if commits != 1 {
		t.Errorf("commit page fetched %d times, want one bulk lookup", commits)
	}
	if titles != 0 {
		t.Errorf("%d per-SHA title lookups, want none once the page answered them all", titles)
	}

	// An auto-refresh of an unchanged list must cost the list request and nothing else at
	// all.
	if _, err := client.ListPipelines(42); err != nil {
		t.Fatalf("unexpected error on refresh: %v", err)
	}
	list2, titles2, commits2 := h.counts()
	if list2 != 2 {
		t.Errorf("pipeline list fetched %d times, want twice", list2)
	}
	if titles2 != titles || commits2 != commits {
		t.Errorf("a refresh asked for titles again (%d per-SHA, %d bulk); they are immutable",
			titles2-titles, commits2-commits)
	}
}

func TestListCommits_SeedsTheTitlesThePipelineListNeeds(t *testing.T) {
	// Overview loads both lists; the commits it already has must spare the pipelines any
	// lookup at all.
	h := &countingHandler{}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	if _, err := client.ListCommits(42, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := client.ListPipelines(42); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, titles, commits := h.counts()
	if commits != 1 {
		t.Errorf("commit page fetched %d times, want only the one Overview asked for", commits)
	}
	if titles != 0 {
		t.Errorf("%d per-SHA title lookups after a commit list, want none", titles)
	}
}
