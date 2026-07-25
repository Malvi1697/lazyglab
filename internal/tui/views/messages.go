package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// --- Messages for async data loading ---

// ProjectsLoadedMsg is sent when projects have been fetched.
type ProjectsLoadedMsg struct {
	Projects []gitlab.Project
	Err      error
}

// ProjectSelectedMsg is sent when the user selects a project.
type ProjectSelectedMsg struct {
	Project gitlab.Project
}

// MRsLoadedMsg is sent when merge requests have been fetched.
type MRsLoadedMsg struct {
	MRs []gitlab.MergeRequest
	Err error
}

// PipelinesLoadedMsg is sent when pipelines have been fetched.
type PipelinesLoadedMsg struct {
	Pipelines []gitlab.Pipeline
	Err       error
}

// JobsLoadedMsg is sent when pipeline jobs have been fetched.
type JobsLoadedMsg struct {
	Jobs []gitlab.Job
	Err  error
}

// IssuesLoadedMsg is sent when issues have been fetched.
type IssuesLoadedMsg struct {
	Issues []gitlab.Issue
	Err    error
}

// BranchesLoadedMsg is sent when branches have been fetched.
type BranchesLoadedMsg struct {
	Branches []gitlab.Branch
	Err      error
}

// BranchSelectedMsg is sent when a branch is selected from the picker.
type BranchSelectedMsg struct {
	Branch gitlab.Branch
}

// StatusMsg is sent to display a status message in the status bar.
type StatusMsg struct {
	Text  string
	IsErr bool
}

// JobActionDoneMsg is sent after a job action completes to refresh the job list.
type JobActionDoneMsg struct {
	Text  string
	IsErr bool
}

// PipelineActionDoneMsg is sent after a pipeline action completes to refresh pipelines.
type PipelineActionDoneMsg struct {
	Text  string
	IsErr bool
}

// JobTraceLoadedMsg is sent when a job's log/trace has been fetched.
type JobTraceLoadedMsg struct {
	Trace string
	Err   error
}

// ErrorMsg represents an error from an async operation.
type ErrorMsg struct {
	Err error
}

// CommitsLoadedMsg is added in Task 3 once gitlab.Commit exists.

// ConfirmMsg asks the shell to show a confirmation dialog for a destructive
// action; the action command runs only if the user confirms.
type ConfirmMsg struct {
	Prompt string
	Action tea.Cmd
}
