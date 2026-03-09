package gitlab

import (
	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

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
