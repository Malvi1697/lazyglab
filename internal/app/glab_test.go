package app

import (
	"path/filepath"
	"testing"
)

func TestLoadGlabConfig_valid(t *testing.T) {
	dir := t.TempDir()
	glabPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, glabPath, `
hosts:
  gitlab.com:
    token: glpat-fromglab
    api_protocol: https
  self-hosted.com:
    token: ""
    api_protocol: https
host: gitlab.com
`)

	cfg, err := loadGlabConfigFrom(glabPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultHost != "gitlab.com" {
		t.Errorf("want default_host=gitlab.com, got %s", cfg.DefaultHost)
	}
	// Should only have hosts with tokens
	if len(cfg.Hosts) != 1 {
		t.Errorf("want 1 host (skipping empty token), got %d", len(cfg.Hosts))
	}
	if cfg.Hosts["gitlab.com"].Token != "glpat-fromglab" {
		t.Errorf("token mismatch")
	}
}

func TestLoadGlabConfig_missingFile(t *testing.T) {
	_, err := loadGlabConfigFrom("/nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadGlabConfig_insecureProtocol(t *testing.T) {
	dir := t.TempDir()
	glabPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, glabPath, `
hosts:
  evil.com:
    token: glpat-evil
    api_protocol: http
host: evil.com
`)

	_, err := loadGlabConfigFrom(glabPath)
	if err == nil {
		t.Fatal("expected error for insecure protocol")
	}
}
