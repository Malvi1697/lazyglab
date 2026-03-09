package app

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is lazyglab's own configuration.
type Config struct {
	DefaultHost string                `yaml:"default_host"`
	Hosts       map[string]HostConfig `yaml:"hosts"`
}

// HostConfig holds per-host GitLab configuration.
type HostConfig struct {
	Token   string `yaml:"token"`
	APIHost string `yaml:"api_host,omitempty"` // optional: if API is on different host

}

// LoadConfig loads config from the default path.
func LoadConfig() (*Config, error) {
	return LoadConfigFrom(configPath())
}

// LoadConfigFrom loads config from a specific path.
func LoadConfigFrom(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading config at %s: %w", path, err)
	}

	// Reject group/world readable config files (contain tokens)
	if perm := info.Mode().Perm(); perm&0077 != 0 {
		return nil, fmt.Errorf(
			"config file %s has permissions %04o, which are too open; "+
				"run: chmod 600 %s", path, perm, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config at %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no hosts found in config at %s", path)
	}

	return &cfg, nil
}

// SaveConfig saves config to the default path with secure permissions.
func SaveConfig(cfg *Config) error {
	return SaveConfigTo(configPath(), cfg)
}

// SaveConfigTo saves config to a specific path with secure permissions.
func SaveConfigTo(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Write with 0600 from the start — no permission race window
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// ConfigExists returns true if lazyglab's own config file exists.
func ConfigExists() bool {
	_, err := os.Stat(configPath())
	return err == nil
}

// configPath returns the path to lazyglab's config file.
func configPath() string {
	if p := os.Getenv("LAZYGLAB_CONFIG"); p != "" {
		return p
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "lazyglab", "config.yml")
	}
	return filepath.Join(configDir, "lazyglab", "config.yml")
}
