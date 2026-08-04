package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gitlab.com", "gitlab.com"},
		{"  gitlab.com  ", "gitlab.com"},
		{"https://gitlab.com", "gitlab.com"},
		{"http://gitlab.example.com", "gitlab.example.com"},
		{"https://gitlab.com/", "gitlab.com"},
		{"https://gitlab.example.com/some/group", "gitlab.example.com"},
		{"gitlab.example.com:8443", "gitlab.example.com:8443"}, // an explicit port is kept
		{"", ""},
	}
	for _, tc := range cases {
		if got := NormalizeHost(tc.in); got != tc.want {
			t.Errorf("NormalizeHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestApplyHostToken_PreservesRestOfConfig(t *testing.T) {
	refresh := 60
	cfg := &Config{
		DefaultHost: "gitlab.example.com",
		Hosts: map[string]HostConfig{
			"gitlab.example.com": {Token: "old-token", APIHost: "api.gitlab.example.com"},
			"gitlab.com":         {Token: "other-token"},
		},
		Settings: Settings{
			Views:           []string{"overview", "pipelines"},
			DefaultView:     "pipelines",
			RefreshInterval: &refresh,
		},
	}

	applyHostToken(cfg, "gitlab.example.com", "new-token")

	if got := cfg.Hosts["gitlab.example.com"].Token; got != "new-token" {
		t.Errorf("token = %q, want %q", got, "new-token")
	}
	if got := cfg.Hosts["gitlab.example.com"].APIHost; got != "api.gitlab.example.com" {
		t.Errorf("api_host must be preserved, got %q", got)
	}
	if got := cfg.Hosts["gitlab.com"].Token; got != "other-token" {
		t.Errorf("other hosts must be preserved, got %q", got)
	}
	if cfg.Settings.DefaultView != "pipelines" || len(cfg.Settings.Views) != 2 {
		t.Error("settings must be preserved")
	}
	if cfg.Settings.RefreshInterval == nil || *cfg.Settings.RefreshInterval != 60 {
		t.Error("refresh_interval must be preserved")
	}
}

func TestApplyHostToken_AddsNewHostAndAdoptsDefault(t *testing.T) {
	cfg := &Config{}
	applyHostToken(cfg, "gitlab.example.com", "tok")

	if got := cfg.Hosts["gitlab.example.com"].Token; got != "tok" {
		t.Errorf("token = %q, want %q", got, "tok")
	}
	if cfg.DefaultHost != "gitlab.example.com" {
		t.Errorf("default_host = %q, want the first configured host", cfg.DefaultHost)
	}
}

func TestApplyHostToken_KeepsExistingDefaultHost(t *testing.T) {
	cfg := &Config{DefaultHost: "gitlab.com", Hosts: map[string]HostConfig{"gitlab.com": {Token: "a"}}}
	applyHostToken(cfg, "gitlab.example.com", "b")

	if cfg.DefaultHost != "gitlab.com" {
		t.Errorf("default_host = %q, want it left alone", cfg.DefaultHost)
	}
}

// TestApplyHostToken_RoundTrip checks the whole save/load cycle keeps the file readable
// by LoadConfig, which rejects group/world readable token files.
func TestApplyHostToken_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", path)

	cfg := &Config{
		Hosts:    map[string]HostConfig{"gitlab.example.com": {Token: "old"}},
		Settings: Settings{DefaultView: "commits"},
	}
	applyHostToken(cfg, "gitlab.example.com", "fresh")
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config perms = %04o, want 0600", perm)
	}

	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig after save: %v", err)
	}
	if got := reloaded.Hosts["gitlab.example.com"].Token; got != "fresh" {
		t.Errorf("reloaded token = %q, want %q", got, "fresh")
	}
	if reloaded.Settings.DefaultView != "commits" {
		t.Errorf("reloaded default_view = %q, want %q", reloaded.Settings.DefaultView, "commits")
	}
}

func TestLoadConfigForUpdate_MissingFileStartsFresh(t *testing.T) {
	t.Setenv("LAZYGLAB_CONFIG", filepath.Join(t.TempDir(), "absent.yml"))

	cfg, err := loadConfigForUpdate()
	if err != nil {
		t.Fatalf("loadConfigForUpdate: %v", err)
	}
	if cfg.Hosts == nil {
		t.Error("expected an initialized Hosts map")
	}
	if len(cfg.Hosts) != 0 {
		t.Errorf("expected an empty config, got %d hosts", len(cfg.Hosts))
	}
}

// TestLoadConfigForUpdate_UnreadableFileErrors guards the important case: a config that
// exists but cannot be parsed must not be silently replaced by an empty one.
func TestLoadConfigForUpdate_UnreadableFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("this: is: not: valid: yaml:\n"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("LAZYGLAB_CONFIG", path)

	if _, err := loadConfigForUpdate(); err == nil {
		t.Error("expected an error for an unparsable config, got nil")
	}
}
