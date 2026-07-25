package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// ListCommits returns recent commits for a project on the given ref
// (empty ref = default branch). CI status is left empty; callers map it from
// pipelines by SHA.
func (c *Client) ListCommits(projectID int, ref string) ([]Commit, error) {
	opts := &gogitlab.ListCommitsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: 50},
	}
	if ref != "" {
		opts.RefName = gogitlab.Ptr(ref)
	}

	apiCommits, _, err := c.api.Commits.ListCommits(projectID, opts)
	if err != nil {
		return nil, err
	}

	commits := make([]Commit, len(apiCommits))
	for i, cm := range apiCommits {
		commits[i] = Commit{
			ID:         util.StripANSI(cm.ID),
			ShortID:    util.StripANSI(cm.ShortID),
			Title:      util.StripANSI(cm.Title),
			AuthorName: util.StripANSI(cm.AuthorName),
			WebURL:     util.StripANSI(cm.WebURL),
		}
		if cm.CreatedAt != nil {
			commits[i].CreatedAt = *cm.CreatedAt
		}
	}
	return commits, nil
}
