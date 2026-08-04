package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	releaseURL   = "https://api.github.com/repos/Malvi1697/lazyglab/releases/latest"
	releasesPage = "https://github.com/Malvi1697/lazyglab/releases"
	checkTimeout = 2 * time.Second
	// The release JSON is a few kilobytes; anything larger is not the API answering.
	maxReleaseJSON = 1 << 20
)

// release is the part of GitHub's release JSON we act on: which version it is and which
// files it ships.
type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// assetURL returns the download URL of the named file, or "" when the release does not
// carry it.
func (r release) assetURL(name string) string {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL
		}
	}
	return ""
}

// fetchRelease reads a GitHub release description.
func fetchRelease(url string, timeout time.Duration) (release, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return release{}, fmt.Errorf("asking GitHub for the latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub answered %s for the latest release", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReleaseJSON)).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("reading the release description: %w", err)
	}
	return rel, nil
}

// LatestVersion returns the newest released version ("0.5.0") when it is newer than the
// running one, and "" when we are up to date or the check failed.
func LatestVersion(currentVersion string) string {
	return latestVersionFrom(releaseURL, currentVersion)
}

func latestVersionFrom(url, currentVersion string) string {
	rel, err := fetchRelease(url, checkTimeout)
	if err != nil {
		return ""
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if latest == "" || !isNewer(latest, versionNumber(currentVersion)) {
		return ""
	}
	return latest
}

// versionNumber reduces a build's version to the numbers isNewer compares: "v0.4.0" and
// "0.4.0-dev" and "0.4.0-2-gabc-dirty" are all 0.4.0.
func versionNumber(v string) string {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

// isNewer returns true if version a is newer than version b.
func isNewer(a, b string) bool {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		if i < len(aParts) {
			_, _ = fmt.Sscanf(aParts[i], "%d", &aNum)
		}
		if i < len(bParts) {
			_, _ = fmt.Sscanf(bParts[i], "%d", &bNum)
		}
		if aNum > bNum {
			return true
		}
		if aNum < bNum {
			return false
		}
	}
	return false
}
