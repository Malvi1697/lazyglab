package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
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
		author := ""
		if issue.Author != nil {
			author = util.StripANSI(issue.Author.Username)
		}
		issues[i] = Issue{
			IID:         int(issue.IID),
			Title:       util.StripANSI(issue.Title),
			Author:      author,
			State:       util.StripANSI(issue.State),
			WebURL:      util.StripANSI(issue.WebURL),
			Description: util.StripANSI(issue.Description),
		}
		if issue.CreatedAt != nil {
			issues[i].CreatedAt = *issue.CreatedAt
		}
		if issue.UpdatedAt != nil {
			issues[i].UpdatedAt = *issue.UpdatedAt
		}
		for _, l := range issue.Labels {
			issues[i].Labels = append(issues[i].Labels, util.StripANSI(l))
		}
		for _, a := range issue.Assignees {
			issues[i].Assignees = append(issues[i].Assignees, util.StripANSI(a.Username))
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
