package util

import (
	"testing"
)

func TestParseGitLabRemote_HTTPS(t *testing.T) {
	host, path := ParseGitLabRemote("https://gitlab.com/owner/project.git")
	if host != "gitlab.com" {
		t.Errorf("expected host=gitlab.com, got %q", host)
	}
	if path != "owner/project" {
		t.Errorf("expected path=owner/project, got %q", path)
	}
}

func TestParseGitLabRemote_HTTPSNoGitSuffix(t *testing.T) {
	host, path := ParseGitLabRemote("https://gitlab.com/owner/project")
	if host != "gitlab.com" {
		t.Errorf("expected host=gitlab.com, got %q", host)
	}
	if path != "owner/project" {
		t.Errorf("expected path=owner/project, got %q", path)
	}
}

func TestParseGitLabRemote_SSH(t *testing.T) {
	host, path := ParseGitLabRemote("git@gitlab.com:owner/project.git")
	if host != "gitlab.com" {
		t.Errorf("expected host=gitlab.com, got %q", host)
	}
	if path != "owner/project" {
		t.Errorf("expected path=owner/project, got %q", path)
	}
}

func TestParseGitLabRemote_SSHNoGitSuffix(t *testing.T) {
	host, path := ParseGitLabRemote("git@gitlab.com:owner/project")
	if host != "gitlab.com" {
		t.Errorf("expected host=gitlab.com, got %q", host)
	}
	if path != "owner/project" {
		t.Errorf("expected path=owner/project, got %q", path)
	}
}

func TestParseGitLabRemote_Subgroups(t *testing.T) {
	tests := []struct {
		url      string
		wantHost string
		wantPath string
	}{
		{
			url:      "https://gitlab.com/org/team/project.git",
			wantHost: "gitlab.com",
			wantPath: "org/team/project",
		},
		{
			url:      "git@gitlab.com:org/team/project.git",
			wantHost: "gitlab.com",
			wantPath: "org/team/project",
		},
		{
			url:      "https://gitlab.com/org/team/subteam/project.git",
			wantHost: "gitlab.com",
			wantPath: "org/team/subteam/project",
		},
		{
			url:      "git@gitlab.com:org/team/subteam/project.git",
			wantHost: "gitlab.com",
			wantPath: "org/team/subteam/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host, path := ParseGitLabRemote(tt.url)
			if host != tt.wantHost {
				t.Errorf("expected host=%q, got %q", tt.wantHost, host)
			}
			if path != tt.wantPath {
				t.Errorf("expected path=%q, got %q", tt.wantPath, path)
			}
		})
	}
}

func TestParseGitLabRemote_CustomHost(t *testing.T) {
	tests := []struct {
		url      string
		wantHost string
		wantPath string
	}{
		{
			url:      "https://gitlab.mycompany.com/infra/deploy-tools.git",
			wantHost: "gitlab.mycompany.com",
			wantPath: "infra/deploy-tools",
		},
		{
			url:      "git@gitlab.mycompany.com:infra/deploy-tools.git",
			wantHost: "gitlab.mycompany.com",
			wantPath: "infra/deploy-tools",
		},
		{
			url:      "https://git.internal.io/team/subgroup/project.git",
			wantHost: "git.internal.io",
			wantPath: "team/subgroup/project",
		},
		{
			url:      "git@git.internal.io:team/subgroup/project.git",
			wantHost: "git.internal.io",
			wantPath: "team/subgroup/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			host, path := ParseGitLabRemote(tt.url)
			if host != tt.wantHost {
				t.Errorf("expected host=%q, got %q", tt.wantHost, host)
			}
			if path != tt.wantPath {
				t.Errorf("expected path=%q, got %q", tt.wantPath, path)
			}
		})
	}
}

func TestParseGitLabRemote_Invalid(t *testing.T) {
	tests := []string{
		"",
		"not-a-url",
		"ftp://gitlab.com/owner/project.git",
		"://missing-scheme",
		"git@",
		"git@:no-host",
		"https://",
	}

	for _, url := range tests {
		t.Run(url, func(t *testing.T) {
			host, path := ParseGitLabRemote(url)
			if host != "" || path != "" {
				t.Errorf("expected empty strings for invalid URL %q, got host=%q path=%q", url, host, path)
			}
		})
	}
}

func TestParseGitRemoteOutput(t *testing.T) {
	output := `origin	git@gitlab.com:owner/project.git (fetch)
origin	git@gitlab.com:owner/project.git (push)
upstream	https://gitlab.com/upstream/project.git (fetch)
upstream	https://gitlab.com/upstream/project.git (push)
`
	remotes := ParseGitRemoteOutput(output)
	if len(remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(remotes))
	}

	// origin should be first
	if remotes[0].Name != "origin" {
		t.Errorf("expected first remote to be origin, got %q", remotes[0].Name)
	}
	if remotes[0].Host != "gitlab.com" {
		t.Errorf("expected origin host=gitlab.com, got %q", remotes[0].Host)
	}
	if remotes[0].Path != "owner/project" {
		t.Errorf("expected origin path=owner/project, got %q", remotes[0].Path)
	}

	if remotes[1].Name != "upstream" {
		t.Errorf("expected second remote to be upstream, got %q", remotes[1].Name)
	}
	if remotes[1].Host != "gitlab.com" {
		t.Errorf("expected upstream host=gitlab.com, got %q", remotes[1].Host)
	}
	if remotes[1].Path != "upstream/project" {
		t.Errorf("expected upstream path=upstream/project, got %q", remotes[1].Path)
	}
}

func TestParseGitRemoteOutput_OriginFirstRegardlessOfOrder(t *testing.T) {
	// origin appears after other remotes in the input
	output := `backup	https://gitlab.com/backup/project.git (fetch)
backup	https://gitlab.com/backup/project.git (push)
fork	git@gitlab.com:fork/project.git (fetch)
fork	git@gitlab.com:fork/project.git (push)
origin	https://gitlab.com/owner/project.git (fetch)
origin	https://gitlab.com/owner/project.git (push)
`
	remotes := ParseGitRemoteOutput(output)
	if len(remotes) != 3 {
		t.Fatalf("expected 3 remotes, got %d", len(remotes))
	}

	if remotes[0].Name != "origin" {
		t.Errorf("expected first remote to be origin, got %q", remotes[0].Name)
	}
	// remaining should be alphabetical
	if remotes[1].Name != "backup" {
		t.Errorf("expected second remote to be backup, got %q", remotes[1].Name)
	}
	if remotes[2].Name != "fork" {
		t.Errorf("expected third remote to be fork, got %q", remotes[2].Name)
	}
}

func TestParseGitRemoteOutput_Empty(t *testing.T) {
	remotes := ParseGitRemoteOutput("")
	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes for empty output, got %d", len(remotes))
	}
}

func TestParseGitRemoteOutput_UnparseableURLsSkipped(t *testing.T) {
	output := `origin	not-a-valid-url (fetch)
origin	not-a-valid-url (push)
`
	remotes := ParseGitRemoteOutput(output)
	if len(remotes) != 0 {
		t.Errorf("expected 0 remotes for unparseable URLs, got %d", len(remotes))
	}
}
