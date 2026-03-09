package gitlab

import "time"

// Project represents a GitLab project.
type Project struct {
	ID                int
	Name              string
	NameWithNamespace string
	WebURL            string
	DefaultBranch     string
}

// MergeRequest represents a GitLab merge request.
type MergeRequest struct {
	IID          int
	Title        string
	Author       string
	SourceBranch string
	TargetBranch string
	State        string
	Draft        bool
	Pipeline     *PipelineInfo
	Approvals    int
	WebURL       string
	Description  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PipelineInfo is a summary of a pipeline (embedded in MR, etc.).
type PipelineInfo struct {
	ID     int
	Status string
	WebURL string
}

// Pipeline represents a full GitLab pipeline.
type Pipeline struct {
	ID        int
	Status    string
	Ref       string
	SHA       string
	WebURL    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Job represents a CI/CD job within a pipeline.
type Job struct {
	ID        int
	Name      string
	Stage     string
	Status    string
	WebURL    string
	Duration  float64
	CreatedAt time.Time
	StartedAt time.Time
}

// Issue represents a GitLab issue.
type Issue struct {
	IID         int
	Title       string
	Author      string
	State       string
	Labels      []string
	Assignees   []string
	WebURL      string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
