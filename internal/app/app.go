package app

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// Run initializes the application and starts the TUI. version is the running
// build, which the shell needs to tell the user when a newer one exists.
func Run(version string) error {
	cfg, err := resolveConfig()
	if err != nil {
		return err
	}

	clients, hostNames, err := buildClients(cfg)
	if err != nil {
		return err
	}

	// Auto-detect project from git remote
	var detectedHost, detectedPath string
	remotes := util.DetectGitRemotes()
	for _, r := range remotes {
		if _, ok := clients[r.Host]; ok {
			detectedHost = r.Host
			detectedPath = r.Path
			break
		}
	}

	// Resolve view configuration and refresh interval from settings.
	viewIDs, warnings := views.ParseViews(cfg.Settings.Views)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "  config: %s\n", w)
	}
	if len(cfg.Settings.Panels) > 0 {
		fmt.Fprintln(os.Stderr, "  config: 'panels' is obsolete; use 'views'")
	}
	defaultIndex := views.DefaultViewIndex(viewIDs, cfg.Settings.DefaultView)
	refreshInterval := time.Duration(cfg.Settings.RefreshSeconds()) * time.Second

	fmt.Println("  Launching lazyglab...")
	fmt.Println()

	activeHost := detectedHost
	if activeHost == "" && len(hostNames) > 0 {
		activeHost = hostNames[0]
	}

	model := tui.NewApp(tui.Options{
		Clients:          clients,
		HostNames:        hostNames,
		DetectedHost:     detectedHost,
		DetectedPath:     detectedPath,
		ViewIDs:          viewIDs,
		DefaultViewIndex: defaultIndex,
		RefreshInterval:  refreshInterval,
		Favorites:        FavoritesFor(cfg, activeHost),
		LastProject:      LastProjectFor(cfg, activeHost),
		Reconfigure:      ReconfigureAuth,
		SaveFavorites:    SaveFavorites,
		SaveLastProject:  SaveLastProject,
		// Asked once, in the background, and only reported if there is something to
		// report. Printing it here instead would put the notice on the screen the alt
		// buffer is about to replace, where nobody ever saw it.
		CheckUpdate: func() string { return LatestVersion(version) },
	})
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
