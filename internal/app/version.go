package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	releaseURL   = "https://api.github.com/repos/Malvi1697/lazyglab/releases/latest"
	checkTimeout = 2 * time.Second
	releasesPage = "https://github.com/Malvi1697/lazyglab/releases"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// CheckForUpdate checks GitHub for a newer release and prints a notice to stderr.
// Silently returns on any error (timeout, network, parse failure).
func CheckForUpdate(currentVersion string) {
	client := &http.Client{Timeout: checkTimeout}
	resp, err := client.Get(releaseURL)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	// Strip -dev suffix for comparison
	current = strings.TrimSuffix(current, "-dev")

	if latest != "" && latest != current && isNewer(latest, current) {
		fmt.Fprintf(os.Stderr, "  Update available: v%s → v%s (%s)\n", current, latest, releasesPage)
	}
}

// isNewer returns true if version a is newer than version b.
// Compares dot-separated numeric segments (e.g. "0.2.0" > "0.1.0").
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
