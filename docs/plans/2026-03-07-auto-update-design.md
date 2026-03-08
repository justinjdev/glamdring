# Auto-Update Design

## Overview

Add a self-update mechanism with two parts: a non-blocking startup check that notifies users of new versions, and a `/update` command that downloads and replaces the binary.

## Components

### 1. `pkg/update` package

Shared update logic with two main functions:

- `CheckLatest(currentVersion string) (*Release, error)` -- hits GitHub Releases API (`/repos/justinjdev/glamdring/releases/latest`), parses the tag, compares semver. Returns nil if already up to date.
- `Download(release *Release, destPath string) error` -- downloads the platform-appropriate tarball, extracts the binary, verifies checksum against `checksums.txt`, replaces the executable at destPath.

Asset naming follows goreleaser convention: `glamdring_<version>_<os>_<arch>.tar.gz`.

The `Release` struct contains: version string, download URL for the current platform, checksum URL.

### 2. Startup check (non-blocking)

- On startup, fire a goroutine that calls `CheckLatest`.
- If a newer version is found, send an `updateAvailableMsg` through the bubbletea message system.
- Renders as a system message: `Update available: v0.3.0 -- run /update to install`
- Respects `disable_update_check: true` in settings.
- No network delay on startup -- fully async.

### 3. `/update` builtin command

- Calls `CheckLatest` to confirm.
- If already latest: `glamdring v0.2.0 is up to date.`
- Shows version diff and enters a confirmation state: `Update glamdring v0.2.0 -> v0.3.0? [y/n]`
- On `y`: downloads, extracts, verifies checksum, replaces binary in-place.
- On completion: `Updated to v0.3.0. Restart glamdring to use the new version.`

### 4. Config flag

- `disable_update_check` boolean in settings -- skips the startup auto-check.
- `/update` still works manually even when auto-check is disabled.

## Constraints

- No auto-restart.
- No background downloading without user action.
- No update during active agent runs.
- Version "dev" skips the startup check (local development builds).
