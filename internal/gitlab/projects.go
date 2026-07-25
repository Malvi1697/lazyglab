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

// ListProjects returns projects the authenticated user is a member of.
func (c *Client) ListProjects() ([]Project, error) {
	membership := true
	opts := &gogitlab.ListProjectsOptions{
		Membership: &membership,
		OrderBy:    gogitlab.Ptr("last_activity_at"),
		Sort:       gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 50,
		},
	}

	apiProjects, _, err := c.api.Projects.ListProjects(opts)
	if err != nil {
		return nil, err
	}

	projects := make([]Project, len(apiProjects))
	for i, p := range apiProjects {
		projects[i] = Project{
			ID:                int(p.ID),
			Name:              util.StripANSI(p.Name),
			NameWithNamespace: util.StripANSI(p.NameWithNamespace),
			PathWithNamespace: util.StripANSI(p.PathWithNamespace),
			WebURL:            util.StripANSI(p.WebURL),
			DefaultBranch:     util.StripANSI(p.DefaultBranch),
		}
	}
	return projects, nil
}
