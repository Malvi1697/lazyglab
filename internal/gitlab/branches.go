package gitlab

import (
	"sort"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// maxBranchPages bounds how many pages of branches we fetch.
const maxBranchPages = 20

// ListBranches returns branches for a project, sorted by most recent activity.
func (c *Client) ListBranches(projectID int) ([]Branch, error) {
	var branches []Branch

	opts := &gogitlab.ListBranchesOptions{
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	for page := 0; page < maxBranchPages; page++ {
		apiBranches, resp, err := c.api.Branches.ListBranches(projectID, opts)
		if err != nil {
			return nil, err
		}

		for _, b := range apiBranches {
			branch := Branch{
				Name:      util.StripANSI(b.Name),
				Protected: b.Protected,
				Merged:    b.Merged,
				Default:   b.Default,
				WebURL:    util.StripANSI(b.WebURL),
			}
			if b.Commit != nil && b.Commit.CommittedDate != nil {
				branch.LastActivity = *b.Commit.CommittedDate
			}
			branches = append(branches, branch)
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Sort by last activity (most recent first), default branch always on top.
	sort.SliceStable(branches, func(i, j int) bool {
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

	pipelines := toPipelines(apiPipelines)

	c.fillCommitTitles(projectID, pipelines)
	c.fillWarnings(projectID, pipelines)
	return pipelines, nil
}
