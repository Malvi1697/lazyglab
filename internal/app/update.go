package app

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	binaryName      = "lazyglab"
	checksumsFile   = "checksums.txt"
	downloadTimeout = 2 * time.Minute
	// The archive is a few megabytes; the cap is there so a wrong URL cannot make
	// us read forever into memory.
	maxArchiveBytes = 100 << 20
)

// SelfUpdate replaces the running binary with the newest release, verifying its
// checksum first. It is what `lazyglab update` does.
func SelfUpdate(currentVersion string, out io.Writer) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding the running binary: %w", err)
	}
	// Installers often leave a symlink behind (Homebrew, /usr/local/bin pointing
	// elsewhere). Replacing the link with a file would strand whatever it pointed
	// at, so we follow it and replace the real thing.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return selfUpdate(releaseURL, currentVersion, exe, out)
}

func selfUpdate(apiURL, currentVersion, exe string, out io.Writer) error {
	current := versionNumber(currentVersion)

	rel, err := fetchRelease(apiURL, downloadTimeout)
	if err != nil {
		return err
	}
	latest := strings.TrimPrefix(rel.TagName, "v")
	if latest == "" {
		return fmt.Errorf("the latest release has no version tag (%s)", releasesPage)
	}
	if !isNewer(latest, current) {
		_, _ = fmt.Fprintf(out, "lazyglab v%s is already the newest version.\n", current)
		return nil
	}

	// Say who owns this copy before downloading anything: replacing a file behind a
	// package manager's back leaves it lying about what is installed, and the next
	// upgrade would silently undo ours.
	if by := managedBy(exe); by != "" {
		return fmt.Errorf("this copy of lazyglab was installed with %s, so it should be updated the same way:\n  %s",
			by, upgradeHint(by))
	}
	if err := ensureWritable(exe); err != nil {
		return err
	}

	name := assetName(latest, runtime.GOOS, runtime.GOARCH)
	assetURL := rel.assetURL(name)
	if assetURL == "" {
		return fmt.Errorf("release v%s has no build for %s/%s (%s)", latest, runtime.GOOS, runtime.GOARCH, releasesPage)
	}
	sumsURL := rel.assetURL(checksumsFile)
	if sumsURL == "" {
		return fmt.Errorf("release v%s ships no %s, so the download cannot be verified", latest, checksumsFile)
	}

	_, _ = fmt.Fprintf(out, "Downloading lazyglab v%s (%s/%s)...\n", latest, runtime.GOOS, runtime.GOARCH)
	archive, err := download(assetURL)
	if err != nil {
		return err
	}
	sums, err := download(sumsURL)
	if err != nil {
		return err
	}

	want, err := checksumFor(string(sums), name)
	if err != nil {
		return err
	}
	got := hex.EncodeToString(sha256Sum(archive))
	if got != want {
		// Either the download is damaged or it is not the file the release signed.
		// Both mean we do not run it.
		return fmt.Errorf("checksum mismatch for %s:\n  expected %s\n  got      %s", name, want, got)
	}
	_, _ = fmt.Fprintln(out, "Checksum verified.")

	bin, err := extractBinary(name, archive)
	if err != nil {
		return err
	}
	if err := replaceExecutable(exe, bin); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(out, "Updated lazyglab v%s → v%s (%s)\n", current, latest, exe)
	return nil
}

// assetName is the release file for a platform, matching the name template in
// .goreleaser.yaml. Keep the two in step.
func assetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("%s_%s_%s_%s.%s", binaryName, version, goos, goarch, ext)
}

// download reads a release asset into memory. The archives are a few megabytes,
// so this saves a temporary file and the cleanup that goes with it.
func download(url string) ([]byte, error) {
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: %s", url, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes))
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	return body, nil
}

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}

// checksumFor finds a file's expected hash in a checksums.txt ("<sha256>  <name>"
// per line).
func checksumFor(checksums, name string) (string, error) {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum listed for %s", name)
}

// extractBinary pulls the lazyglab executable out of a release archive.
func extractBinary(assetName string, archive []byte) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return binaryFromZip(archive)
	}
	return binaryFromTarGz(archive)
}

// isBinaryEntry reports whether an archive entry is the executable itself rather
// than the README and licence shipped beside it.
func isBinaryEntry(path string) bool {
	base := filepath.Base(filepath.ToSlash(path))
	return base == binaryName || base == binaryName+".exe"
}

func binaryFromTarGz(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("reading the downloaded archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading the downloaded archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg || !isBinaryEntry(hdr.Name) {
			continue
		}
		bin, err := io.ReadAll(io.LimitReader(tr, maxArchiveBytes))
		if err != nil {
			return nil, fmt.Errorf("reading %s out of the archive: %w", hdr.Name, err)
		}
		return bin, nil
	}
	return nil, fmt.Errorf("the archive contains no %s executable", binaryName)
}

func binaryFromZip(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("reading the downloaded archive: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isBinaryEntry(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("reading %s out of the archive: %w", f.Name, err)
		}
		defer func() { _ = rc.Close() }()
		bin, err := io.ReadAll(io.LimitReader(rc, maxArchiveBytes))
		if err != nil {
			return nil, fmt.Errorf("reading %s out of the archive: %w", f.Name, err)
		}
		return bin, nil
	}
	return nil, fmt.Errorf("the archive contains no %s executable", binaryName)
}

// replaceExecutable swaps a new binary in for the running one. The new file is
// written beside the old one so the rename stays on one filesystem and is atomic:
// at no point is there a half-written lazyglab on the path.
func replaceExecutable(exe string, bin []byte) error {
	mode := os.FileMode(0o755)
	if info, err := os.Stat(exe); err == nil {
		mode = info.Mode().Perm()
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, "."+binaryName+"-new-*")
	if err != nil {
		return fmt.Errorf("writing the new binary next to %s: %w", exe, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(bin); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("writing the new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("writing the new binary: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		cleanup()
		return fmt.Errorf("making the new binary executable: %w", err)
	}

	// The old binary is moved aside rather than deleted: on Windows the running
	// image cannot be removed, and everywhere else it is the thing we put back if
	// the swap fails halfway.
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		cleanup()
		return fmt.Errorf("moving the old binary aside: %w", err)
	}
	if err := os.Rename(tmpName, exe); err != nil {
		_ = os.Rename(old, exe)
		cleanup()
		return fmt.Errorf("putting the new binary in place: %w", err)
	}
	// Best effort: Windows keeps the file we are executing locked until we exit.
	_ = os.Remove(old)
	return nil
}

// ensureWritable checks we can create a file beside the binary, which is what the
// swap needs — the binary's own mode says nothing about the directory.
func ensureWritable(exe string) error {
	dir := filepath.Dir(exe)
	probe, err := os.CreateTemp(dir, "."+binaryName+"-check-*")
	if err != nil {
		return fmt.Errorf("%s is not writable by this user, so lazyglab cannot replace itself:\n  sudo lazyglab update\nor reinstall with: curl -sL https://raw.githubusercontent.com/Malvi1697/lazyglab/master/install.sh | sh", dir)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return nil
}

// managedBy names the package manager that owns this copy of the binary, or ""
// when we installed it ourselves and are free to replace it.
func managedBy(exe string) string {
	path := filepath.ToSlash(exe)
	switch {
	case strings.Contains(path, "/Cellar/"), strings.Contains(path, "/linuxbrew/"):
		return "Homebrew"
	case strings.HasPrefix(path, "/usr/bin/"), strings.HasPrefix(path, "/bin/"):
		// The .deb and .rpm both install here; nothing else should.
		return "a system package"
	case strings.Contains(path, "/nix/store/"):
		return "Nix"
	}
	return ""
}

// upgradeHint is the command that updates a copy we refuse to overwrite.
func upgradeHint(manager string) string {
	switch manager {
	case "Homebrew":
		return "brew upgrade lazyglab"
	case "Nix":
		return "update your Nix inputs and rebuild"
	default:
		return "apt install --only-upgrade lazyglab   (or: sudo dnf upgrade lazyglab)"
	}
}
