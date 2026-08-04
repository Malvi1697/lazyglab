package gitlab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// GraphQL exists for one reason: the REST list endpoints do not carry a pipeline's
// stages, and asking per pipeline would be thirty requests for one screen.
const (
	graphqlTimeout = 10 * time.Second
	// GitLab refuses more than twenty pipelines per ids filter ("Cannot query more than 20
	// pipelines by ID at once"), so a page of thirty is two requests rather than thirty.
	maxStageIDs = 20
	// A page of thirty pipelines with their stages is tens of kilobytes; well past that
	// and something other than the API is answering.
	maxGraphQLResponse = 8 << 20
)

// graphqlURL is the endpoint beside the REST one: .../api/v4 -> .../api/graphql.
func (c *Client) graphqlURL() string {
	if i := strings.LastIndex(c.baseURL, "/api/"); i >= 0 {
		return c.baseURL[:i] + "/api/graphql"
	}
	return strings.TrimSuffix(c.baseURL, "/") + "/api/graphql"
}

// graphql runs one query and decodes data into out.
func (c *Client) graphql(query string, variables map[string]any, out any) error {
	if c.token == "" {
		return fmt.Errorf("graphql: no token")
	}

	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("graphql: encoding query: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.graphqlURL(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("graphql: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: graphqlTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("graphql: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql: HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxGraphQLResponse)).Decode(&envelope); err != nil {
		return fmt.Errorf("graphql: reading the answer: %w", err)
	}
	// GraphQL answers 200 with an errors array, so a failed query looks like a success
	// until this is checked.
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("graphql: %s", envelope.Errors[0].Message)
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("graphql: empty answer")
	}
	return json.Unmarshal(envelope.Data, out)
}

// pipelineStagesQuery asks for the stages of specific pipelines, and for the status of
// every job in them.
const pipelineStagesQuery = `query ($path: ID!, $ids: [ID!]) {
  project(fullPath: $path) {
    pipelines(ids: $ids, first: 100) {
      nodes {
        id
        stages { nodes { name jobs { nodes { status } } } }
      }
    }
  }
}`

// PipelineStages returns the stages of each given pipeline, keyed by pipeline ID.
func (c *Client) PipelineStages(projectPath string, pipelines []Pipeline) map[int][]Stage {
	stages := make(map[int][]Stage, len(pipelines))

	var ask []Pipeline
	for _, p := range pipelines {
		if cached, ok := c.stageCache.Load(p.ID); ok {
			stages[p.ID] = cached.([]Stage)
			continue
		}
		ask = append(ask, p)
	}
	if len(ask) == 0 || projectPath == "" {
		return stages
	}

	for start := 0; start < len(ask); start += maxStageIDs {
		batch := ask[start:min(start+maxStageIDs, len(ask))]
		c.readStages(projectPath, batch, stages)
	}

	// Cache only what cannot change again.
	for _, p := range ask {
		if list, ok := stages[p.ID]; ok && isFinished(p.Status) {
			c.stageCache.Store(p.ID, list)
		}
	}
	return stages
}

// readStages asks about one batch of pipelines and adds what comes back to stages.
func (c *Client) readStages(projectPath string, batch []Pipeline, stages map[int][]Stage) {
	ids := make([]string, len(batch))
	for i, p := range batch {
		ids[i] = "gid://gitlab/Ci::Pipeline/" + strconv.Itoa(p.ID)
	}

	var answer struct {
		Project struct {
			Pipelines struct {
				Nodes []struct {
					ID     string `json:"id"`
					Stages struct {
						Nodes []struct {
							Name string `json:"name"`
							Jobs struct {
								Nodes []struct {
									Status string `json:"status"`
								} `json:"nodes"`
							} `json:"jobs"`
						} `json:"nodes"`
					} `json:"stages"`
				} `json:"nodes"`
			} `json:"pipelines"`
		} `json:"project"`
	}
	if err := c.graphql(pipelineStagesQuery, map[string]any{"path": projectPath, "ids": ids}, &answer); err != nil {
		// The marks are an extra; an instance with GraphQL disabled, or a query this version
		// rejects, must not cost the list itself.
		return
	}

	for _, node := range answer.Project.Pipelines.Nodes {
		id := pipelineIDFromGID(node.ID)
		if id == 0 {
			continue
		}
		list := make([]Stage, 0, len(node.Stages.Nodes))
		for _, s := range node.Stages.Nodes {
			jobs := make([]string, 0, len(s.Jobs.Nodes))
			for _, j := range s.Jobs.Nodes {
				// GraphQL shouts its enums: SUCCESS, CANCELED.
				jobs = append(jobs, strings.ToLower(util.StripANSI(j.Status)))
			}
			list = append(list, Stage{
				Name:   util.StripANSI(s.Name),
				Status: StageStatus(jobs),
				Jobs:   len(jobs),
			})
		}
		stages[id] = list
	}
}

// StageStatus is how a stage went, given how its jobs went: the worst thing that
// happened to any of them, except that anything still moving wins.
func StageStatus(jobs []string) string {
	// Read down the list: the first state present is the one the stage is in.
	for _, want := range []string{
		"running",
		"failed",
		"canceled", "canceling",
		"pending", "created", "waiting_for_resource", "preparing", "scheduled",
		"manual",
		"success",
	} {
		for _, got := range jobs {
			if got != want {
				continue
			}
			switch want {
			case "canceling":
				return "canceled"
			case "created", "waiting_for_resource", "preparing", "scheduled":
				return "pending"
			}
			return want
		}
	}
	// No jobs at all, or only skipped ones: nothing ran here.
	return "skipped"
}

// pipelineIDFromGID reads the numeric id out of "gid://gitlab/Ci::Pipeline/724560".
func pipelineIDFromGID(gid string) int {
	i := strings.LastIndexByte(gid, '/')
	if i < 0 {
		return 0
	}
	id, err := strconv.Atoi(gid[i+1:])
	if err != nil {
		return 0
	}
	return id
}

// isFinished reports whether a pipeline has reached a state it will not leave.
func isFinished(status string) bool {
	switch status {
	case "success", "failed", "canceled", "skipped":
		return true
	}
	return false
}
