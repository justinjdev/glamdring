#!/bin/bash
# Render a single theme preview to stdout.
# Usage: ./preview.sh <theme-name>
#
# For VHS screenshots: vhs docs/screenshots/themes.tape
# For PNG generation:  pipe through ansisvg + rsvg-convert
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
GLAMDRING_PREVIEW=1 go test -run "TestThemePreview/$1\$" -v ./internal/tui/ 2>/dev/null \
  | grep -v '=== RUN\|--- PASS\|--- SKIP\|--- FAIL\|^PASS\|^ok '
