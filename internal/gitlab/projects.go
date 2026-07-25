package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// GetProjectByPath returns a single project by its "group/project" path.
// Favorites are stored as paths and ListProjects only returns the 50 most
// recently active ones, so a favorite is fetched directly rather than looked up
// in that list.
func (c *Client) GetProjectByPath(path string) (*Project, error) {
	p, _, err := c.api.Projects.GetProject(path, nil)
	if err != nil {
		return nil, err
	}
	return &Project{
		ID:                int(p.ID),
		Name:              util.StripANSI(p.Name),
		NameWithNamespace: util.StripANSI(p.NameWithNamespace),
		PathWithNamespace: util.StripANSI(p.PathWithNamespace),
		WebURL:            util.StripANSI(p.WebURL),
		DefaultBranch:     util.StripANSI(p.DefaultBranch),
	}, nil
}

// maxProjectPages bounds how many pages of projects we fetch, so a member of an
// unusually large number of projects cannot stall startup (100 per page => 2000).
const maxProjectPages = 20

// ListProjects returns every project the authenticated user is a member of,
// most recently active first. All pages are fetched: a single page would hide
// the rest of the user's projects from the switcher entirely.
func (c *Client) ListProjects() ([]Project, error) {
	membership := true
	opts := &gogitlab.ListProjectsOptions{
		Membership: &membership,
		OrderBy:    gogitlab.Ptr("last_activity_at"),
		Sort:       gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var projects []Project
	for page := 0; page < maxProjectPages; page++ {
		apiProjects, resp, err := c.api.Projects.ListProjects(opts)
		if err != nil {
			return nil, err
		}

		for _, p := range apiProjects {
			projects = append(projects, Project{
				ID:                int(p.ID),
				Name:              util.StripANSI(p.Name),
				NameWithNamespace: util.StripANSI(p.NameWithNamespace),
				PathWithNamespace: util.StripANSI(p.PathWithNamespace),
				WebURL:            util.StripANSI(p.WebURL),
				DefaultBranch:     util.StripANSI(p.DefaultBranch),
			})
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return projects, nil
}
