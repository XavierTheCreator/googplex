#!/usr/bin/env bash
# Builds Googleplex for 64-bit Linux: build/bin/googleplex and, where
# dpkg-deb exists (Debian, Ubuntu), a .deb package.
#
# Needs Go 1.26+, Node 20+, and the GTK and WebKitGTK headers:
#   sudo apt install build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev
# (Fedora: sudo dnf install gtk3-devel webkit2gtk4.1-devel)
# webkit2_41 builds against WebKitGTK 4.1, the version current distros ship.
set -euo pipefail
cd "$(dirname "$0")/.."

for tool in go node pkg-config; do
  command -v "$tool" >/dev/null 2>&1 || { echo "Missing $tool"; exit 1; }
done
pkg-config --exists webkit2gtk-4.1 gtk+-3.0 || {
  echo "Missing GTK/WebKitGTK headers: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev"; exit 1; }

command -v wails >/dev/null 2>&1 || go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
WAILS="$(command -v wails || echo "$(go env GOPATH)/bin/wails")"

"$WAILS" build -clean -trimpath -tags webkit2_41 -platform linux/amd64
echo "Built build/bin/googleplex"

command -v dpkg-deb >/dev/null 2>&1 || { echo "No dpkg-deb: skipping the .deb. Run build/bin/googleplex directly."; exit 0; }

VERSION=1.0.0
PKG="$(mktemp -d)/googleplex_${VERSION}_amd64"
trap 'rm -rf "$(dirname "$PKG")"' EXIT
mkdir -p "$PKG/DEBIAN" "$PKG/usr/bin" "$PKG/usr/share/applications" "$PKG/usr/share/icons/hicolor/512x512/apps"
install -m 755 build/bin/googleplex "$PKG/usr/bin/googleplex"
install -m 644 build/appicon.png "$PKG/usr/share/icons/hicolor/512x512/apps/googleplex.png"
cat > "$PKG/usr/share/applications/googleplex.desktop" <<DESKTOP
[Desktop Entry]
Name=Googleplex
Comment=Gmail, Calendar, YouTube, Drive and Tasks in one place
Exec=googleplex
Icon=googleplex
Type=Application
Categories=Office;Network;
DESKTOP
cat > "$PKG/DEBIAN/control" <<CONTROL
Package: googleplex
Version: $VERSION
Architecture: amd64
Maintainer: Googleplex <noreply@localhost>
Depends: libgtk-3-0, libwebkit2gtk-4.1-0
Description: Gmail, Calendar, YouTube, Drive and Tasks in one desktop app
 Talks to Google through your own Google Cloud project. Data stays on your computer.
CONTROL
dpkg-deb --build --root-owner-group "$PKG" "build/bin/googleplex_${VERSION}_amd64.deb" >/dev/null
echo "Built build/bin/googleplex_${VERSION}_amd64.deb"
