package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// maxNotes bounds a discussion. A thread longer than this is being read on the
// web anyway, and the oldest hundred are rarely what you came for.
const maxNotes = 100

// ListMergeRequestNotes returns a merge request's discussion, oldest first —
// which is how a conversation reads.
func (c *Client) ListMergeRequestNotes(projectID, mrIID int) ([]Note, error) {
	opts := &gogitlab.ListMergeRequestNotesOptions{
		OrderBy:     gogitlab.Ptr("created_at"),
		Sort:        gogitlab.Ptr("asc"),
		ListOptions: gogitlab.ListOptions{PerPage: maxNotes},
	}

	apiNotes, _, err := c.api.Notes.ListMergeRequestNotes(projectID, int64(mrIID), opts)
	if err != nil {
		return nil, err
	}
	return mapNotes(apiNotes), nil
}

// CreateMergeRequestNote posts a comment on a merge request.
func (c *Client) CreateMergeRequestNote(projectID, mrIID int, body string) error {
	_, _, err := c.api.Notes.CreateMergeRequestNote(projectID, int64(mrIID),
		&gogitlab.CreateMergeRequestNoteOptions{Body: gogitlab.Ptr(body)})
	return err
}

// ListIssueNotes returns an issue's discussion, oldest first.
func (c *Client) ListIssueNotes(projectID, issueIID int) ([]Note, error) {
	opts := &gogitlab.ListIssueNotesOptions{
		OrderBy:     gogitlab.Ptr("created_at"),
		Sort:        gogitlab.Ptr("asc"),
		ListOptions: gogitlab.ListOptions{PerPage: maxNotes},
	}

	apiNotes, _, err := c.api.Notes.ListIssueNotes(projectID, int64(issueIID), opts)
	if err != nil {
		return nil, err
	}
	return mapNotes(apiNotes), nil
}

// CreateIssueNote posts a comment on an issue.
func (c *Client) CreateIssueNote(projectID, issueIID int, body string) error {
	_, _, err := c.api.Notes.CreateIssueNote(projectID, int64(issueIID),
		&gogitlab.CreateIssueNoteOptions{Body: gogitlab.Ptr(body)})
	return err
}

// mapNotes converts the API's notes into ours.
func mapNotes(apiNotes []*gogitlab.Note) []Note {
	notes := make([]Note, 0, len(apiNotes))
	for _, n := range apiNotes {
		if n == nil {
			continue
		}
		note := Note{
			ID:         int(n.ID),
			Author:     util.StripANSI(n.Author.Username),
			Body:       util.StripANSI(n.Body),
			System:     n.System,
			Internal:   n.Internal,
			Resolvable: n.Resolvable,
			Resolved:   n.Resolved,
		}
		if n.CreatedAt != nil {
			note.CreatedAt = *n.CreatedAt
		}
		// A comment on a line of code is worth saying so, since the line itself is
		// not in the thread.
		if n.Position != nil {
			note.OnPath = util.StripANSI(n.Position.NewPath)
			if note.OnPath == "" {
				note.OnPath = util.StripANSI(n.Position.OldPath)
			}
			note.OnLine = int(n.Position.NewLine)
		}
		notes = append(notes, note)
	}
	return notes
}
