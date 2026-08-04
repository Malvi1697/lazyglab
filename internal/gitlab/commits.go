package gitlab

import (
	"strings"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// GetCommit returns one commit by SHA, including its full message, which the list
// endpoint does not carry.
func (c *Client) GetCommit(projectID int, sha string) (*Commit, error) {
	cm, _, err := c.api.Commits.GetCommit(projectID, sha, nil)
	if err != nil {
		return nil, err
	}
	commit := &Commit{
		ID:         util.StripANSI(cm.ID),
		ShortID:    util.StripANSI(cm.ShortID),
		Title:      util.StripANSI(cm.Title),
		Message:    util.StripANSI(cm.Message),
		AuthorName: util.StripANSI(cm.AuthorName),
		WebURL:     util.StripANSI(cm.WebURL),
		ParentIDs:  cm.ParentIDs,
	}
	if cm.CreatedAt != nil {
		commit.CreatedAt = *cm.CreatedAt
	}
	return commit, nil
}

// ListCommits returns recent commits for a project on the given ref (empty ref =
// default branch).
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
		// The pipeline list will want these same titles by SHA, and this is the cheapest
		// place they will ever be: already fetched, already parsed.
		if commits[i].ID != "" && commits[i].Title != "" {
			c.titleCache.Store(commits[i].ID, commits[i].Title)
		}
	}
	return commits, nil
}

// GetCommitRefs returns the branches and tags that contain a commit, as shown on
// GitLab's commit page.
func (c *Client) GetCommitRefs(projectID int, sha string) ([]CommitRef, error) {
	opts := &gogitlab.GetCommitRefsOptions{
		Type:        gogitlab.Ptr("all"),
		ListOptions: gogitlab.ListOptions{PerPage: 20},
	}
	apiRefs, _, err := c.api.Commits.GetCommitRefs(projectID, sha, opts)
	if err != nil {
		return nil, err
	}

	refs := make([]CommitRef, len(apiRefs))
	for i, r := range apiRefs {
		refs[i] = CommitRef{Type: util.StripANSI(r.Type), Name: util.StripANSI(r.Name)}
	}
	return refs, nil
}

// ListCommitMergeRequests returns the merge requests associated with a commit.
func (c *Client) ListCommitMergeRequests(projectID int, sha string) ([]MergeRequest, error) {
	apiMRs, _, err := c.api.Commits.ListMergeRequestsByCommit(projectID, sha)
	if err != nil {
		return nil, err
	}

	mrs := make([]MergeRequest, len(apiMRs))
	for i, mr := range apiMRs {
		mrs[i] = MergeRequest{
			IID:          int(mr.IID),
			Title:        util.StripANSI(mr.Title),
			SourceBranch: util.StripANSI(mr.SourceBranch),
			TargetBranch: util.StripANSI(mr.TargetBranch),
			State:        util.StripANSI(mr.State),
			WebURL:       util.StripANSI(mr.WebURL),
		}
		if mr.Author != nil {
			mrs[i].Author = util.StripANSI(mr.Author.Username)
		}
	}
	return mrs, nil
}

// maxDiffFiles bounds how many files of a commit we ask for.
const maxDiffFiles = 100

// GetCommitDiff returns a commit's changes, one unified diff per file.
func (c *Client) GetCommitDiff(projectID int, sha string) ([]FileDiff, error) {
	opts := &gogitlab.GetCommitDiffOptions{
		ListOptions: gogitlab.ListOptions{PerPage: maxDiffFiles},
	}

	apiDiffs, _, err := c.api.Commits.GetCommitDiff(projectID, sha, opts)
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

// countDiffLines counts added and removed lines in a unified diff, ignoring the +++/---
// file headers.
func countDiffLines(diff string) (added, removed int) {
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			added++
		case strings.HasPrefix(line, "-"):
			removed++
		}
	}
	return added, removed
}
