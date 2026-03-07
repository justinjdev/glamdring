#!/bin/bash
# Build and capture theme screenshots using the real TUI via VHS.
# Usage: ./docs/screenshots/preview.sh
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

echo "Building glamdring..."
go build -o /tmp/glamdring-demo ./cmd/glamdring

echo "Running VHS tape..."
vhs docs/screenshots/themes.tape

echo "Screenshots saved to docs/screenshots/"
ls -l docs/screenshots/theme-*.png
