package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// ListMergeRequests returns open merge requests for a project.
func (c *Client) ListMergeRequests(projectID int) ([]MergeRequest, error) {
	opts := &gogitlab.ListProjectMergeRequestsOptions{
		State:   gogitlab.Ptr("opened"),
		OrderBy: gogitlab.Ptr("updated_at"),
		Sort:    gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 50,
		},
	}

	apiMRs, _, err := c.api.MergeRequests.ListProjectMergeRequests(projectID, opts)
	if err != nil {
		return nil, err
	}

	mrs := make([]MergeRequest, len(apiMRs))
	for i, mr := range apiMRs {
		author := ""
		if mr.Author != nil {
			author = util.StripANSI(mr.Author.Username)
		}
		mrs[i] = MergeRequest{
			IID:          int(mr.IID),
			Title:        util.StripANSI(mr.Title),
			Author:       author,
			SourceBranch: util.StripANSI(mr.SourceBranch),
			TargetBranch: util.StripANSI(mr.TargetBranch),
			State:        util.StripANSI(mr.State),
			Draft:        mr.Draft,
			WebURL:       util.StripANSI(mr.WebURL),
			Description:  util.StripANSI(mr.Description),
		}
		if mr.CreatedAt != nil {
			mrs[i].CreatedAt = *mr.CreatedAt
		}
		if mr.UpdatedAt != nil {
			mrs[i].UpdatedAt = *mr.UpdatedAt
		}
		// BasicMergeRequest doesn't have Pipeline; skip it for list view
	}
	return mrs, nil
}

// GetMergeRequest returns a single merge request by IID.
func (c *Client) GetMergeRequest(projectID, mrIID int) (*MergeRequest, error) {
	mr, _, err := c.api.MergeRequests.GetMergeRequest(projectID, int64(mrIID), nil)
	if err != nil {
		return nil, err
	}

	author := ""
	if mr.Author != nil {
		author = util.StripANSI(mr.Author.Username)
	}
	result := &MergeRequest{
		IID:          int(mr.IID),
		Title:        util.StripANSI(mr.Title),
		Author:       author,
		SourceBranch: util.StripANSI(mr.SourceBranch),
		TargetBranch: util.StripANSI(mr.TargetBranch),
		State:        util.StripANSI(mr.State),
		Draft:        mr.Draft,
		WebURL:       util.StripANSI(mr.WebURL),
		Description:  util.StripANSI(mr.Description),
	}
	if mr.CreatedAt != nil {
		result.CreatedAt = *mr.CreatedAt
	}
	if mr.UpdatedAt != nil {
		result.UpdatedAt = *mr.UpdatedAt
	}
	if mr.Pipeline != nil {
		result.Pipeline = &PipelineInfo{
			ID:     int(mr.Pipeline.ID),
			Status: util.StripANSI(mr.Pipeline.Status),
			WebURL: util.StripANSI(mr.Pipeline.WebURL),
		}
	}
	return result, nil
}

// ApproveMergeRequest approves a merge request.
func (c *Client) ApproveMergeRequest(projectID, mrIID int) error {
	_, _, err := c.api.MergeRequestApprovals.ApproveMergeRequest(projectID, int64(mrIID), nil)
	return err
}

// MergeMergeRequest merges a merge request.
func (c *Client) MergeMergeRequest(projectID, mrIID int) error {
	_, _, err := c.api.MergeRequests.AcceptMergeRequest(projectID, int64(mrIID), nil)
	return err
}
