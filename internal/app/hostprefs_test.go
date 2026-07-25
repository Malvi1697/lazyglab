package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFavoritesFor(t *testing.T) {
	cfg := &Config{Hosts: map[string]HostConfig{
		"gitlab.olc.cz": {Token: "t", Favorites: []string{"a/b", "c/d"}},
		"gitlab.com":    {Token: "t"},
	}}

	if got := FavoritesFor(cfg, "gitlab.olc.cz"); len(got) != 2 || got[0] != "a/b" {
		t.Errorf("FavoritesFor = %v, want [a/b c/d]", got)
	}
	if got := FavoritesFor(cfg, "gitlab.com"); len(got) != 0 {
		t.Errorf("a host without favorites should return none, got %v", got)
	}
	if got := FavoritesFor(cfg, "unknown.host"); len(got) != 0 {
		t.Errorf("an unknown host should return none, got %v", got)
	}
}

func TestSaveFavorites_PreservesTokenAndSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", path)

	refresh := 45
	initial := &Config{
		DefaultHost: "gitlab.olc.cz",
		Hosts: map[string]HostConfig{
			"gitlab.olc.cz": {Token: "secret", APIHost: "api.gitlab.olc.cz"},
			"gitlab.com":    {Token: "other"},
		},
		Settings: Settings{DefaultView: "pipelines", RefreshInterval: &refresh},
	}
	if err := SaveConfig(initial); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := SaveFavorites("gitlab.olc.cz", []string{"group/one", "group/two"}); err != nil {
		t.Fatalf("SaveFavorites: %v", err)
	}

	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	host := reloaded.Hosts["gitlab.olc.cz"]
	if len(host.Favorites) != 2 || host.Favorites[1] != "group/two" {
		t.Errorf("favorites = %v, want [group/one group/two]", host.Favorites)
	}
	if host.Token != "secret" {
		t.Errorf("token must survive a favorites write, got %q", host.Token)
	}
	if host.APIHost != "api.gitlab.olc.cz" {
		t.Errorf("api_host must survive, got %q", host.APIHost)
	}
	if reloaded.Hosts["gitlab.com"].Token != "other" {
		t.Error("other hosts must survive")
	}
	if reloaded.Settings.DefaultView != "pipelines" {
		t.Errorf("settings must survive, got default_view %q", reloaded.Settings.DefaultView)
	}
	if reloaded.Settings.RefreshInterval == nil || *reloaded.Settings.RefreshInterval != 45 {
		t.Error("refresh_interval must survive")
	}
}

func TestSaveFavorites_EmptyListClearsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", path)

	if err := SaveConfig(&Config{Hosts: map[string]HostConfig{
		"h": {Token: "t", Favorites: []string{"a/b"}},
	}}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := SaveFavorites("h", nil); err != nil {
		t.Fatalf("SaveFavorites: %v", err)
	}

	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := reloaded.Hosts["h"].Favorites; len(got) != 0 {
		t.Errorf("favorites = %v, want none", got)
	}
	if reloaded.Hosts["h"].Token != "t" {
		t.Error("clearing favorites must not drop the token")
	}
}

func TestSaveLastProject_RoundTripAndPreservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", path)

	if err := SaveConfig(&Config{
		DefaultHost: "gitlab.olc.cz",
		Hosts: map[string]HostConfig{
			"gitlab.olc.cz": {Token: "secret", Favorites: []string{"g/starred"}},
		},
		Settings: Settings{DefaultView: "commits"},
	}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	if err := SaveLastProject("gitlab.olc.cz", "g/worked-on"); err != nil {
		t.Fatalf("SaveLastProject: %v", err)
	}

	reloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := LastProjectFor(reloaded, "gitlab.olc.cz"); got != "g/worked-on" {
		t.Errorf("last project = %q, want g/worked-on", got)
	}
	host := reloaded.Hosts["gitlab.olc.cz"]
	if host.Token != "secret" {
		t.Errorf("token must survive, got %q", host.Token)
	}
	if len(host.Favorites) != 1 || host.Favorites[0] != "g/starred" {
		t.Errorf("favorites must survive, got %v", host.Favorites)
	}
	if reloaded.Settings.DefaultView != "commits" {
		t.Error("settings must survive")
	}
}

func TestSaveLastProject_UnchangedWritesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	t.Setenv("LAZYGLAB_CONFIG", path)

	if err := SaveConfig(&Config{Hosts: map[string]HostConfig{
		"h": {Token: "t", LastProject: "g/p"},
	}}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if err := SaveLastProject("h", "g/p"); err != nil {
		t.Fatalf("SaveLastProject: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("re-selecting the same project should not rewrite the config")
	}
}

func TestLastProjectFor_UnsetIsEmpty(t *testing.T) {
	cfg := &Config{Hosts: map[string]HostConfig{"h": {Token: "t"}}}
	if got := LastProjectFor(cfg, "h"); got != "" {
		t.Errorf("want empty, got %q", got)
	}
	if got := LastProjectFor(cfg, "other"); got != "" {
		t.Errorf("unknown host should be empty, got %q", got)
	}
}
