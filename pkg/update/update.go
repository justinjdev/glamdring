package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Release describes an available update.
type Release struct {
	Version     string
	AssetURL    string
	ChecksumURL string
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// CheckLatest checks the GitHub Releases API for a newer version of glamdring.
// It returns nil if the current version is up to date or if currentVersion is "dev".
func CheckLatest(currentVersion string) (*Release, error) {
	return checkLatest(currentVersion, runtime.GOOS, runtime.GOARCH, "https://api.github.com")
}

func checkLatest(currentVersion, goos, goarch, apiBase string) (*Release, error) {
	if currentVersion == "dev" {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := apiBase + "/repos/justinjdev/glamdring/releases/latest"

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decoding release response: %w", err)
	}

	if !isNewer(rel.TagName, currentVersion) {
		return nil, nil
	}

	versionWithoutV := strings.TrimPrefix(rel.TagName, "v")
	assetName := fmt.Sprintf("glamdring_%s_%s_%s.tar.gz", versionWithoutV, goos, goarch)

	var assetURL, checksumURL string
	for _, a := range rel.Assets {
		switch a.Name {
		case assetName:
			assetURL = a.URL
		case "checksums.txt":
			checksumURL = a.URL
		}
	}

	if assetURL == "" {
		return nil, fmt.Errorf("no matching asset %s in release %s", assetName, rel.TagName)
	}

	return &Release{
		Version:     rel.TagName,
		AssetURL:    assetURL,
		ChecksumURL: checksumURL,
	}, nil
}

func isNewer(latest, current string) bool {
	lMaj, lMin, lPat, lok := parseSemver(latest)
	cMaj, cMin, cPat, cok := parseSemver(current)
	if !lok || !cok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPat > cPat
}

// parseSemver parses a stable semver string (MAJOR.MINOR.PATCH). Prerelease
// versions like "v1.0.0-rc1" are intentionally rejected so that only stable
// releases are offered as updates.
func parseSemver(s string) (major, minor, patch int, ok bool) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// Download downloads the release tarball, verifies its checksum (if available),
// extracts the glamdring binary, and atomically replaces the file at destPath.
func Download(rel *Release, destPath string) error {
	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Get(rel.AssetURL)
	if err != nil {
		return fmt.Errorf("downloading asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading asset: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading asset body: %w", err)
	}

	if rel.ChecksumURL != "" {
		if err := verifyChecksum(client, rel.ChecksumURL, rel.AssetURL, data); err != nil {
			return err
		}
	}

	binary, err := extractBinary(data)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".glamdring-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

func verifyChecksum(client *http.Client, checksumURL, assetURL string, data []byte) error {
	resp, err := client.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading checksums: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading checksums: %w", err)
	}

	assetFilename := filepath.Base(assetURL)
	actual := fmt.Sprintf("%x", sha256.Sum256(data))

	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		if parts[1] == assetFilename {
			if parts[0] != actual {
				return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", assetFilename, parts[0], actual)
			}
			return nil
		}
	}

	return fmt.Errorf("checksum not found for %s in checksums file", assetFilename)
}

func extractBinary(tarball []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, fmt.Errorf("opening gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == "glamdring" {
			content, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading binary from tar: %w", err)
			}
			return content, nil
		}
	}

	return nil, fmt.Errorf("glamdring binary not found in archive")
}
