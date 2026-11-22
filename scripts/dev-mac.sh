#!/usr/bin/env bash
# Checks the tools Googleplex needs on macOS, installs the Wails CLI, then starts dev mode.
set -euo pipefail
cd "$(dirname "$0")/.."

missing=0
xcode-select -p >/dev/null 2>&1 || { echo "Missing Xcode command line tools: run  xcode-select --install"; missing=1; }
command -v go   >/dev/null 2>&1 || { echo "Missing Go 1.23+:  brew install go"; missing=1; }
command -v node >/dev/null 2>&1 || { echo "Missing Node 20+:  brew install node"; missing=1; }
[ "$missing" = 0 ] || exit 1

if ! command -v wails >/dev/null 2>&1 && [ ! -x "$(go env GOPATH)/bin/wails" ]; then
  echo "Installing Wails CLI v2.17.1..."
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
fi
WAILS="$(command -v wails || echo "$(go env GOPATH)/bin/wails")"

[ -f .env ] || echo "No .env found. That's fine: the app's setup screen can take your client JSON instead."
exec "$WAILS" dev
