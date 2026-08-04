package gitlab

import (
	"strings"
	"sync"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// GetProjectByPath returns a single project by its "group/project" path, used to
// open a favorite that is not among the loaded projects (for instance one capped
// out by maxProjectPages).
func (c *Client) GetProjectByPath(path string) (*Project, error) {
	p, _, err := c.api.Projects.GetProject(path, nil)
	if err != nil {
		return nil, err
	}
	project := toProject(p)
	return &project, nil
}

// toProject maps an API project into a domain one, for both the list and the
// by-path lookup.
func toProject(p *gogitlab.Project) Project {
	return Project{
		ID:                int(p.ID),
		Name:              util.StripANSI(p.Name),
		NameWithNamespace: util.StripANSI(p.NameWithNamespace),
		PathWithNamespace: util.StripANSI(p.PathWithNamespace),
		WebURL:            util.StripANSI(p.WebURL),
		DefaultBranch:     util.StripANSI(p.DefaultBranch),
		SSHCloneURL:       util.StripANSI(p.SSHURLToRepo),
		HTTPCloneURL:      util.StripANSI(p.HTTPURLToRepo),
		ReadmeFile:        readmeFile(p.ReadmeURL),
	}
}

// readmeFile is the repository path of a project's README, taken from the URL
// GitLab sends: ".../-/blob/main/docs/README.md" -> "docs/README.md".
//
// Cheaper than looking for it: the project payload already carries this, and a
// project without one carries an empty string, which is the answer too.
func readmeFile(readmeURL string) string {
	if readmeURL == "" {
		return ""
	}
	i := strings.Index(readmeURL, "/-/blob/")
	if i < 0 {
		// Some deployments write the older form without the "/-/" separator.
		i = strings.Index(readmeURL, "/blob/")
		if i < 0 {
			return ""
		}
		rest := readmeURL[i+len("/blob/"):]
		return util.StripANSI(afterFirstSlash(rest))
	}
	rest := readmeURL[i+len("/-/blob/"):]
	return util.StripANSI(afterFirstSlash(rest))
}

// afterFirstSlash drops the ref that follows /blob/, leaving the file path.
func afterFirstSlash(s string) string {
	if i := strings.Index(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return ""
}

const (
	// maxProjectPages bounds how many pages of projects we fetch, so a member of
	// an unusually large number of projects cannot stall startup (100 => 2000).
	maxProjectPages = 20
	// projectPageWorkers bounds concurrent page requests. Someone in 800 projects
	// needs 8 pages; fetching them in parallel turns 8 round trips into ~2.
	projectPageWorkers = 4
)

// ListProjects returns every project the authenticated user is a member of,
// most recently active first.
//
// All pages are fetched — a single page would hide the rest of the user's
// projects from the switcher entirely — but cheaply: the simple representation
// carries every field we map while being far smaller (measured against a real
// instance: 93 KiB in 1.5s per 100 projects, versus 424 KiB in 3.7s), and pages
// after the first are requested concurrently.
func (c *Client) ListProjects() ([]Project, error) {
	first, resp, err := c.projectPage(1)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return first, nil
	}

	// Prefer the advertised page count so the rest can be fetched in parallel.
	// GitLab withholds it for very large collections; then walk page by page.
	switch {
	case resp.TotalPages > 1:
		return c.projectPagesConcurrent(first, min(int(resp.TotalPages), maxProjectPages))
	case resp.TotalPages == 0 && resp.NextPage != 0:
		return c.projectPagesSequential(first, int(resp.NextPage))
	default:
		return first, nil
	}
}

// projectPage fetches one page of projects.
func (c *Client) projectPage(page int) ([]Project, *gogitlab.Response, error) {
	membership := true
	simple := true
	opts := &gogitlab.ListProjectsOptions{
		Membership: &membership,
		// The simple representation omits permissions, statistics and namespace
		// details we never render, which is most of the payload.
		Simple:  &simple,
		OrderBy: gogitlab.Ptr("last_activity_at"),
		Sort:    gogitlab.Ptr("desc"),
		ListOptions: gogitlab.ListOptions{
			PerPage: 100,
			Page:    int64(page),
		},
	}

	apiProjects, resp, err := c.api.Projects.ListProjects(opts)
	if err != nil {
		return nil, resp, err
	}

	projects := make([]Project, len(apiProjects))
	for i, p := range apiProjects {
		projects[i] = toProject(p)
	}
	return projects, resp, nil
}

// projectPagesConcurrent fetches pages 2..totalPages in parallel and appends
// them to first in page order, preserving the server's activity ordering.
func (c *Client) projectPagesConcurrent(first []Project, totalPages int) ([]Project, error) {
	pages := make([][]Project, totalPages+1)
	sem := make(chan struct{}, projectPageWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for page := 2; page <= totalPages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			items, _, err := c.projectPage(page)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				return
			}
			pages[page] = items
		}(page)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	projects := first
	for page := 2; page <= totalPages; page++ {
		projects = append(projects, pages[page]...)
	}
	return projects, nil
}

// projectPagesSequential walks NextPage links, for servers that do not report a
// total page count.
func (c *Client) projectPagesSequential(first []Project, nextPage int) ([]Project, error) {
	projects := first
	for fetched := 1; fetched < maxProjectPages && nextPage != 0; fetched++ {
		items, resp, err := c.projectPage(nextPage)
		if err != nil {
			return nil, err
		}
		projects = append(projects, items...)
		if resp == nil {
			break
		}
		nextPage = int(resp.NextPage)
	}
	return projects, nil
}
