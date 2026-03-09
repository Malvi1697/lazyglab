package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

// ValidateToken checks a token against the GitLab API and returns the username.
// Accepts optional http.Client for testing with TLS test servers.
// Error messages never include the token value.
func ValidateToken(baseURL, token string, httpClient ...*http.Client) (string, error) {
	client := http.DefaultClient
	if len(httpClient) > 0 && httpClient[0] != nil {
		client = httpClient[0]
	}

	req, err := http.NewRequest("GET", baseURL+"/api/v4/user", nil)
	if err != nil {
		return "", fmt.Errorf("connection failed: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("connection failed: unable to reach %s", baseURL)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("authentication failed: invalid or expired token")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server error: %s returned HTTP %d", baseURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response failed")
	}

	var user struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(body, &user); err != nil || user.Username == "" {
		return "", fmt.Errorf("unexpected response from %s", baseURL)
	}

	return user.Username, nil
}
