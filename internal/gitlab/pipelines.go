package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

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
			Status: p.Status,
			Ref:    p.Ref,
			SHA:    p.SHA,
			WebURL: p.WebURL,
		}
		if p.CreatedAt != nil {
			pipelines[i].CreatedAt = *p.CreatedAt
		}
		if p.UpdatedAt != nil {
			pipelines[i].UpdatedAt = *p.UpdatedAt
		}
	}
	return pipelines, nil
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
			Name:     j.Name,
			Stage:    j.Stage,
			Status:   j.Status,
			WebURL:   j.WebURL,
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
		Status: p.Status,
		Ref:    p.Ref,
		SHA:    p.SHA,
		WebURL: p.WebURL,
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
