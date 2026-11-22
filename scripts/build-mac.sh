#!/usr/bin/env bash
# Builds Googleplex for Apple Silicon Macs: build/bin/Googleplex.app and a
# drag-to-Applications disk image, build/bin/Googleplex-mac-arm64.dmg.
#
# The app is signed ad hoc (no Apple Developer account needed), which is fine
# for your own Mac. On another Mac, Gatekeeper will say it can't check the
# app: right-click it, choose Open, then Open again (once). Notarizing with a
# paid Apple Developer ID removes that step; it's optional for personal use.
set -euo pipefail
cd "$(dirname "$0")/.."

missing=0
xcode-select -p >/dev/null 2>&1 || { echo "Missing Xcode command line tools: run  xcode-select --install"; missing=1; }
command -v go   >/dev/null 2>&1 || { echo "Missing Go 1.26+:  brew install go"; missing=1; }
command -v node >/dev/null 2>&1 || { echo "Missing Node 20+:  brew install node"; missing=1; }
[ "$missing" = 0 ] || exit 1

if ! command -v wails >/dev/null 2>&1 && [ ! -x "$(go env GOPATH)/bin/wails" ]; then
  echo "Installing Wails CLI v2.17.1..."
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
fi
WAILS="$(command -v wails || echo "$(go env GOPATH)/bin/wails")"

"$WAILS" build -clean -trimpath -platform darwin/arm64

APP=build/bin/Googleplex.app
DMG=build/bin/Googleplex-mac-arm64.dmg
codesign --force --deep --sign - "$APP"
codesign --verify --deep "$APP"

# A disk image with the app and an Applications shortcut to drag it onto.
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT
cp -R "$APP" "$STAGE/"
ln -s /Applications "$STAGE/Applications"
rm -f "$DMG"
hdiutil create -volname Googleplex -srcfolder "$STAGE" -ov -format UDZO "$DMG" >/dev/null

echo "Built $APP"
echo "Built $DMG ($(du -h "$DMG" | cut -f1))"
