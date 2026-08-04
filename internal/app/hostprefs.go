package app

// FavoritesFor returns the starred project paths configured for a host.
func FavoritesFor(cfg *Config, host string) []string {
	return cfg.Hosts[host].Favorites
}

// LastProjectFor returns the project path last selected on a host.
func LastProjectFor(cfg *Config, host string) string {
	return cfg.Hosts[host].LastProject
}

// SaveLastProject records the project path last selected on a host, so the next launch
// resumes there.
func SaveLastProject(host, path string) error {
	cfg, err := loadConfigForUpdate()
	if err != nil {
		return err
	}

	entry := cfg.Hosts[host]
	if entry.LastProject == path {
		return nil // nothing to write
	}
	entry.LastProject = path
	cfg.Hosts[host] = entry

	return SaveConfig(cfg)
}

// SaveFavorites persists the starred project paths for a host, leaving that host's
// token and api_host, every other host and all settings untouched.
func SaveFavorites(host string, favorites []string) error {
	cfg, err := loadConfigForUpdate()
	if err != nil {
		return err
	}

	entry := cfg.Hosts[host]
	if len(favorites) == 0 {
		// Keep the file tidy: an empty list is omitted rather than written as [].
		entry.Favorites = nil
	} else {
		entry.Favorites = favorites
	}
	cfg.Hosts[host] = entry

	return SaveConfig(cfg)
}
