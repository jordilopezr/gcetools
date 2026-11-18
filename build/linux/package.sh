#!/usr/bin/env bash
# Builds the Linux .deb, .rpm and AppImage from already compiled binaries.
#   build/linux/package.sh <version> <dir-with-gcetools-and-gcetools-cli> <out-dir>
# Needs nfpm and appimagetool on PATH. Run from the repository root.
#
# The AppImage bundles only gceTools itself: GTK 3 and WebKitGTK 4.1 come
# from the host, the same requirement as the .tar.gz (present by default on
# GNOME desktops of Ubuntu 22.04+, Debian 12+, Fedora 36+).
set -euo pipefail
version=$1 src=$(realpath "$2") out=$(realpath -m "$3")
mkdir -p "$out"

# nfpm does not expand variables in `contents`, so substitute them here.
cfg=$(mktemp --suffix=.yaml)
sed -e "s|\${VERSION}|$version|g" -e "s|\${SRC}|$src|g" -e "s|\${ARCH}|amd64|g" build/linux/nfpm.yaml > "$cfg"
nfpm package -f "$cfg" -p deb -t "$out/gcetools-$version-linux-amd64.deb"
nfpm package -f "$cfg" -p rpm -t "$out/gcetools-$version-linux-x86_64.rpm"
rm -f "$cfg"

appdir=$(mktemp -d)/gceTools.AppDir
trap 'rm -rf "$(dirname "$appdir")"' EXIT
install -Dm755 "$src/gcetools" "$appdir/usr/bin/gcetools"
install -Dm755 "$src/gcetools-cli" "$appdir/usr/bin/gcetools-cli"
install -Dm644 build/linux/gcetools.desktop "$appdir/usr/share/applications/gcetools.desktop"
install -Dm644 assets/brand/linux/gcetools-256.png "$appdir/usr/share/icons/hicolor/256x256/apps/gcetools.png"
install -Dm644 assets/brand/linux/gcetools.svg "$appdir/usr/share/icons/hicolor/scalable/apps/gcetools.svg"
cp build/linux/gcetools.desktop "$appdir/gcetools.desktop"
cp assets/brand/linux/gcetools-256.png "$appdir/gcetools.png"
ln -s gcetools.png "$appdir/.DirIcon"
cat > "$appdir/AppRun" <<'RUN'
#!/bin/sh
# `gceTools.AppImage scan ...` runs the console scan, like the plain binary.
here="$(dirname "$(readlink -f "$0")")"
exec "$here/usr/bin/gcetools" "$@"
RUN
chmod +x "$appdir/AppRun"
ARCH=x86_64 VERSION=$version appimagetool --no-appstream "$appdir" "$out/gcetools-$version-linux-x86_64.AppImage"
ls -la "$out"
