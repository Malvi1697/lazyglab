package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// GlabConfig represents the glab CLI configuration file structure.
type GlabConfig struct {
	Hosts       map[string]HostConfig `yaml:"hosts"`
	DefaultHost string                `yaml:"host"`
}

// HostConfig holds per-host GitLab configuration from glab.
type HostConfig struct {
	Token       string `yaml:"token"`
	APIHost     string `yaml:"api_host"`
	APIProtocol string `yaml:"api_protocol"`
	User        string `yaml:"user"`
}

// LoadGlabConfig reads authentication config from the glab CLI config file.
func LoadGlabConfig() (*GlabConfig, error) {
	path, err := glabConfigPath()
	if err != nil {
		return nil, fmt.Errorf("finding glab config: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading glab config at %s: %w", path, err)
	}

	var cfg GlabConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing glab config: %w", err)
	}

	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in glab config at %s", path)
	}

	return &cfg, nil
}

func glabConfigPath() (string, error) {
	// Check GLAB_CONFIG_DIR env var first
	if dir := os.Getenv("GLAB_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.yml"), nil
	}

	// macOS: ~/Library/Application Support/glab-cli/config.yml
	if runtime.GOOS == "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path := filepath.Join(home, "Library", "Application Support", "glab-cli", "config.yml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Linux/fallback: ~/.config/glab-cli/config.yml
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(configDir, "glab-cli", "config.yml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("glab config not found; please configure glab first: https://gitlab.com/gitlab-org/cli")
}
