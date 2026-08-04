package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true},
		{"0.1.0", "0.2.0", false},
		{"0.1.0", "0.1.0", false},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"0.1.1", "0.1.0", true},
		{"0.1.0", "0.1.1", false},
		{"1.0.0", "0.0.1", true},
		{"0.2", "0.1.0", true},
		{"0.1.0", "0.2", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := isNewer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestVersionNumber_ADevBuildIsStillItsVersion(t *testing.T) {
	// The version comes from an ldflag, and a local build carries whatever git describe
	// said.
	for _, v := range []string{"0.4.0", "v0.4.0", "0.4.0-dev", "v0.4.0-2-gabc1234", "0.4.0-dirty", "0.4.0+build5"} {
		if got := versionNumber(v); got != "0.4.0" {
			t.Errorf("versionNumber(%q) = %q, want 0.4.0", v, got)
		}
	}
}

// releaseServer serves one release description at the URL it returns.
func releaseServer(t *testing.T, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestLatestVersion_OnlyNamesAReleaseWorthInstalling(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		current string
		want    string
	}{
		{"newer release", `{"tag_name": "v0.2.0"}`, "0.1.0", "0.2.0"},
		{"same version", `{"tag_name": "v0.1.0"}`, "0.1.0", ""},
		{"remote is older", `{"tag_name": "v0.0.9"}`, "0.1.0", ""},
		{"v-prefixed current", `{"tag_name": "v0.2.0"}`, "v0.1.0", "0.2.0"},
		// A dev build of 0.1.0 is not yet 0.1.0's successor, so 0.2.0 is news and 0.1.0 is
		// not.
		{"dev build, newer release", `{"tag_name": "v0.2.0"}`, "0.1.0-dev", "0.2.0"},
		{"dev build of the newest", `{"tag_name": "v0.1.0"}`, "0.1.0-dev", ""},
		{"no tag", `{"tag_name": ""}`, "0.1.0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := latestVersionFrom(releaseServer(t, tt.body), tt.current); got != tt.want {
				t.Errorf("latestVersionFrom(%s) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}

func TestLatestVersion_AFailedCheckIsSilent(t *testing.T) {
	// Nobody asked for this check, so a GitHub outage, a captive portal or a mangled
	// answer must not put anything on screen.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()

	for name, url := range map[string]string{
		"server error": broken.URL,
		"invalid JSON": releaseServer(t, `not json`),
		"unreachable":  "http://192.0.2.1:1",
	} {
		if got := latestVersionFrom(url, "0.1.0"); got != "" {
			t.Errorf("%s: got %q, want no version", name, got)
		}
	}
}
