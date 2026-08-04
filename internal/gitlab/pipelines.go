package gitlab

import (
	"bytes"
	"strings"
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

	pipelines := toPipelines(apiPipelines)

	c.fillCommitTitles(projectID, pipelines)
	c.fillWarnings(projectID, pipelines)
	return pipelines, nil
}

// toPipelines maps a page of API pipelines into domain ones.
func toPipelines(apiPipelines []*gogitlab.PipelineInfo) []Pipeline {
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
	return pipelines
}

// fillCommitTitles gives each pipeline the title of the commit it built.
func (c *Client) fillCommitTitles(projectID int, pipelines []Pipeline) {
	// The SHAs we have never resolved.
	missing := make(map[string]struct{})
	for i, p := range pipelines {
		if p.SHA == "" {
			continue
		}
		if title, ok := c.titleCache.Load(p.SHA); ok {
			pipelines[i].CommitTitle = title.(string)
			continue
		}
		missing[p.SHA] = struct{}{}
	}
	if len(missing) == 0 {
		return
	}

	// One page of commits answers up to fifty SHAs for a single request, so it beats
	// asking per SHA the moment more than a couple are missing.
	if len(missing) > 2 {
		if _, err := c.ListCommits(projectID, ""); err == nil {
			for sha := range missing {
				if _, ok := c.titleCache.Load(sha); ok {
					delete(missing, sha)
				}
			}
		}
	}

	// Fetch concurrently, but bounded, so a page of new pipelines doesn't fan out into
	// dozens of simultaneous requests and trip GitLab's rate limiting.
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for sha := range missing {
		wg.Add(1)
		go func(sha string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if title := c.GetCommitTitle(projectID, sha); title != "" {
				c.titleCache.Store(sha, title)
			}
		}(sha)
	}
	wg.Wait()

	for i := range pipelines {
		if title, ok := c.titleCache.Load(pipelines[i].SHA); ok {
			pipelines[i].CommitTitle = title.(string)
		}
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

// ListPipelinesBySHA returns the pipelines run for one commit.
func (c *Client) ListPipelinesBySHA(projectID int, sha string) ([]Pipeline, error) {
	opts := &gogitlab.ListProjectPipelinesOptions{
		SHA:         gogitlab.Ptr(sha),
		ListOptions: gogitlab.ListOptions{PerPage: 20},
	}

	apiPipelines, _, err := c.api.Pipelines.ListProjectPipelines(projectID, opts)
	if err != nil {
		return nil, err
	}

	pipelines := toPipelines(apiPipelines)

	c.fillWarnings(projectID, pipelines)
	return pipelines, nil
}

// GetPipeline returns one pipeline including GitLab's detailed status.
func (c *Client) GetPipeline(projectID, pipelineID int) (*Pipeline, error) {
	p, _, err := c.api.Pipelines.GetPipeline(projectID, int64(pipelineID))
	if err != nil {
		return nil, err
	}

	pipeline := &Pipeline{
		ID:     int(p.ID),
		Status: util.StripANSI(p.Status),
		Ref:    util.StripANSI(p.Ref),
		SHA:    util.StripANSI(p.SHA),
		WebURL: util.StripANSI(p.WebURL),
	}
	if p.CreatedAt != nil {
		pipeline.CreatedAt = *p.CreatedAt
	}
	if p.UpdatedAt != nil {
		pipeline.UpdatedAt = *p.UpdatedAt
	}
	if d := p.DetailedStatus; d != nil {
		pipeline.StatusLabel = util.StripANSI(d.Label)
		// GitLab groups this as "success-with-warnings"; match loosely so a wording change
		// does not silently drop the distinction.
		pipeline.HasWarnings = strings.Contains(d.Group, "warning") ||
			strings.Contains(strings.ToLower(d.Label), "warning")
	}
	return pipeline, nil
}

// fillWarnings marks the pipelines that succeeded with warnings.
func (c *Client) fillWarnings(projectID int, pipelines []Pipeline) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)

	for i := range pipelines {
		// Only a success can be a success-with-warnings, and an unfinished one would have to
		// be asked again anyway.
		if pipelines[i].Status != "success" {
			continue
		}
		if v, ok := c.warningCache.Load(pipelines[i].ID); ok {
			verdict := v.(pipelineVerdict)
			pipelines[i].StatusLabel = verdict.label
			pipelines[i].HasWarnings = verdict.warnings
			continue
		}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			full, err := c.GetPipeline(projectID, pipelines[i].ID)
			if err != nil {
				return
			}
			c.warningCache.Store(pipelines[i].ID, pipelineVerdict{
				label: full.StatusLabel, warnings: full.HasWarnings,
			})
			pipelines[i].StatusLabel = full.StatusLabel
			pipelines[i].HasWarnings = full.HasWarnings
		}(i)
	}
	wg.Wait()
}
