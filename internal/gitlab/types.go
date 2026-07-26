package gitlab

import "time"

// Project represents a GitLab project.
type Project struct {
	ID                int
	Name              string
	NameWithNamespace string
	PathWithNamespace string
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
	ID          int
	Status      string
	Ref         string
	SHA         string
	CommitTitle string
	WebURL      string
	CreatedAt   time.Time
	UpdatedAt   time.Time

	// StatusLabel is GitLab's own wording for the status, e.g. "passed with
	// warnings" — a success whose allowed-to-fail jobs failed. Only the single
	// pipeline endpoint reports it, so it is empty for pipelines from a list.
	StatusLabel string
	// HasWarnings is true when StatusLabel describes a success with warnings.
	HasWarnings bool
}

// FileDiff is one file's change within a commit, as a unified diff.
type FileDiff struct {
	OldPath string
	NewPath string
	Diff    string // unified diff text; empty when GitLab withheld it
	New     bool
	Deleted bool
	Renamed bool
	// Withheld is true when GitLab excluded the diff because the change is too
	// large to send, so an empty Diff is not mistaken for an empty change.
	Withheld bool

	Added   int // counted from the diff text
	Removed int
}

// Path is the file's current path, or its old one if it was deleted.
func (f FileDiff) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

// CommitRef is a branch or tag that contains a commit.
type CommitRef struct {
	Type string // "branch" or "tag"
	Name string
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

// Branch represents a GitLab repository branch.
type Branch struct {
	Name         string
	Protected    bool
	Merged       bool
	Default      bool
	WebURL       string
	LastActivity time.Time // commit date for sorting
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

// Commit is a repository commit.
type Commit struct {
	ID         string // full SHA, used for copying and exact pipeline matching
	ShortID    string
	Message    string   // full commit message; only set by GetCommit
	ParentIDs  []string // parent SHAs; only set by GetCommit
	Title      string
	AuthorName string
	CreatedAt  time.Time
	WebURL     string
	Status     string // CI status, resolved by callers from pipelines by SHA ("" if none)
}

// Todo is one item on the user's GitLab To-Do list: something waiting on them,
// in any project. Action says why it is there ("review_requested",
// "build_failed", "mentioned", …) and Target what kind of thing it points at.
type Todo struct {
	ID          int
	Action      string
	Target      string // "MergeRequest", "Issue", "Commit", …
	Reference   string // "!42" / "#7", empty for targets GitLab does not number
	Title       string
	TargetState string // the target's own state, e.g. "opened"
	ProjectPath string
	Author      string
	Body        string
	State       string // always "pending" for what we fetch
	WebURL      string
	CreatedAt   time.Time
}
