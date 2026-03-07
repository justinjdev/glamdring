# Auto-Update Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add self-update with startup version check notification and `/update` command + `glamdring update` CLI subcommand.

**Architecture:** New `pkg/update` package handles GitHub Releases API calls, checksum verification, and binary replacement. The TUI integrates via an async startup check (tea.Cmd) and a `/update` builtin. The CLI adds a `glamdring update` subcommand that runs the same logic non-interactively. A `disable_update_check` setting suppresses the startup check.

**Tech Stack:** net/http (GitHub API), archive/tar + compress/gzip (extraction), crypto/sha256 (checksum), semver comparison (manual -- tags are simple `vX.Y.Z`).

---

### Task 1: pkg/update -- Release struct and CheckLatest

**Files:**
- Create: `pkg/update/update.go`
- Test: `pkg/update/update_test.go`

**Step 1: Write the failing test**

```go
// pkg/update/update_test.go
package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckLatest_NewerVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/justinjdev/glamdring/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		resp := ghRelease{
			TagName: "v0.3.0",
			Assets: []ghAsset{
				{Name: "glamdring_0.3.0_darwin_arm64.tar.gz", URL: "https://example.com/glamdring_0.3.0_darwin_arm64.tar.gz"},
				{Name: "checksums.txt", URL: "https://example.com/checksums.txt"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rel, err := checkLatest("v0.2.0", "darwin", "arm64", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel == nil {
		t.Fatal("expected release, got nil")
	}
	if rel.Version != "v0.3.0" {
		t.Errorf("version = %q, want v0.3.0", rel.Version)
	}
	if rel.AssetURL == "" {
		t.Error("expected asset URL")
	}
	if rel.ChecksumURL == "" {
		t.Error("expected checksum URL")
	}
}

func TestCheckLatest_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ghRelease{TagName: "v0.2.0"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	rel, err := checkLatest("v0.2.0", "darwin", "arm64", srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Fatalf("expected nil (up to date), got %+v", rel)
	}
}

func TestCheckLatest_DevVersionSkipped(t *testing.T) {
	rel, err := checkLatest("dev", "darwin", "arm64", "http://unused")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Fatalf("expected nil for dev version, got %+v", rel)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/update/ -v -run TestCheckLatest`
Expected: FAIL -- package does not exist

**Step 3: Write minimal implementation**

```go
// pkg/update/update.go
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

const defaultAPIBase = "https://api.github.com"

// Release describes an available update.
type Release struct {
	Version     string
	AssetURL    string
	ChecksumURL string
}

// ghRelease is the GitHub API response shape (subset).
type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// CheckLatest checks GitHub for a newer release than currentVersion.
// Returns nil if already up to date or if currentVersion is "dev".
func CheckLatest(currentVersion string) (*Release, error) {
	return checkLatest(currentVersion, runtime.GOOS, runtime.GOARCH, defaultAPIBase)
}

func checkLatest(currentVersion, goos, goarch, apiBase string) (*Release, error) {
	if currentVersion == "dev" {
		return nil, nil
	}

	url := apiBase + "/repos/justinjdev/glamdring/releases/latest"
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("check latest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: status %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	if !isNewer(rel.TagName, currentVersion) {
		return nil, nil
	}

	// Find the matching asset for this platform.
	ver := strings.TrimPrefix(rel.TagName, "v")
	wantName := fmt.Sprintf("glamdring_%s_%s_%s.tar.gz", ver, goos, goarch)

	var assetURL, checksumURL string
	for _, a := range rel.Assets {
		if a.Name == wantName {
			assetURL = a.URL
		}
		if a.Name == "checksums.txt" {
			checksumURL = a.URL
		}
	}

	if assetURL == "" {
		return nil, fmt.Errorf("no asset found for %s/%s in release %s", goos, goarch, rel.TagName)
	}

	return &Release{
		Version:     rel.TagName,
		AssetURL:    assetURL,
		ChecksumURL: checksumURL,
	}, nil
}

// isNewer returns true if latest is a higher semver than current.
// Both must be in "vX.Y.Z" format.
func isNewer(latest, current string) bool {
	lMaj, lMin, lPatch, lok := parseSemver(latest)
	cMaj, cMin, cPatch, cok := parseSemver(current)
	if !lok || !cok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPatch > cPatch
}

func parseSemver(s string) (major, minor, patch int, ok bool) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var err error
	major, err = strconv.Atoi(parts[0])
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
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/update/ -v -run TestCheckLatest`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/update/update.go pkg/update/update_test.go
git commit -m "feat(update): add CheckLatest with GitHub Releases API"
```

---

### Task 2: Semver comparison tests

**Files:**
- Modify: `pkg/update/update_test.go`

**Step 1: Write the test**

```go
func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.0", "v0.2.0", true},
		{"v0.2.0", "v0.2.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v1.0.0", "v0.9.9", true},
		{"v0.2.1", "v0.2.0", true},
		{"v0.2.0", "v0.2.1", false},
	}
	for _, tt := range tests {
		got := isNewer(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}
```

**Step 2: Run test**

Run: `go test ./pkg/update/ -v -run TestIsNewer`
Expected: PASS (already implemented in Task 1)

**Step 3: Commit**

```bash
git add pkg/update/update_test.go
git commit -m "test(update): add semver comparison tests"
```

---

### Task 3: Download, verify checksum, and replace binary

**Files:**
- Modify: `pkg/update/update.go`
- Modify: `pkg/update/update_test.go`

**Step 1: Write the failing test**

```go
func TestDownloadAndReplace(t *testing.T) {
	// Create a fake binary inside a tar.gz for the test server to serve.
	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho updated")
	tw.WriteHeader(&tar.Header{
		Name: "glamdring",
		Size: int64(len(content)),
		Mode: 0o755,
	})
	tw.Write(content)
	tw.Close()
	gw.Close()

	tarBytes := tarBuf.Bytes()

	// Compute checksum.
	h := sha256.Sum256(tarBytes)
	checksumLine := fmt.Sprintf("%x  glamdring_0.3.0_darwin_arm64.tar.gz\n", h)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset.tar.gz":
			w.Write(tarBytes)
		case "/checksums.txt":
			w.Write([]byte(checksumLine))
		}
	}))
	defer srv.Close()

	// Create a temp file to act as the "current binary".
	tmpFile, err := os.CreateTemp("", "glamdring-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("old binary")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rel := &Release{
		Version:     "v0.3.0",
		AssetURL:    srv.URL + "/asset.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
	}
	if err := Download(rel, tmpFile.Name()); err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	got, _ := os.ReadFile(tmpFile.Name())
	if string(got) != string(content) {
		t.Errorf("binary content = %q, want %q", got, content)
	}
}

func TestDownloadChecksumMismatch(t *testing.T) {
	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{Name: "glamdring", Size: 5, Mode: 0o755})
	tw.Write([]byte("hello"))
	tw.Close()
	gw.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/asset.tar.gz":
			w.Write(tarBuf.Bytes())
		case "/checksums.txt":
			w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  glamdring_0.3.0_darwin_arm64.tar.gz\n"))
		}
	}))
	defer srv.Close()

	tmpFile, _ := os.CreateTemp("", "glamdring-test-*")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	rel := &Release{
		Version:     "v0.3.0",
		AssetURL:    srv.URL + "/asset.tar.gz",
		ChecksumURL: srv.URL + "/checksums.txt",
	}
	err := Download(rel, tmpFile.Name())
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error should mention checksum, got: %v", err)
	}
}
```

Add these imports to the test file: `"archive/tar"`, `"bytes"`, `"compress/gzip"`, `"crypto/sha256"`, `"os"`.

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/update/ -v -run TestDownload`
Expected: FAIL -- Download not defined

**Step 3: Write implementation**

Add to `pkg/update/update.go`:

```go
import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
)

// Download fetches the release tarball, verifies its checksum, extracts the
// binary, and replaces the file at destPath.
func Download(rel *Release, destPath string) error {
	client := &http.Client{Timeout: 5 * time.Minute}

	// Download the tarball.
	resp, err := client.Get(rel.AssetURL)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	defer resp.Body.Close()

	tarball, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read asset: %w", err)
	}

	// Verify checksum if available.
	if rel.ChecksumURL != "" {
		if err := verifyChecksum(client, rel.ChecksumURL, rel.AssetURL, tarball); err != nil {
			return err
		}
	}

	// Extract the "glamdring" binary from the tarball.
	binary, err := extractBinary(tarball)
	if err != nil {
		return err
	}

	// Write to a temp file next to the destination, then rename.
	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, ".glamdring-update-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(binary); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod temp: %w", err)
	}
	tmp.Close()

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

func verifyChecksum(client *http.Client, checksumURL, assetURL string, data []byte) error {
	resp, err := client.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}

	// Parse the checksums.txt file -- format: "<hash>  <filename>"
	assetName := filepath.Base(assetURL)
	var wantHash string
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			wantHash = parts[0]
			break
		}
	}

	if wantHash == "" {
		return fmt.Errorf("checksum not found for %s", assetName)
	}

	gotHash := fmt.Sprintf("%x", sha256.Sum256(data))
	if gotHash != wantHash {
		return fmt.Errorf("checksum mismatch: got %s, want %s", gotHash[:16]+"...", wantHash[:16]+"...")
	}
	return nil
}

func extractBinary(tarball []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tarball))
	if err != nil {
		return nil, fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read: %w", err)
		}
		if filepath.Base(hdr.Name) == "glamdring" && hdr.Typeflag == tar.TypeReg {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read binary from tar: %w", err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("glamdring binary not found in archive")
}
```

Add `"bytes"` to the imports in `update.go`.

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/update/ -v -run TestDownload`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/update/update.go pkg/update/update_test.go
git commit -m "feat(update): add Download with checksum verification"
```

---

### Task 4: disable_update_check setting

**Files:**
- Modify: `pkg/config/settings.go`
- Modify: `pkg/config/settings_test.go`

**Step 1: Write the failing test**

Add to `pkg/config/settings_test.go`:

```go
func TestDisableUpdateCheck(t *testing.T) {
	s := Settings{DisableUpdateCheck: true}
	if !s.DisableUpdateCheck {
		t.Error("expected DisableUpdateCheck to be true")
	}

	// Verify merge behavior.
	base := DefaultSettings()
	override := Settings{DisableUpdateCheck: true}
	mergeSettings(&base, &override)
	if !base.DisableUpdateCheck {
		t.Error("expected merge to set DisableUpdateCheck")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/config/ -v -run TestDisableUpdateCheck`
Expected: FAIL -- DisableUpdateCheck not a field

**Step 3: Add the field**

Add to `Settings` struct in `pkg/config/settings.go`:

```go
DisableUpdateCheck bool `json:"disable_update_check,omitempty"`
```

Add to `mergeSettings` in `pkg/config/settings.go`:

```go
if override.DisableUpdateCheck {
	base.DisableUpdateCheck = true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./pkg/config/ -v -run TestDisableUpdateCheck`
Expected: PASS

**Step 5: Commit**

```bash
git add pkg/config/settings.go pkg/config/settings_test.go
git commit -m "feat(config): add disable_update_check setting"
```

---

### Task 5: Startup update check in TUI

**Files:**
- Modify: `internal/tui/model.go`

**Step 1: Add the updateAvailableMsg type and version field**

Add to `Model` struct:

```go
// version is the compiled-in version string, used for update checks.
version string
```

Add a setter:

```go
func (m *Model) SetVersion(v string) {
	m.version = v
}
```

Add the message type:

```go
// updateAvailableMsg signals that a newer version is available.
type updateAvailableMsg struct {
	version string
}
```

**Step 2: Add the startup check command**

Add a new method:

```go
// checkUpdateCmd returns a tea.Cmd that checks for updates asynchronously.
func (m Model) checkUpdateCmd() tea.Cmd {
	version := m.version
	return func() tea.Msg {
		rel, err := update.CheckLatest(version)
		if err != nil || rel == nil {
			return nil
		}
		return updateAvailableMsg{version: rel.Version}
	}
}
```

Add `"github.com/justin/glamdring/pkg/update"` to imports.

**Step 3: Wire it into Init**

In `Init()`, add `m.checkUpdateCmd()` to the `tea.Batch` call (alongside existing commands). Only fire it if `m.disableUpdateCheck` is false.

Add field to Model:

```go
disableUpdateCheck bool
```

Add setter:

```go
func (m *Model) SetDisableUpdateCheck(v bool) {
	m.disableUpdateCheck = v
}
```

Update `Init()`:

```go
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.input.Init(),
		m.output.Init(),
		m.startupCmd(),
		m.spinner.Tick,
	}
	if !m.disableUpdateCheck {
		cmds = append(cmds, m.checkUpdateCmd())
	}
	return tea.Batch(cmds...)
}
```

**Step 4: Handle the message in Update**

Add a case in the `Update` switch:

```go
case updateAvailableMsg:
	m.output.AppendSystem(fmt.Sprintf("Update available: %s -- run /update to install", msg.version))
	return m, nil
```

**Step 5: Wire version and setting in main.go**

In `cmd/glamdring/main.go`, after creating the model:

```go
m.SetVersion(version)
m.SetDisableUpdateCheck(settings.DisableUpdateCheck)
```

**Step 6: Commit**

```bash
git add internal/tui/model.go cmd/glamdring/main.go
git commit -m "feat(tui): add async startup update check"
```

---

### Task 6: /update builtin command

**Files:**
- Modify: `internal/tui/builtins.go`
- Modify: `internal/tui/model.go`

**Step 1: Add /update to the builtin registry**

In `builtins.go`, add to `builtinCommands`:

```go
"update": cmdUpdate,
```

Add to `builtinDescriptions`:

```go
"update": "Check for and install updates",
```

**Step 2: Implement cmdUpdate**

This command needs a confirmation flow. Use a new `StateUpdate` state with a pending release.

Add to `model.go`:

```go
StateUpdate  // waiting for update confirmation
```

Add field:

```go
pendingUpdate *update.Release
```

Implement `cmdUpdate` in `builtins.go`:

```go
func cmdUpdate(m *Model, args string) tea.Cmd {
	m.output.AppendSystem("Checking for updates...")

	rel, err := update.CheckLatest(m.version)
	if err != nil {
		m.output.AppendError(fmt.Sprintf("Update check failed: %s", err))
		return nil
	}
	if rel == nil {
		m.output.AppendSystem(fmt.Sprintf("glamdring %s is up to date.", m.version))
		return nil
	}

	m.pendingUpdate = rel
	m.state = StateUpdate
	m.output.AppendSystem(fmt.Sprintf("Update glamdring %s -> %s?", m.version, rel.Version))
	return nil
}
```

**Step 3: Add StateUpdate key handler**

In `handleKeyMsg`, add a case for `StateUpdate` in the state switch:

```go
case StateUpdate:
	return m.handleUpdateKey(msg)
```

Implement `handleUpdateKey`:

```go
func (m Model) handleUpdateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		rel := m.pendingUpdate
		m.pendingUpdate = nil
		m.state = StateInput
		m.output.AppendSystem("Downloading update...")

		return m, func() tea.Msg {
			exe, err := os.Executable()
			if err != nil {
				return updateDoneMsg{err: err}
			}
			exe, err = filepath.EvalSymlinks(exe)
			if err != nil {
				return updateDoneMsg{err: err}
			}
			if err := update.Download(rel, exe); err != nil {
				return updateDoneMsg{err: err}
			}
			return updateDoneMsg{version: rel.Version}
		}

	case "n", "N":
		m.pendingUpdate = nil
		m.state = StateInput
		m.output.AppendSystem("Update cancelled.")
		return m, m.input.Focus()
	}
	return m, nil
}
```

Add the done message type:

```go
type updateDoneMsg struct {
	version string
	err     error
}
```

Handle it in `Update`:

```go
case updateDoneMsg:
	if msg.err != nil {
		m.output.AppendError(fmt.Sprintf("Update failed: %s", msg.err))
	} else {
		m.output.AppendSystem(fmt.Sprintf("Updated to %s. Restart glamdring to use the new version.", msg.version))
	}
	m.state = StateInput
	return m, m.input.Focus()
```

**Step 4: Add confirmation prompt rendering**

In `View()`, add a case for `StateUpdate`:

```go
case StateUpdate:
	input = m.renderUpdatePrompt()
```

Implement:

```go
func (m Model) renderUpdatePrompt() string {
	title := m.styles.PermissionTitle.Render("Install update?")
	help := m.styles.PermissionHelp.Render("[y]es  [n]o")
	content := title + "\n" + help
	return m.styles.PermissionBorder.Width(m.width - 4).Render(content)
}
```

**Step 5: Commit**

```bash
git add internal/tui/builtins.go internal/tui/model.go
git commit -m "feat(tui): add /update command with confirmation"
```

---

### Task 7: glamdring update CLI subcommand

**Files:**
- Modify: `cmd/glamdring/main.go`

**Step 1: Add the subcommand**

In the `switch os.Args[1]` block in `main()`, add:

```go
case "update":
	runUpdate(version)
	return
```

**Step 2: Implement runUpdate**

```go
func runUpdate(currentVersion string) {
	fmt.Println("Checking for updates...")
	rel, err := update.CheckLatest(currentVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if rel == nil {
		fmt.Printf("glamdring %s is up to date.\n", currentVersion)
		return
	}

	fmt.Printf("Update available: %s -> %s\n", currentVersion, rel.Version)
	fmt.Print("Install? [y/N] ")

	var answer string
	fmt.Scanln(&answer)
	if answer != "y" && answer != "Y" {
		fmt.Println("Update cancelled.")
		return
	}

	fmt.Println("Downloading...")
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := update.Download(rel, exe); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated to %s. Restart glamdring to use the new version.\n", rel.Version)
}
```

Add `"github.com/justin/glamdring/pkg/update"` to imports.

**Step 3: Run build**

Run: `go build ./cmd/glamdring/`
Expected: compiles successfully

**Step 4: Commit**

```bash
git add cmd/glamdring/main.go
git commit -m "feat(cli): add glamdring update subcommand"
```

---

### Task 8: Update README

**Files:**
- Modify: `README.md`

**Step 1: Add update docs**

Add a section documenting:
- `glamdring update` -- check for and install updates from the command line
- `/update` -- check for and install updates from inside the TUI
- `disable_update_check: true` in settings to suppress startup notifications

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add update command documentation"
```

---

### Task 9: Run full test suite

**Step 1: Run all tests**

Run: `go test ./...`
Expected: all PASS

**Step 2: Run build**

Run: `go build ./cmd/glamdring/`
Expected: compiles
