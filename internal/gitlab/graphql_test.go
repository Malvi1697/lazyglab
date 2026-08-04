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

// The shape GitLab really sends: stages carrying their jobs, statuses shouted.
const twoPipelineStages = `{"data":{"project":{"pipelines":{"nodes":[
	{"id":"gid://gitlab/Ci::Pipeline/724560","stages":{"nodes":[
		{"name":"lint","jobs":{"nodes":[{"status":"SUCCESS"},{"status":"SUCCESS"}]}},
		{"name":"build","jobs":{"nodes":[{"status":"SUCCESS"}]}},
		{"name":"deploy","jobs":{"nodes":[{"status":"SKIPPED"}]}}]}},
	{"id":"gid://gitlab/Ci::Pipeline/724553","stages":{"nodes":[
		{"name":"lint","jobs":{"nodes":[{"status":"SUCCESS"}]}},
		{"name":"test","jobs":{"nodes":[{"status":"FAILED"},{"status":"SUCCESS"}]}}]}}
]}}}}`

// The pipeline that started all this: stage 5 to 7 hold nothing but canceled jobs, and
// GitLab's own CiStage.status calls every one of them a success.
const canceledPipelineStages = `{"data":{"project":{"pipelines":{"nodes":[
	{"id":"gid://gitlab/Ci::Pipeline/724403","stages":{"nodes":[
		{"name":"lint","status":"success","jobs":{"nodes":[{"status":"SUCCESS"},{"status":"SUCCESS"}]}},
		{"name":"build","status":"canceled","jobs":{"nodes":[{"status":"CANCELED"},{"status":"SUCCESS"}]}},
		{"name":"security","status":"canceled","jobs":{"nodes":[{"status":"FAILED"},{"status":"CANCELED"}]}},
		{"name":"staging","status":"success","jobs":{"nodes":[{"status":"CANCELED"},{"status":"CANCELED"}]}},
		{"name":"production","status":"success","jobs":{"nodes":[{"status":"CANCELED"}]}},
		{"name":"operations","status":"success","jobs":{"nodes":[{"status":"CANCELED"}]}}]}}
]}}}}`

func TestPipelineStages_OneRequestForTheWholePage(t *testing.T) {
	// The whole point: the REST list has no stages, and asking per pipeline would be one
	// request per row on every refresh.
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
	if got := stages[724560][0].Jobs; got != 2 {
		t.Errorf("lint holds %d jobs, want 2", got)
	}
}

func TestPipelineStages_ACanceledStageIsNotGreen(t *testing.T) {
	// The bug this replaced: the row showed three green marks for a pipeline whose last
	// three stages held nothing but canceled jobs, which is what CiStage.status claimed.
	h := &stagesHandler{body: canceledPipelineStages}
	client, srv := setupTestClient(t, h)
	defer srv.Close()

	stages := client.PipelineStages("group/project", []Pipeline{{ID: 724403, Status: "canceled"}})

	want := []string{"success", "canceled", "failed", "canceled", "canceled", "canceled"}
	got := stages[724403]
	if len(got) != len(want) {
		t.Fatalf("got %d stages, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Status != w {
			t.Errorf("stage %q = %q, want %q", got[i].Name, got[i].Status, w)
		}
	}
}

func TestStageStatus_TheWorstThingThatHappened(t *testing.T) {
	tests := []struct {
		name string
		jobs []string
		want string
	}{
		{"all passed", []string{"success", "success"}, "success"},
		{"one failed", []string{"success", "failed"}, "failed"},
		{"all canceled", []string{"canceled", "canceled"}, "canceled"},
		{"canceled and passed", []string{"canceled", "success"}, "canceled"},
		// Anything still moving wins: the stage has not finished failing yet.
		{"failed but still running", []string{"failed", "running"}, "running"},
		{"waiting to start", []string{"created", "created"}, "pending"},
		{"manual", []string{"manual", "success"}, "manual"},
		{"nothing ran", []string{"skipped", "skipped"}, "skipped"},
		{"no jobs at all", nil, "skipped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StageStatus(tt.jobs); got != tt.want {
				t.Errorf("StageStatus(%v) = %q, want %q", tt.jobs, got, tt.want)
			}
		})
	}
}

func TestPipelineStages_AFinishedPipelineIsAskedAboutOnce(t *testing.T) {
	// A finished pipeline's stages are as immutable as its verdict, so the thirty-second
	// refresh must not ask again.
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
	// GraphQL answers HTTP 200 with an errors array, and an instance may not offer it at
	// all.
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
