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

	// The page's own detail: what state it is in, who is on it, what it is tagged.
	result.SHA = util.StripANSI(mr.SHA)
	result.MergeStatus = util.StripANSI(mr.DetailedMergeStatus)
	result.HasConflicts = mr.HasConflicts
	for _, l := range mr.Labels {
		result.Labels = append(result.Labels, util.StripANSI(l))
	}
	for _, r := range mr.Reviewers {
		if r != nil {
			result.Reviewers = append(result.Reviewers, util.StripANSI(r.Username))
		}
	}
	for _, a := range mr.Assignees {
		if a != nil {
			result.Assignees = append(result.Assignees, util.StripANSI(a.Username))
		}
	}
	return result, nil
}

// GetMergeRequestDiff returns a merge request's changes, one unified diff per file —
// the same shape as a commit's, so the same reader displays both.
func (c *Client) GetMergeRequestDiff(projectID, mrIID int) ([]FileDiff, error) {
	opts := &gogitlab.ListMergeRequestDiffsOptions{
		ListOptions: gogitlab.ListOptions{PerPage: maxDiffFiles},
	}

	apiDiffs, _, err := c.api.MergeRequests.ListMergeRequestDiffs(projectID, int64(mrIID), opts)
	if err != nil {
		return nil, err
	}

	diffs := make([]FileDiff, len(apiDiffs))
	for i, d := range apiDiffs {
		diffs[i] = FileDiff{
			OldPath: util.StripANSI(d.OldPath),
			NewPath: util.StripANSI(d.NewPath),
			Diff:    d.Diff,
			New:     d.NewFile,
			Deleted: d.DeletedFile,
			Renamed: d.RenamedFile,
			// GitLab omits the text for changes it will not send in full.
			Withheld: d.Diff == "" && !d.RenamedFile,
		}
		diffs[i].Added, diffs[i].Removed = countDiffLines(d.Diff)
	}
	return diffs, nil
}

// GetMergeRequestApprovals returns who has approved a merge request and what it still
// needs.
func (c *Client) GetMergeRequestApprovals(projectID, mrIID int) (*MRApprovals, error) {
	a, _, err := c.api.MergeRequestApprovals.GetConfiguration(projectID, int64(mrIID))
	if err != nil {
		return nil, err
	}

	out := &MRApprovals{
		Approved:    a.Approved,
		Required:    int(a.ApprovalsRequired),
		Left:        int(a.ApprovalsLeft),
		CanApprove:  a.UserCanApprove,
		HasApproved: a.UserHasApproved,
	}
	for _, u := range a.ApprovedBy {
		if u != nil && u.User != nil {
			out.ApprovedBy = append(out.ApprovedBy, util.StripANSI(u.User.Username))
		}
	}
	return out, nil
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
