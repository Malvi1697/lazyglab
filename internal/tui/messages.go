package tui

import "github.com/janvseticek/lazyglab/internal/gitlab"

// PanelID identifies which side panel is active.
type PanelID int

const (
	PanelProjects PanelID = iota
	PanelMergeRequests
	PanelPipelines
	PanelIssues
)

// PanelName returns a human-readable name for the panel.
func (p PanelID) PanelName() string {
	switch p {
	case PanelProjects:
		return "Projects"
	case PanelMergeRequests:
		return "Merge Requests"
	case PanelPipelines:
		return "Pipelines"
	case PanelIssues:
		return "Issues"
	default:
		return "Unknown"
	}
}

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

// IssuesLoadedMsg is sent when issues have been fetched.
type IssuesLoadedMsg struct {
	Issues []gitlab.Issue
	Err    error
}

// StatusMsg is sent to display a status message in the status bar.
type StatusMsg struct {
	Text  string
	IsErr bool
}

// ErrorMsg represents an error from an async operation.
type ErrorMsg struct {
	Err error
}
