package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// glabRawConfig represents the glab CLI configuration file structure.
type glabRawConfig struct {
	Hosts       map[string]glabHostConfig `yaml:"hosts"`
	DefaultHost string                    `yaml:"host"`
}

type glabHostConfig struct {
	Token       string `yaml:"token"`
	APIHost     string `yaml:"api_host"`
	APIProtocol string `yaml:"api_protocol"`
}

// loadGlabConfigFrom reads glab's config and converts to our Config format.
// Skips hosts without tokens. Rejects insecure protocols.
func loadGlabConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading glab config: %w", err)
	}

	var raw glabRawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing glab config: %w", err)
	}

	cfg := &Config{
		DefaultHost: raw.DefaultHost,
		Hosts:       make(map[string]HostConfig),
	}

	for host, hc := range raw.Hosts {
		if hc.Token == "" {
			continue
		}
		protocol := hc.APIProtocol
		if protocol == "" {
			protocol = "https"
		}
		if protocol != "https" {
			return nil, fmt.Errorf("glab host %s uses insecure protocol %q", host, protocol)
		}
		entry := HostConfig{Token: hc.Token}
		if hc.APIHost != "" && hc.APIHost != host {
			entry.APIHost = hc.APIHost
		}
		cfg.Hosts[host] = entry
	}

	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no hosts with tokens found in glab config")
	}

	return cfg, nil
}

// findGlabConfig returns the path to glab's config file, or "" if not found.
func findGlabConfig() string {
	if dir := os.Getenv("GLAB_CONFIG_DIR"); dir != "" {
		p := filepath.Join(dir, "config.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	if runtime.GOOS == "darwin" {
		if home, err := os.UserHomeDir(); err == nil {
			p := filepath.Join(home, "Library", "Application Support", "glab-cli", "config.yml")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	if configDir, err := os.UserConfigDir(); err == nil {
		p := filepath.Join(configDir, "glab-cli", "config.yml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}
