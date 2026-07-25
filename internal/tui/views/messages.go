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

// CommitsLoadedMsg carries fetched commits.
type CommitsLoadedMsg struct {
	Commits []gitlab.Commit
	Err     error
}

// ConfirmMsg asks the shell to show a confirmation dialog for a destructive
// action; the action command runs only if the user confirms.
type ConfirmMsg struct {
	Prompt string
	Action tea.Cmd
}

// CommitDetailLoadedMsg carries a commit's full message and the pipelines run for
// it (GitLab creates pipelines per ref, so there may be none).
type CommitDetailLoadedMsg struct {
	SHA       string
	Commit    *gitlab.Commit
	Pipelines []gitlab.Pipeline
	Refs      []gitlab.CommitRef
	MRs       []gitlab.MergeRequest
	Jobs      []gitlab.Job
	Diffs     []gitlab.FileDiff
	Err       error
}

// LoadErr returns the error carried by any message that reports the outcome of
// a data load, or nil for messages that carry none. Every such message passes
// through the shell before being delegated to a view, so this gives the shell a
// single place to notice failures (e.g. an unusable token) regardless of which
// view triggered the request.
func LoadErr(msg tea.Msg) error {
	switch m := msg.(type) {
	case ProjectsLoadedMsg:
		return m.Err
	case BranchesLoadedMsg:
		return m.Err
	case PipelinesLoadedMsg:
		return m.Err
	case JobsLoadedMsg:
		return m.Err
	case JobTraceLoadedMsg:
		return m.Err
	case MRsLoadedMsg:
		return m.Err
	case IssuesLoadedMsg:
		return m.Err
	case CommitsLoadedMsg:
		return m.Err
	case CommitDetailLoadedMsg:
		return m.Err
	case ErrorMsg:
		return m.Err
	}
	return nil
}
