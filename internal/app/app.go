package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui"
)

// Run initializes the application and starts the TUI.
func Run() error {
	cfg, err := LoadGlabConfig()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	clients := make(map[string]*gitlab.Client)
	var hostNames []string
	for host, hostCfg := range cfg.Hosts {
		if hostCfg.Token == "" {
			continue // skip hosts without a token
		}
		protocol := hostCfg.APIProtocol
		if protocol == "" {
			protocol = "https"
		}
		if protocol != "https" {
			return fmt.Errorf("refusing to connect to %s over insecure protocol %q (only https is supported)", host, protocol)
		}
		apiHost := hostCfg.APIHost
		if apiHost == "" {
			apiHost = host
		}
		baseURL := fmt.Sprintf("%s://%s/api/v4", protocol, apiHost)
		client, err := gitlab.NewClient(hostCfg.Token, baseURL, host)
		if err != nil {
			return fmt.Errorf("creating GitLab client for %s: %w", host, err)
		}
		clients[host] = client
		hostNames = append(hostNames, host)
	}

	if len(clients) == 0 {
		return fmt.Errorf("no GitLab hosts with tokens found in glab config")
	}

	// Put the default host first if it has a token
	if cfg.DefaultHost != "" {
		for i, h := range hostNames {
			if h == cfg.DefaultHost && i > 0 {
				hostNames[0], hostNames[i] = hostNames[i], hostNames[0]
				break
			}
		}
	}

	model := tui.NewApp(clients, hostNames)
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
