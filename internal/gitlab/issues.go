package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

// ListIssues returns open issues for a project.
func (c *Client) ListIssues(projectID int) ([]Issue, error) {
	opts := &gogitlab.ListProjectIssuesOptions{
		State:   gogitlab.Ptr("opened"),
		OrderBy: gogitlab.Ptr("updated_at"),
		Sort:    gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 50,
		},
	}

	apiIssues, _, err := c.api.Issues.ListProjectIssues(projectID, opts)
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, len(apiIssues))
	for i, issue := range apiIssues {
		issues[i] = Issue{
			IID:         int(issue.IID),
			Title:       issue.Title,
			Author:      issue.Author.Username,
			State:       issue.State,
			WebURL:      issue.WebURL,
			Description: issue.Description,
			CreatedAt:   *issue.CreatedAt,
			UpdatedAt:   *issue.UpdatedAt,
		}
		for _, l := range issue.Labels {
			issues[i].Labels = append(issues[i].Labels, l)
		}
		for _, a := range issue.Assignees {
			issues[i].Assignees = append(issues[i].Assignees, a.Username)
		}
	}
	return issues, nil
}

// CloseIssue closes an issue.
func (c *Client) CloseIssue(projectID, issueIID int) error {
	_, _, err := c.api.Issues.UpdateIssue(projectID, int64(issueIID), &gogitlab.UpdateIssueOptions{
		StateEvent: gogitlab.Ptr("close"),
	})
	return err
}

// ReopenIssue reopens a closed issue.
func (c *Client) ReopenIssue(projectID, issueIID int) error {
	_, _, err := c.api.Issues.UpdateIssue(projectID, int64(issueIID), &gogitlab.UpdateIssueOptions{
		StateEvent: gogitlab.Ptr("reopen"),
	})
	return err
}
