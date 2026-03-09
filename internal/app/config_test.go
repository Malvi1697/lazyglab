package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

func TestLoadConfig_valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-xxxxxxxxxxxxxxxxxxxx
`)

	cfg, err := LoadConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DefaultHost != "gitlab.com" {
		t.Errorf("want default_host=gitlab.com, got %s", cfg.DefaultHost)
	}
	if cfg.Hosts["gitlab.com"].Token != "glpat-xxxxxxxxxxxxxxxxxxxx" {
		t.Errorf("token mismatch")
	}
}

func TestLoadConfig_missingFile(t *testing.T) {
	_, err := LoadConfigFrom("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_noHosts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `default_host: gitlab.com`)

	_, err := LoadConfigFrom(cfgPath)
	if err == nil {
		t.Fatal("expected error for no hosts")
	}
}

func TestSaveConfig_permissions(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "lazyglab")
	cfgPath := filepath.Join(cfgDir, "config.yml")

	cfg := &Config{
		DefaultHost: "gitlab.com",
		Hosts: map[string]HostConfig{
			"gitlab.com": {Token: "glpat-test"},
		},
	}

	if err := SaveConfigTo(cfgPath, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dirInfo, err := os.Stat(cfgDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("want dir perm 0700, got %o", dirInfo.Mode().Perm())
	}

	fileInfo, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("want file perm 0600, got %o", fileInfo.Mode().Perm())
	}

	loaded, err := LoadConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if loaded.Hosts["gitlab.com"].Token != "glpat-test" {
		t.Errorf("roundtrip token mismatch")
	}
}

func TestConfigPath(t *testing.T) {
	t.Setenv("LAZYGLAB_CONFIG", "/tmp/test/config.yml")
	path := configPath()
	if path != "/tmp/test/config.yml" {
		t.Errorf("want env override, got %s", path)
	}
}

func TestConfigPath_default(t *testing.T) {
	t.Setenv("LAZYGLAB_CONFIG", "")
	path := configPath()
	if path == "" {
		t.Fatal("configPath returned empty string")
	}
	if filepath.Base(path) != "config.yml" {
		t.Errorf("want config.yml filename, got %s", filepath.Base(path))
	}
	if filepath.Base(filepath.Dir(path)) != "lazyglab" {
		t.Errorf("want lazyglab directory, got %s", filepath.Base(filepath.Dir(path)))
	}
}

func TestConfigExists_false(t *testing.T) {
	t.Setenv("LAZYGLAB_CONFIG", "/nonexistent/path/config.yml")
	if ConfigExists() {
		t.Error("expected ConfigExists to return false for nonexistent path")
	}
}

func TestConfigExists_true(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `
default_host: gitlab.com
hosts:
  gitlab.com:
    token: test
`)
	t.Setenv("LAZYGLAB_CONFIG", cfgPath)
	if !ConfigExists() {
		t.Error("expected ConfigExists to return true for existing file")
	}
}

func TestLoadConfig_defaultPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-default
`)
	t.Setenv("LAZYGLAB_CONFIG", cfgPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Hosts["gitlab.com"].Token != "glpat-default" {
		t.Errorf("token mismatch via LoadConfig()")
	}
}

func TestSaveConfig_defaultPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", cfgPath)

	cfg := &Config{
		DefaultHost: "gitlab.com",
		Hosts: map[string]HostConfig{
			"gitlab.com": {Token: "glpat-save-default"},
		},
	}

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("roundtrip via default path failed: %v", err)
	}
	if loaded.Hosts["gitlab.com"].Token != "glpat-save-default" {
		t.Errorf("roundtrip token mismatch via default path")
	}
}

func TestLoadConfig_multipleHosts(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-public
  self-hosted.example.com:
    token: glpat-private
    api_host: api.example.com
`)

	cfg, err := LoadConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Hosts) != 2 {
		t.Errorf("want 2 hosts, got %d", len(cfg.Hosts))
	}
	if cfg.Hosts["self-hosted.example.com"].APIHost != "api.example.com" {
		t.Errorf("APIHost mismatch")
	}
}

func TestSaveConfig_overwriteExisting(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")

	cfg1 := &Config{
		DefaultHost: "gitlab.com",
		Hosts: map[string]HostConfig{
			"gitlab.com": {Token: "original"},
		},
	}
	if err := SaveConfigTo(cfgPath, cfg1); err != nil {
		t.Fatalf("first save failed: %v", err)
	}

	cfg2 := &Config{
		DefaultHost: "gitlab.com",
		Hosts: map[string]HostConfig{
			"gitlab.com": {Token: "updated"},
		},
	}
	if err := SaveConfigTo(cfgPath, cfg2); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	loaded, err := LoadConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("load after overwrite failed: %v", err)
	}
	if loaded.Hosts["gitlab.com"].Token != "updated" {
		t.Errorf("want updated token, got %s", loaded.Hosts["gitlab.com"].Token)
	}
}

func TestLoadConfig_insecurePermissions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	// Write with 0644 — group/world readable
	if err := os.WriteFile(cfgPath, []byte(`
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-xxxxxxxxxxxxxxxxxxxx
`), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	_, err := LoadConfigFrom(cfgPath)
	if err == nil {
		t.Fatal("expected error for insecure file permissions")
	}
	if !strings.Contains(err.Error(), "permissions") {
		t.Errorf("error should mention permissions, got: %v", err)
	}
}

func TestLoadConfig_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	writeTestFile(t, cfgPath, `{{{invalid yaml`)

	_, err := LoadConfigFrom(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
