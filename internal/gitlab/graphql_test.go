package gitlab

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// stagesHandler answers the stages query and records what was asked for.
type stagesHandler struct {
	calls    int
	askedIDs []string
	body     string
}

func (h *stagesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/api/graphql") {
		http.NotFound(w, r)
		return
	}
	h.calls++

	raw, _ := io.ReadAll(r.Body)
	var req struct {
		Variables struct {
			IDs []string `json:"ids"`
		} `json:"variables"`
	}
	_ = json.Unmarshal(raw, &req)
	h.askedIDs = req.Variables.IDs

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(h.body))
}

const twoPipelineStages = `{"data":{"project":{"pipelines":{"nodes":[
	{"id":"gid://gitlab/Ci::Pipeline/724560","stages":{"nodes":[
		{"name":"lint","status":"success"},{"name":"build","status":"success"},{"name":"deploy","status":"skipped"}]}},
	{"id":"gid://gitlab/Ci::Pipeline/724553","stages":{"nodes":[
		{"name":"lint","status":"success"},{"name":"test","status":"failed"}]}}
]}}}}`

func TestPipelineStages_OneRequestForTheWholePage(t *testing.T) {
	// The whole point: the REST list has no stages, and asking per pipeline would be
	// one request per row on every refresh.
	h := &stagesHandler{body: twoPipelineStages}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	stages := client.PipelineStages("group/project", []Pipeline{
		{ID: 724560, Status: "success"},
		{ID: 724553, Status: "failed"},
	})

	if h.calls != 1 {
		t.Errorf("asked %d times, want one request for both pipelines", h.calls)
	}
	if len(h.askedIDs) != 2 || h.askedIDs[0] != "gid://gitlab/Ci::Pipeline/724560" {
		t.Errorf("asked for %v, want both pipelines as global ids", h.askedIDs)
	}
	if got := len(stages[724560]); got != 3 {
		t.Errorf("pipeline 724560 has %d stages, want 3", got)
	}
	if got := stages[724553]; len(got) != 2 || got[1].Name != "test" || got[1].Status != "failed" {
		t.Errorf("pipeline 724553 stages = %v, want lint/test with test failed", got)
	}
}

func TestPipelineStages_AFinishedPipelineIsAskedAboutOnce(t *testing.T) {
	// A finished pipeline's stages are as immutable as its verdict, so the
	// thirty-second refresh must not ask again.
	h := &stagesHandler{body: twoPipelineStages}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	finished := []Pipeline{{ID: 724560, Status: "success"}}
	client.PipelineStages("group/project", finished)
	stages := client.PipelineStages("group/project", finished)

	if h.calls != 1 {
		t.Errorf("asked %d times, want the second refresh to cost nothing", h.calls)
	}
	if len(stages[724560]) != 3 {
		t.Errorf("second call returned %v, want the cached stages", stages[724560])
	}
}

func TestPipelineStages_ARunningPipelineIsAskedAgain(t *testing.T) {
	// Its stages are exactly what is still changing.
	h := &stagesHandler{body: twoPipelineStages}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	running := []Pipeline{{ID: 724560, Status: "running"}}
	client.PipelineStages("group/project", running)
	client.PipelineStages("group/project", running)

	if h.calls != 2 {
		t.Errorf("asked %d times, want a running pipeline refetched", h.calls)
	}
}

func TestPipelineStages_AFailedQueryCostsOnlyTheMarks(t *testing.T) {
	// GraphQL answers HTTP 200 with an errors array, and an instance may not offer it
	// at all. Either way the list itself must still draw.
	h := &stagesHandler{body: `{"errors":[{"message":"Field 'stages' doesn't exist"}]}`}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	stages := client.PipelineStages("group/project", []Pipeline{{ID: 1, Status: "success"}})
	if len(stages) != 0 {
		t.Errorf("stages = %v, want none rather than a guess", stages)
	}
}

func TestGraphQLURL_SitsBesideTheRESTOne(t *testing.T) {
	c := &Client{baseURL: "https://gitlab.example.com/api/v4"}
	if got := c.graphqlURL(); got != "https://gitlab.example.com/api/graphql" {
		t.Errorf("graphqlURL() = %q", got)
	}
	// A base URL without the usual suffix still resolves to something askable.
	c = &Client{baseURL: "https://gitlab.example.com/"}
	if got := c.graphqlURL(); got != "https://gitlab.example.com/api/graphql" {
		t.Errorf("graphqlURL() = %q", got)
	}
}

func TestPipelineStages_BatchesAtGitLabsLimit(t *testing.T) {
	// GitLab refuses more than twenty pipelines per ids filter, and used to refuse the
	// whole query — a page of thirty silently arrived with no marks at all.
	h := &stagesHandler{body: twoPipelineStages}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	var page []Pipeline
	for id := 1; id <= 30; id++ {
		page = append(page, Pipeline{ID: id, Status: "running"})
	}
	client.PipelineStages("group/project", page)

	if h.calls != 2 {
		t.Errorf("asked %d times for 30 pipelines, want 2 batches of at most 20", h.calls)
	}
	if len(h.askedIDs) > maxStageIDs {
		t.Errorf("a batch asked about %d pipelines, want at most %d", len(h.askedIDs), maxStageIDs)
	}
}
