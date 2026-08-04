package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// tarGz builds a release archive: the binary plus the README and licence GoReleaser
// ships beside it, so the extraction has to pick the right one.
func tarGz(t *testing.T, binary string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := []struct {
		name, body string
		mode       int64
	}{
		{"README.md", "# lazyglab", 0o644},
		{"LICENSE", "MIT", 0o644},
		{binaryName, binary, 0o755},
	}
	for _, f := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: f.name, Mode: f.mode, Size: int64(len(f.body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatalf("writing the test archive: %v", err)
		}
		if _, err := tw.Write([]byte(f.body)); err != nil {
			t.Fatalf("writing the test archive: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("closing the test archive: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing the test archive: %v", err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, binary string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"README.md": "# lazyglab", binaryName + ".exe": binary} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("writing the test archive: %v", err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatalf("writing the test archive: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing the test archive: %v", err)
	}
	return buf.Bytes()
}

// fakeRelease serves a release of the given version whose archive contains the given
// binary, with a checksums.txt to match.
func fakeRelease(t *testing.T, version, binary string, corrupt bool) string {
	t.Helper()
	name := assetName(version, runtime.GOOS, runtime.GOARCH)
	archive := tarGz(t, binary)
	if runtime.GOOS == "windows" {
		archive = zipArchive(t, binary)
	}
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%s  %s\n%s  %s\n",
		hex.EncodeToString(sum[:]), name,
		strings.Repeat("0", 64), "lazyglab_"+version+"_other_arch.tar.gz")
	if corrupt {
		archive = append(archive, "tampered"...)
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	mux.HandleFunc("/"+name, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/"+checksumsFile, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})
	mux.HandleFunc("/release", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name": "v%s", "assets": [
			{"name": %q, "browser_download_url": %q},
			{"name": %q, "browser_download_url": %q}
		]}`, version, name, server.URL+"/"+name, checksumsFile, server.URL+"/"+checksumsFile)
	})
	return server.URL + "/release"
}

// installedBinary writes a stand-in for the running binary and returns its path.
func installedBinary(t *testing.T, content string) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), binaryName)
	if err := os.WriteFile(exe, []byte(content), 0o755); err != nil {
		t.Fatalf("writing the test binary: %v", err)
	}
	return exe
}

func TestSelfUpdate_ReplacesTheBinaryInPlace(t *testing.T) {
	exe := installedBinary(t, "old binary")
	url := fakeRelease(t, "0.5.0", "new binary", false)

	var out bytes.Buffer
	if err := selfUpdate(url, "0.4.0", exe, &out); err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("reading the updated binary: %v", err)
	}
	if string(got) != "new binary" {
		t.Errorf("binary = %q, want the one from the release", got)
	}
	// It has to stay executable, or the next launch fails with a permission error that
	// says nothing about what happened.
	info, err := os.Stat(exe)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode = %v, want the executable bits kept", info.Mode().Perm())
	}
	if !strings.Contains(out.String(), "v0.4.0 → v0.5.0") {
		t.Errorf("output = %q, want it to name both versions", out.String())
	}
	if !strings.Contains(out.String(), "Checksum verified") {
		t.Errorf("output = %q, want the verification said out loud", out.String())
	}

	// Nothing left behind: a leftover .old or a temp file in a directory on PATH is litter
	// at best and a stale binary someone runs at worst.
	entries, err := os.ReadDir(filepath.Dir(exe))
	if err != nil {
		t.Fatalf("reading the install dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != binaryName {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("install dir holds %v, want only %s", names, binaryName)
	}
}

func TestSelfUpdate_ABadChecksumLeavesTheOldBinaryAlone(t *testing.T) {
	// The whole point of verifying: a damaged or substituted download must never end up
	// being the thing we execute next.
	exe := installedBinary(t, "old binary")
	url := fakeRelease(t, "0.5.0", "new binary", true)

	err := selfUpdate(url, "0.4.0", exe, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected a checksum error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("error = %v, want it to say the checksum did not match", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old binary" {
		t.Errorf("binary = %q, want the old one untouched", got)
	}
}

func TestSelfUpdate_UpToDateDoesNothing(t *testing.T) {
	exe := installedBinary(t, "current binary")
	url := fakeRelease(t, "0.4.0", "new binary", false)

	var out bytes.Buffer
	if err := selfUpdate(url, "0.4.0", exe, &out); err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}
	if !strings.Contains(out.String(), "already the newest") {
		t.Errorf("output = %q, want it to say we are up to date", out.String())
	}
	if got, _ := os.ReadFile(exe); string(got) != "current binary" {
		t.Errorf("binary = %q, want it left alone", got)
	}
}

func TestSelfUpdate_ADevBuildOfTheNewestIsUpToDate(t *testing.T) {
	// Running a locally built v0.4.0-dev and being told to download v0.4.0 would silently
	// replace the build under test.
	exe := installedBinary(t, "dev binary")
	url := fakeRelease(t, "0.4.0", "release binary", false)

	var out bytes.Buffer
	if err := selfUpdate(url, "0.4.0-dev", exe, &out); err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "dev binary" {
		t.Errorf("binary = %q, want the dev build left in place", got)
	}
}

func TestSelfUpdate_NoBuildForThisPlatform(t *testing.T) {
	exe := installedBinary(t, "old binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.5.0", "assets": [
			{"name": "lazyglab_0.5.0_plan9_mips.tar.gz", "browser_download_url": "http://example.invalid/a"},
			{"name": "checksums.txt", "browser_download_url": "http://example.invalid/c"}
		]}`)
	}))
	defer server.Close()

	err := selfUpdate(server.URL, "0.4.0", exe, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when the release has nothing for us")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("error = %v, want it to name the platform it looked for", err)
	}
}

func TestSelfUpdate_RefusesWithoutChecksums(t *testing.T) {
	// A release with no checksums.txt cannot be verified, and downloading a binary we
	// cannot verify is worse than not updating.
	exe := installedBinary(t, "old binary")
	name := assetName("0.5.0", runtime.GOOS, runtime.GOARCH)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name": "v0.5.0", "assets": [
			{"name": %q, "browser_download_url": "http://example.invalid/a"}
		]}`, name)
	}))
	defer server.Close()

	err := selfUpdate(server.URL, "0.4.0", exe, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), checksumsFile) {
		t.Errorf("error = %v, want it to refuse for want of %s", err, checksumsFile)
	}
}

func TestAssetName_MatchesWhatGoReleaserPublishes(t *testing.T) {
	// These are the names on the v0.4.0 release; getting one wrong means the update fails
	// only on the platform nobody tested.
	tests := map[string]string{
		"darwin/arm64":  "lazyglab_0.4.0_darwin_arm64.tar.gz",
		"linux/amd64":   "lazyglab_0.4.0_linux_amd64.tar.gz",
		"windows/amd64": "lazyglab_0.4.0_windows_amd64.zip",
	}
	for platform, want := range tests {
		goos, goarch, _ := strings.Cut(platform, "/")
		if got := assetName("0.4.0", goos, goarch); got != want {
			t.Errorf("assetName(%s) = %q, want %q", platform, got, want)
		}
	}
}

func TestChecksumFor(t *testing.T) {
	checksums := "aaa  lazyglab_0.5.0_linux_amd64.tar.gz\nbbb  lazyglab_0.5.0_darwin_arm64.tar.gz\n"

	got, err := checksumFor(checksums, "lazyglab_0.5.0_darwin_arm64.tar.gz")
	if err != nil {
		t.Fatalf("checksumFor: %v", err)
	}
	if got != "bbb" {
		t.Errorf("got %q, want bbb", got)
	}
	if _, err := checksumFor(checksums, "lazyglab_0.5.0_windows_arm64.zip"); err == nil {
		t.Error("expected an error for a file the list does not mention")
	}
}

func TestExtractBinary_PicksTheExecutableNotTheReadme(t *testing.T) {
	tar := tarGz(t, "the binary")
	got, err := extractBinary("lazyglab_0.5.0_linux_amd64.tar.gz", tar)
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if string(got) != "the binary" {
		t.Errorf("got %q, want the executable", got)
	}

	// The Windows archive holds lazyglab.exe, and it is read by the same code path.
	got, err = extractBinary("lazyglab_0.5.0_windows_amd64.zip", zipArchive(t, "the exe"))
	if err != nil {
		t.Fatalf("extractBinary (zip): %v", err)
	}
	if string(got) != "the exe" {
		t.Errorf("got %q, want the executable", got)
	}

	if _, err := extractBinary("lazyglab_0.5.0_linux_amd64.tar.gz", []byte("not an archive")); err == nil {
		t.Error("expected an error for something that is not an archive")
	}
}

func TestManagedBy_LeavesPackageManagersAlone(t *testing.T) {
	// Overwriting a packaged file makes the package manager lie about what is installed,
	// and its next upgrade would undo us.
	tests := map[string]string{
		"/opt/homebrew/Cellar/lazyglab/0.4.0/bin/lazyglab": "Homebrew",
		"/usr/bin/lazyglab":                          "a system package",
		"/nix/store/abc-lazyglab-0.4.0/bin/lazyglab": "Nix",
		"/usr/local/bin/lazyglab":                    "",
		"/home/jan/go/bin/lazyglab":                  "",
	}
	for path, want := range tests {
		if got := managedBy(path); got != want {
			t.Errorf("managedBy(%q) = %q, want %q", path, got, want)
		}
	}
}
