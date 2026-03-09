# First-Run Setup Wizard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace glab CLI config dependency with lazyglab's own config file and interactive first-run setup wizard.

**Architecture:** New `internal/app/config.go` owns config loading/saving with `~/.config/lazyglab/config.yml`. New `internal/app/setup.go` handles the interactive CLI wizard. `app.Run()` checks for config, offers glab import or runs wizard, then launches TUI. `golang.org/x/term` for secure password input.

**Tech Stack:** Go, `gopkg.in/yaml.v3`, `golang.org/x/term`, `gitlab.com/gitlab-org/api/client-go`

---

### Task 1: Own Config Format — Types and Load/Save

**Files:**
- Rewrite: `internal/app/config.go`
- Create: `internal/app/config_test.go`

**Step 1: Write failing tests**

```go
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_valid(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	os.WriteFile(cfgPath, []byte(`
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-xxxxxxxxxxxxxxxxxxxx
`), 0600)

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
	os.WriteFile(cfgPath, []byte(`default_host: gitlab.com`), 0600)

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

	// Verify directory permissions
	dirInfo, _ := os.Stat(cfgDir)
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("want dir perm 0700, got %o", dirInfo.Mode().Perm())
	}

	// Verify file permissions
	fileInfo, _ := os.Stat(cfgPath)
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("want file perm 0600, got %o", fileInfo.Mode().Perm())
	}

	// Verify roundtrip
	loaded, err := LoadConfigFrom(cfgPath)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if loaded.Hosts["gitlab.com"].Token != "glpat-test" {
		t.Errorf("roundtrip token mismatch")
	}
}

func TestConfigPath(t *testing.T) {
	// With env var override
	t.Setenv("LAZYGLAB_CONFIG", "/tmp/test/config.yml")
	path := configPath()
	if path != "/tmp/test/config.yml" {
		t.Errorf("want env override, got %s", path)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -v -run TestLoadConfig`
Expected: FAIL — `LoadConfigFrom`, `SaveConfigTo`, `Config` not defined

**Step 3: Rewrite config.go with own config format**

Replace `internal/app/config.go` entirely:

```go
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
	defer f.Close()

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
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "feat: own config format with secure file permissions"
```

---

### Task 2: glab Config Import

**Files:**
- Create: `internal/app/glab.go`
- Create: `internal/app/glab_test.go`

**Step 1: Write failing tests**

```go
package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGlabConfig_valid(t *testing.T) {
	dir := t.TempDir()
	glabPath := filepath.Join(dir, "config.yml")
	os.WriteFile(glabPath, []byte(`
hosts:
  gitlab.com:
    token: glpat-fromglab
    api_protocol: https
  self-hosted.com:
    token: ""
    api_protocol: https
host: gitlab.com
`), 0600)

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
	os.WriteFile(glabPath, []byte(`
hosts:
  evil.com:
    token: glpat-evil
    api_protocol: http
host: evil.com
`), 0600)

	_, err := loadGlabConfigFrom(glabPath)
	if err == nil {
		t.Fatal("expected error for insecure protocol")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/app/ -v -run TestLoadGlabConfig`
Expected: FAIL — `loadGlabConfigFrom` not defined

**Step 3: Implement glab.go**

```go
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

// loadGlabConfigFrom reads glab's config and converts to our format.
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
```

**Step 4: Run tests**

Run: `go test ./internal/app/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/app/glab.go internal/app/glab_test.go
git commit -m "feat: glab config import support"
```

---

### Task 3: Token Validation via API

**Files:**
- Modify: `internal/gitlab/client.go`
- Create: `internal/gitlab/client_test.go`

**Step 1: Write failing test**

```go
package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateToken_success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"username":"testuser"}`))
	}))
	defer srv.Close()

	username, err := ValidateToken(srv.URL, "test-token", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "testuser" {
		t.Errorf("want testuser, got %s", username)
	}
}

func TestValidateToken_unauthorized(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"401 Unauthorized"}`))
	}))
	defer srv.Close()

	_, err := ValidateToken(srv.URL, "bad-token", srv.Client())
	if err == nil {
		t.Fatal("expected error for bad token")
	}
}

func TestValidateToken_serverError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ValidateToken(srv.URL, "token", srv.Client())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/gitlab/ -v -run TestValidateToken`
Expected: FAIL — `ValidateToken` not defined

**Step 3: Implement ValidateToken in client.go**

Add to `internal/gitlab/client.go`:

```go
// ValidateToken checks a token against the GitLab API and returns the username.
// Uses a custom http.Client if provided (for testing with TLS test servers),
// otherwise uses http.DefaultClient.
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
	defer resp.Body.Close()

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
```

Add imports: `"encoding/json"`, `"io"`, `"net/http"`.

**Step 4: Run tests**

Run: `go test ./internal/gitlab/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/gitlab/client.go internal/gitlab/client_test.go
git commit -m "feat: add token validation via /api/v4/user"
```

---

### Task 4: Interactive Setup Wizard

**Files:**
- Create: `internal/app/setup.go`

**Step 1: Implement setup.go**

No TDD for this one — it's interactive I/O that's hard to unit test. We test it manually.

```go
package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// RunSetup runs the interactive first-run setup wizard.
// Returns the resulting Config, or an error if the user cancels.
func RunSetup() (*Config, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  No config found. Let's set up lazyglab.")
	fmt.Println()

	// Host
	fmt.Print("  GitLab host [gitlab.com]: ")
	host, _ := reader.ReadString('\n')
	host = strings.TrimSpace(host)
	if host == "" {
		host = "gitlab.com"
	}

	// Token
	fmt.Println()
	fmt.Println("  Scopes needed: api (or read_api for read-only)")
	fmt.Printf("  Create one at: https://%s/-/user_settings/personal_access_tokens\n", host)
	fmt.Println()
	fmt.Print("  Personal access token: ")

	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // newline after hidden input
	if err != nil {
		return nil, fmt.Errorf("reading token: %w", err)
	}
	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return nil, fmt.Errorf("setup canceled: no token provided")
	}

	// Validate
	fmt.Print("  Testing connection... ")
	baseURL := fmt.Sprintf("https://%s", host)
	username, err := gitlab.ValidateToken(baseURL, token)
	if err != nil {
		fmt.Println("FAILED")
		return nil, fmt.Errorf("token validation failed: %w", err)
	}
	fmt.Printf("OK (logged in as @%s)\n", username)

	cfg := &Config{
		DefaultHost: host,
		Hosts: map[string]HostConfig{
			host: {Token: token},
		},
	}

	// Save
	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}
	fmt.Printf("  Config saved to %s\n", configPath())
	fmt.Println()

	return cfg, nil
}

// OfferGlabImport checks for glab config and offers to import it.
// Returns the imported Config, or nil if user declines or glab not found.
func OfferGlabImport() (*Config, error) {
	glabPath := findGlabConfig()
	if glabPath == "" {
		return nil, nil
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Printf("  Found glab config at %s\n", glabPath)
	fmt.Print("  Import hosts from glab? (y/n): ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		return nil, nil
	}

	cfg, err := loadGlabConfigFrom(glabPath)
	if err != nil {
		return nil, fmt.Errorf("importing glab config: %w", err)
	}

	if err := SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("saving imported config: %w", err)
	}

	fmt.Printf("  Imported %d host(s). Config saved to %s\n", len(cfg.Hosts), configPath())
	fmt.Println()

	return cfg, nil
}
```

**Step 2: Add golang.org/x/term dependency**

Run: `go get golang.org/x/term`

**Step 3: Build to verify**

Run: `go build ./...`
Expected: success

**Step 4: Commit**

```bash
git add internal/app/setup.go go.mod go.sum
git commit -m "feat: interactive first-run setup wizard with secure token input"
```

---

### Task 5: Wire Up app.Run() — Config Loading Priority

**Files:**
- Rewrite: `internal/app/app.go`
- Modify: `main.go` (add `setup` subcommand)

**Step 1: Rewrite app.go**

```go
package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui"
)

// Run initializes the application and starts the TUI.
func Run() error {
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}

	clients, hostNames, err := buildClients(cfg)
	if err != nil {
		return err
	}

	fmt.Println("  Launching lazyglab...")
	fmt.Println()

	model := tui.NewApp(clients, hostNames)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}

// Setup runs the setup wizard explicitly (via `lazyglab setup`).
func Setup() error {
	_, err := RunSetup()
	return err
}

// resolveConfig loads config with priority: own config > glab import > wizard.
func resolveConfig() (*Config, error) {
	// 1. Own config exists — use it
	if ConfigExists() {
		return LoadConfig()
	}

	// 2. glab config exists — offer import
	cfg, err := OfferGlabImport()
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return cfg, nil
	}

	// 3. Run setup wizard
	return RunSetup()
}

// buildClients creates GitLab API clients from config.
func buildClients(cfg *Config) (map[string]*gitlab.Client, []string, error) {
	clients := make(map[string]*gitlab.Client)
	var hostNames []string

	for host, hostCfg := range cfg.Hosts {
		if hostCfg.Token == "" {
			continue
		}
		apiHost := hostCfg.APIHost
		if apiHost == "" {
			apiHost = host
		}
		baseURL := fmt.Sprintf("https://%s/api/v4", apiHost)
		client, err := gitlab.NewClient(hostCfg.Token, baseURL, host)
		if err != nil {
			return nil, nil, fmt.Errorf("creating GitLab client for %s: %w", host, err)
		}
		clients[host] = client
		hostNames = append(hostNames, host)
	}

	if len(clients) == 0 {
		return nil, nil, fmt.Errorf("no GitLab hosts with tokens found in config")
	}

	// Put default host first
	if cfg.DefaultHost != "" {
		for i, h := range hostNames {
			if h == cfg.DefaultHost && i > 0 {
				hostNames[0], hostNames[i] = hostNames[i], hostNames[0]
				break
			}
		}
	}

	return clients, hostNames, nil
}
```

**Step 2: Update main.go to support `setup` subcommand**

```go
package main

import (
	"fmt"
	"os"

	"github.com/Malvi1697/lazyglab/internal/app"
)

var version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("lazyglab %s\n", version)
			os.Exit(0)
		case "setup":
			if err := app.Setup(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Build and verify**

Run: `go build ./...`
Expected: success

**Step 4: Manual smoke test**

Run: `LAZYGLAB_CONFIG=/tmp/nonexistent.yml go run .`
Expected: setup wizard starts, asks for host and token

**Step 5: Commit**

```bash
git add internal/app/app.go main.go
git commit -m "feat: wire config priority (own > glab import > wizard) and setup subcommand"
```

---

### Task 6: Config File Permission Verification

**Files:**
- Modify: `internal/app/config.go`
- Modify: `internal/app/config_test.go`

**Step 1: Write failing test**

Add to `config_test.go`:

```go
func TestLoadConfig_warnInsecurePermissions(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yml")
	os.WriteFile(cfgPath, []byte(`
default_host: gitlab.com
hosts:
  gitlab.com:
    token: glpat-xxxxxxxxxxxxxxxxxxxx
`), 0644) // too open!

	_, err := LoadConfigFrom(cfgPath)
	if err == nil {
		t.Fatal("expected error for insecure file permissions")
	}
	if !strings.Contains(err.Error(), "permissions") {
		t.Errorf("error should mention permissions, got: %v", err)
	}
}
```

Add `"strings"` to test imports.

**Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -v -run TestLoadConfig_warnInsecure`
Expected: FAIL — currently loads without checking permissions

**Step 3: Add permission check to LoadConfigFrom**

In `config.go`, add at the start of `LoadConfigFrom`, after reading the file info:

```go
func LoadConfigFrom(path string) (*Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading config at %s: %w", path, err)
	}

	// Check file permissions — reject if group/world readable
	if perm := info.Mode().Perm(); perm&0077 != 0 {
		return nil, fmt.Errorf(
			"config file %s has permissions %04o, which are too open; "+
				"run: chmod 600 %s", path, perm, path)
	}

	data, err := os.ReadFile(path)
	// ... rest unchanged
```

**Step 4: Run tests**

Run: `go test ./internal/app/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/app/config.go internal/app/config_test.go
git commit -m "fix(security): reject config files with insecure permissions"
```

---

### Task 7: Update CLAUDE.md and README

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

**Step 1: Update CLAUDE.md**

- Remove references to glab CLI dependency for auth
- Update config section to document `~/.config/lazyglab/config.yml`
- Document `lazyglab setup` subcommand
- Update config loading priority

**Step 2: Update README.md**

- Remove "requires glab" from prerequisites
- Add "First Run" section explaining the setup wizard
- Document manual config file format
- Document `lazyglab setup` for reconfiguration

**Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: update config documentation for own config format"
```
