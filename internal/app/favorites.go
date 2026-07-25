package app

// FavoritesFor returns the starred project paths configured for a host.
func FavoritesFor(cfg *Config, host string) []string {
	return cfg.Hosts[host].Favorites
}

// SaveFavorites persists the starred project paths for a host, leaving that
// host's token and api_host, every other host and all settings untouched.
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
