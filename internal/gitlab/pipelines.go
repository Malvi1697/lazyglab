package gitlab

import (
	"bytes"
	"sync"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// GetCommitTitle returns the first line of a commit message.
func (c *Client) GetCommitTitle(projectID int, sha string) string {
	commit, _, err := c.api.Commits.GetCommit(projectID, sha, nil)
	if err != nil || commit == nil {
		return ""
	}
	return commit.Title
}

// ListPipelines returns recent pipelines for a project.
func (c *Client) ListPipelines(projectID int) ([]Pipeline, error) {
	opts := &gogitlab.ListProjectPipelinesOptions{
		OrderBy: gogitlab.Ptr("updated_at"),
		Sort:    gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 30,
		},
	}

	apiPipelines, _, err := c.api.Pipelines.ListProjectPipelines(projectID, opts)
	if err != nil {
		return nil, err
	}

	pipelines := make([]Pipeline, len(apiPipelines))
	for i, p := range apiPipelines {
		pipelines[i] = Pipeline{
			ID:     int(p.ID),
			Status: util.StripANSI(p.Status),
			Ref:    util.StripANSI(p.Ref),
			SHA:    util.StripANSI(p.SHA),
			WebURL: util.StripANSI(p.WebURL),
		}
		if p.CreatedAt != nil {
			pipelines[i].CreatedAt = *p.CreatedAt
		}
		if p.UpdatedAt != nil {
			pipelines[i].UpdatedAt = *p.UpdatedAt
		}
	}

	c.fillCommitTitles(projectID, pipelines)
	return pipelines, nil
}

// fillCommitTitles fetches commit titles for pipelines concurrently,
// deduplicating by SHA.
func (c *Client) fillCommitTitles(projectID int, pipelines []Pipeline) {
	// Collect unique SHAs
	unique := make(map[string]struct{})
	for _, p := range pipelines {
		if p.SHA != "" {
			unique[p.SHA] = struct{}{}
		}
	}

	// Fetch titles concurrently
	titles := make(map[string]string, len(unique))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for sha := range unique {
		wg.Add(1)
		go func(sha string) {
			defer wg.Done()
			title := c.GetCommitTitle(projectID, sha)
			mu.Lock()
			titles[sha] = title
			mu.Unlock()
		}(sha)
	}
	wg.Wait()

	// Fill in
	for i := range pipelines {
		pipelines[i].CommitTitle = titles[pipelines[i].SHA]
	}
}

// ListPipelineJobs returns jobs for a specific pipeline.
func (c *Client) ListPipelineJobs(projectID, pipelineID int) ([]Job, error) {
	opts := &gogitlab.ListJobsOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
		},
	}

	apiJobs, _, err := c.api.Jobs.ListPipelineJobs(projectID, int64(pipelineID), opts)
	if err != nil {
		return nil, err
	}

	jobs := make([]Job, len(apiJobs))
	for i, j := range apiJobs {
		jobs[i] = Job{
			ID:       int(j.ID),
			Name:     util.StripANSI(j.Name),
			Stage:    util.StripANSI(j.Stage),
			Status:   util.StripANSI(j.Status),
			WebURL:   util.StripANSI(j.WebURL),
			Duration: j.Duration,
		}
		if j.CreatedAt != nil {
			jobs[i].CreatedAt = *j.CreatedAt
		}
		if j.StartedAt != nil {
			jobs[i].StartedAt = *j.StartedAt
		}
	}
	return jobs, nil
}

// RetryPipeline retries a pipeline.
func (c *Client) RetryPipeline(projectID, pipelineID int) error {
	_, _, err := c.api.Pipelines.RetryPipelineBuild(projectID, int64(pipelineID))
	return err
}

// CancelPipeline cancels a running pipeline.
func (c *Client) CancelPipeline(projectID, pipelineID int) error {
	_, _, err := c.api.Pipelines.CancelPipelineBuild(projectID, int64(pipelineID))
	return err
}

// RunPipeline creates and triggers a new pipeline on the given ref.
func (c *Client) RunPipeline(projectID int, ref string) (*Pipeline, error) {
	opts := &gogitlab.CreatePipelineOptions{
		Ref: gogitlab.Ptr(ref),
	}
	p, _, err := c.api.Pipelines.CreatePipeline(projectID, opts)
	if err != nil {
		return nil, err
	}
	result := &Pipeline{
		ID:     int(p.ID),
		Status: util.StripANSI(p.Status),
		Ref:    util.StripANSI(p.Ref),
		SHA:    util.StripANSI(p.SHA),
		WebURL: util.StripANSI(p.WebURL),
	}
	if p.CreatedAt != nil {
		result.CreatedAt = *p.CreatedAt
	}
	return result, nil
}

// RetryJob retries a single job.
func (c *Client) RetryJob(projectID, jobID int) error {
	_, _, err := c.api.Jobs.RetryJob(projectID, int64(jobID))
	return err
}

// CancelJob cancels a running job.
func (c *Client) CancelJob(projectID, jobID int) error {
	_, _, err := c.api.Jobs.CancelJob(projectID, int64(jobID))
	return err
}

// PlayJob triggers a manual job.
func (c *Client) PlayJob(projectID, jobID int) error {
	_, _, err := c.api.Jobs.PlayJob(projectID, int64(jobID), nil)
	return err
}

// GetJobTrace retrieves the log/trace output for a job.
func (c *Client) GetJobTrace(projectID, jobID int) (string, error) {
	reader, _, err := c.api.Jobs.GetTraceFile(projectID, int64(jobID))
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
