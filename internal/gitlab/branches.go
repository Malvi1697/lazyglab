package gitlab

import (
	"sort"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// ListBranches returns branches for a project, sorted by most recent activity.
func (c *Client) ListBranches(projectID int) ([]Branch, error) {
	opts := &gogitlab.ListBranchesOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
		},
	}

	apiBranches, _, err := c.api.Branches.ListBranches(projectID, opts)
	if err != nil {
		return nil, err
	}

	branches := make([]Branch, len(apiBranches))
	for i, b := range apiBranches {
		branches[i] = Branch{
			Name:      util.StripANSI(b.Name),
			Protected: b.Protected,
			Merged:    b.Merged,
			Default:   b.Default,
			WebURL:    util.StripANSI(b.WebURL),
		}
		if b.Commit != nil && b.Commit.CommittedDate != nil {
			branches[i].LastActivity = *b.Commit.CommittedDate
		}
	}

	// Sort by last activity (most recent first), default branch always on top
	sort.Slice(branches, func(i, j int) bool {
		if branches[i].Default != branches[j].Default {
			return branches[i].Default
		}
		return branches[i].LastActivity.After(branches[j].LastActivity)
	})

	return branches, nil
}

// ListPipelinesByRef returns pipelines filtered by branch ref.
func (c *Client) ListPipelinesByRef(projectID int, ref string) ([]Pipeline, error) {
	opts := &gogitlab.ListProjectPipelinesOptions{
		Ref:     gogitlab.Ptr(ref),
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
	return pipelines, nil
}
