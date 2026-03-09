package util

import (
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// GitRemote represents a parsed git remote with its name and GitLab coordinates.
type GitRemote struct {
	Name string
	Host string
	Path string
}

// sshRemoteRegex matches SSH-style git remote URLs: git@host:path.git
var sshRemoteRegex = regexp.MustCompile(`^[\w.-]+@([\w.-]+):(.+)$`)

// ParseGitLabRemote extracts the host and project path from a git remote URL.
// It supports both HTTPS and SSH formats, strips the .git suffix, and handles
// subgroup paths. Returns empty strings if the URL cannot be parsed.
func ParseGitLabRemote(remoteURL string) (host, path string) {
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteURL == "" {
		return "", ""
	}

	// Try SSH format first: git@host:owner/project.git
	if matches := sshRemoteRegex.FindStringSubmatch(remoteURL); matches != nil {
		host = matches[1]
		path = strings.TrimSuffix(matches[2], ".git")
		path = strings.TrimPrefix(path, "/")
		if host == "" || path == "" || strings.Contains(path, "..") {
			return "", ""
		}
		return host, path
	}

	// Try HTTPS format: https://host/owner/project.git
	u, err := url.Parse(remoteURL)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return "", ""
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", ""
	}

	host = u.Host
	path = strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	if path == "" || strings.Contains(path, "..") {
		return "", ""
	}

	return host, path
}

// ParseGitRemoteOutput parses the output of `git remote -v` and returns a
// deduplicated, sorted list of GitRemote entries. Only fetch lines are
// processed. The "origin" remote is sorted first, followed by others in
// alphabetical order. Remotes with unparseable URLs are skipped.
func ParseGitRemoteOutput(output string) []GitRemote {
	if output == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var remotes []GitRemote

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		// Only process fetch lines
		if !strings.HasSuffix(line, "(fetch)") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		remoteURL := parts[1]

		// Deduplicate by name
		if _, ok := seen[name]; ok {
			continue
		}

		host, path := ParseGitLabRemote(remoteURL)
		if host == "" || path == "" {
			continue
		}

		seen[name] = struct{}{}
		remotes = append(remotes, GitRemote{
			Name: name,
			Host: host,
			Path: path,
		})
	}

	// Sort: origin first, then alphabetical
	sort.Slice(remotes, func(i, j int) bool {
		if remotes[i].Name == "origin" {
			return true
		}
		if remotes[j].Name == "origin" {
			return false
		}
		return remotes[i].Name < remotes[j].Name
	})

	return remotes
}

// DetectGitRemotes runs `git remote -v` in the current directory and returns
// parsed remotes. Returns nil if git is not available or not in a repository.
func DetectGitRemotes() []GitRemote {
	cmd := exec.Command("git", "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return ParseGitRemoteOutput(string(out))
}
