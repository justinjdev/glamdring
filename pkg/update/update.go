package update

import (
	"encoding/json"
	"fmt"
	"net/http"
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
