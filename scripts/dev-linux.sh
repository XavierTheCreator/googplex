#!/usr/bin/env bash
# Linux dev mode. Needs: go, node, libgtk-3-dev, libwebkit2gtk-4.1-dev (Ubuntu 24.04+ names).
set -euo pipefail
cd "$(dirname "$0")/.."
command -v wails >/dev/null 2>&1 || go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
exec "$(command -v wails || echo "$(go env GOPATH)/bin/wails")" dev -tags webkit2_41
