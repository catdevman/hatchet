package browser

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ShellVersion is the pinned chrome-headless-shell release, bumped
// deliberately alongside the axe pin (see scripts/update-pins.sh).
const ShellVersion = "131.0.6778.204"

// shellChecksums holds the expected sha256 of each platform's zip for
// ShellVersion. Downloads verify against these when present; platforms
// without an entry install with a warning (HLD OQ5). Populated by
// scripts/update-pins.sh.
var shellChecksums = map[string]string{
	"linux64": "afaac86e302c4874245991a3d509d529c50ecdd447affff0cdc2b08520906c32",
}

const cftURL = "https://storage.googleapis.com/chrome-for-testing-public/%s/%s/chrome-headless-shell-%s.zip"

func shellPlatform() (string, error) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "linux64", nil
	case "darwin/amd64":
		return "mac-x64", nil
	case "darwin/arm64":
		return "mac-arm64", nil
	case "windows/amd64":
		return "win64", nil
	default:
		return "", fmt.Errorf("no chrome-headless-shell build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func installDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "hatchet"), nil
}

// InstalledShell returns the managed headless shell's path if present.
func InstalledShell() (string, bool) {
	platform, err := shellPlatform()
	if err != nil {
		return "", false
	}
	dir, err := installDir()
	if err != nil {
		return "", false
	}
	bin := shellBinary(dir, platform)
	if _, err := os.Stat(bin); err != nil {
		return "", false
	}
	return bin, true
}

func shellBinary(dir, platform string) string {
	name := "chrome-headless-shell"
	if strings.HasPrefix(platform, "win") {
		name += ".exe"
	}
	return filepath.Join(dir, ShellVersion, "chrome-headless-shell-"+platform, name)
}

// Install downloads and unpacks the pinned headless shell into the user
// cache, returning the binary path. Safe to re-run; an existing install is
// reused.
func Install(ctx context.Context, logf func(string, ...any)) (string, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if bin, ok := InstalledShell(); ok {
		logf("chrome-headless-shell %s already installed at %s", ShellVersion, bin)
		return bin, nil
	}

	platform, err := shellPlatform()
	if err != nil {
		return "", err
	}
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	versionDir := filepath.Join(dir, ShellVersion)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return "", err
	}

	url := fmt.Sprintf(cftURL, ShellVersion, platform, platform)
	logf("downloading %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: %s", url, resp.Status)
	}

	// Zip needs random access; buffer via temp file, hashing as it streams.
	tmp, err := os.CreateTemp(dir, "shell-*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	hash := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body)
	if err != nil {
		return "", err
	}
	logf("downloaded %d MB, extracting", size/(1<<20))

	if want := shellChecksums[platform]; want != "" {
		if got := hex.EncodeToString(hash.Sum(nil)); got != want {
			return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, want)
		}
	} else {
		logf("warning: no pinned checksum for platform %s; download not verified", platform)
	}

	if err := unzip(tmp.Name(), versionDir); err != nil {
		return "", err
	}
	bin := shellBinary(dir, platform)
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("archive did not contain expected binary %s", bin)
	}
	logf("installed chrome-headless-shell %s at %s", ShellVersion, bin)
	return bin, nil
}

func unzip(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		path := filepath.Join(dest, f.Name)
		// Reject entries escaping the destination (zip-slip).
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("archive entry %q escapes destination", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	return nil
}
