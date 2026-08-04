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

// PipelineStagesLoadedMsg carries the stages of the pipelines on screen, so the list
// can say how far each one got.
type PipelineStagesLoadedMsg struct {
	Stages map[int][]gitlab.Stage
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

// TodosLoadedMsg is sent when the user's to-do list has been fetched.
type TodosLoadedMsg struct {
	Todos []gitlab.Todo
	Err   error
}

// TodoActionDoneMsg is sent after a to-do was cleared, so the list can be refetched:
// the row is gone on GitLab's side.
type TodoActionDoneMsg struct {
	Text  string
	IsErr bool
}

// MRDetailLoadedMsg carries everything the merge-request page shows: the merge request
// itself, its approvals, its pipeline with that pipeline's jobs, and its changed files.
type MRDetailLoadedMsg struct {
	IID       int
	MR        *gitlab.MergeRequest
	Approvals *gitlab.MRApprovals
	Pipeline  *gitlab.Pipeline
	Jobs      []gitlab.Job
	Diffs     []gitlab.FileDiff
	Notes     []gitlab.Note
	Err       error
}

// MRActionDoneMsg is sent after approving or merging, so the page can refetch: both
// change what it says about itself.
type MRActionDoneMsg struct {
	Text  string
	IsErr bool
}

// IssueNotesLoadedMsg carries an issue's discussion.
type IssueNotesLoadedMsg struct {
	IID   int
	Notes []gitlab.Note
	Err   error
}

// IssueActionDoneMsg is sent after commenting on an issue, so the discussion can be
// refetched.
type IssueActionDoneMsg struct {
	Text  string
	IsErr bool
}

// ReadmeLoadedMsg carries a project's README.
type ReadmeLoadedMsg struct {
	File   string
	Source string
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

// ConfirmMsg asks the shell to show a confirmation dialog for a destructive action; the
// action command runs only if the user confirms.
type ConfirmMsg struct {
	Prompt string
	Action tea.Cmd
}

// CommitDetailLoadedMsg carries a commit's full message and the pipelines run for it
// (GitLab creates pipelines per ref, so there may be none).
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

// loadResult is implemented by every message that reports the outcome of a data load,
// whether it succeeded or not.
type loadResult interface{ loadErr() error }

func (m ProjectsLoadedMsg) loadErr() error       { return m.Err }
func (m BranchesLoadedMsg) loadErr() error       { return m.Err }
func (m PipelinesLoadedMsg) loadErr() error      { return m.Err }
func (m PipelineStagesLoadedMsg) loadErr() error { return nil }
func (m JobsLoadedMsg) loadErr() error           { return m.Err }
func (m JobTraceLoadedMsg) loadErr() error       { return m.Err }
func (m MRsLoadedMsg) loadErr() error            { return m.Err }
func (m IssuesLoadedMsg) loadErr() error         { return m.Err }
func (m TodosLoadedMsg) loadErr() error          { return m.Err }
func (m CommitsLoadedMsg) loadErr() error        { return m.Err }
func (m CommitDetailLoadedMsg) loadErr() error   { return m.Err }
func (m MRDetailLoadedMsg) loadErr() error       { return m.Err }
func (m IssueNotesLoadedMsg) loadErr() error     { return m.Err }
func (m ReadmeLoadedMsg) loadErr() error         { return m.Err }
func (m ErrorMsg) loadErr() error                { return m.Err }

// IsLoadResult reports whether a message is a data load reporting back, whether it
// succeeded or not.
func IsLoadResult(msg tea.Msg) bool {
	_, ok := msg.(loadResult)
	return ok
}

// LoadErr returns the error carried by any message that reports the outcome of a data
// load, or nil for messages that carry none.
func LoadErr(msg tea.Msg) error {
	if r, ok := msg.(loadResult); ok {
		return r.loadErr()
	}
	return nil
}
