package gitlab

import (
	"fmt"

	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

// Client wraps the GitLab API client with domain-specific operations.
type Client struct {
	api      *gogitlab.Client
	hostname string
}

// NewClient creates a new GitLab API client.
func NewClient(token, baseURL, hostname string) (*Client, error) {
	client, err := gogitlab.NewClient(token, gogitlab.WithBaseURL(baseURL))
	if err != nil {
		return nil, fmt.Errorf("creating gitlab client: %w", err)
	}
	return &Client{api: client, hostname: hostname}, nil
}

// Hostname returns the hostname this client is connected to.
func (c *Client) Hostname() string {
	return c.hostname
}
