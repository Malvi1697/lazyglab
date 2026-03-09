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

	// Read token with echo disabled (secure input)
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
