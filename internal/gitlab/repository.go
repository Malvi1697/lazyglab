package gitlab

import (
	"strings"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// maxReadmeBytes caps what we will render. A README past this is documentation
// being read on the web, and holding a megabyte of it to show forty rows is not
// what a dashboard is for.
const maxReadmeBytes = 128 * 1024

// GetReadme returns a project's README as text, from the given ref (empty = the
// project default). The file's path comes from Project.ReadmeFile, which GitLab
// hands over with the project itself.
func (c *Client) GetReadme(projectID int, file, ref string) (string, error) {
	if file == "" {
		return "", nil
	}
	opts := &gogitlab.GetRawFileOptions{}
	if ref != "" {
		opts.Ref = gogitlab.Ptr(ref)
	}

	raw, _, err := c.api.RepositoryFiles.GetRawFile(projectID, file, opts)
	if err != nil {
		return "", err
	}
	if len(raw) > maxReadmeBytes {
		raw = raw[:maxReadmeBytes]
	}
	// A README is untrusted text like any other: it must not be able to paint the
	// screen with escape sequences of its own.
	return strings.ReplaceAll(util.StripANSI(string(raw)), "\r\n", "\n"), nil
}
