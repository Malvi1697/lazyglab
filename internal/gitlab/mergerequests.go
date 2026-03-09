package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"
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
		mrs[i] = MergeRequest{
			IID:          int(mr.IID),
			Title:        mr.Title,
			Author:       mr.Author.Username,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			State:        mr.State,
			Draft:        mr.Draft,
			WebURL:       mr.WebURL,
			Description:  mr.Description,
			CreatedAt:    *mr.CreatedAt,
			UpdatedAt:    *mr.UpdatedAt,
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

	result := &MergeRequest{
		IID:          int(mr.IID),
		Title:        mr.Title,
		Author:       mr.Author.Username,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		State:        mr.State,
		Draft:        mr.Draft,
		WebURL:       mr.WebURL,
		Description:  mr.Description,
		CreatedAt:    *mr.CreatedAt,
		UpdatedAt:    *mr.UpdatedAt,
	}
	if mr.Pipeline != nil {
		result.Pipeline = &PipelineInfo{
			ID:     int(mr.Pipeline.ID),
			Status: mr.Pipeline.Status,
			WebURL: mr.Pipeline.WebURL,
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
