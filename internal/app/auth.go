package app

import (
	"fmt"
	"strings"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// NormalizeHost cleans up a user-typed GitLab host: a pasted URL keeps working
// because the scheme and any trailing slash or path are stripped, so we never
// build a broken "https://https://gitlab.com" base URL.
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	return host
}

// ReconfigureAuth validates a host/token pair, persists the token to the config
// and returns a ready client plus the authenticated username. It is what the
// TUI's re-authentication overlay calls; keeping it here means all config and
// base-URL handling stays in one place (and the TUI needs no config import).
func ReconfigureAuth(host, token string) (*gitlab.Client, string, error) {
	host = NormalizeHost(host)
	token = strings.TrimSpace(token)
	if host == "" {
		return nil, "", fmt.Errorf("host is required")
	}
	if token == "" {
		return nil, "", fmt.Errorf("token is required")
	}

	cfg, err := loadConfigForUpdate()
	if err != nil {
		return nil, "", err
	}

	// An existing entry may point the API at a different host; keep that.
	apiHost := cfg.Hosts[host].APIHost
	if apiHost == "" {
		apiHost = host
	}

	username, err := gitlab.ValidateToken("https://"+apiHost, token)
	if err != nil {
		return nil, "", err
	}

	applyHostToken(cfg, host, token)
	if err := SaveConfig(cfg); err != nil {
		return nil, "", err
	}

	client, err := gitlab.NewClient(token, "https://"+apiHost+"/api/v4", host)
	if err != nil {
		return nil, "", err
	}
	return client, username, nil
}

// applyHostToken records token for host, leaving every other host entry, that
// host's api_host and all settings untouched — re-authenticating must never cost
// the user the rest of their config. It also adopts the host as the default when
// none is set yet.
func applyHostToken(cfg *Config, host, token string) {
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]HostConfig)
	}
	entry := cfg.Hosts[host]
	entry.Token = token
	cfg.Hosts[host] = entry
	if cfg.DefaultHost == "" {
		cfg.DefaultHost = host
	}
}

// loadConfigForUpdate returns the config to modify in place: the existing one
// when there is a readable file, otherwise a fresh one. A file that exists but
// cannot be read is an error rather than an empty config, so a bad permission
// or parse error never silently discards other hosts and settings.
func loadConfigForUpdate() (*Config, error) {
	cfg := &Config{}
	if ConfigExists() {
		loaded, err := LoadConfig()
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}
	if cfg.Hosts == nil {
		cfg.Hosts = make(map[string]HostConfig)
	}
	return cfg, nil
}
