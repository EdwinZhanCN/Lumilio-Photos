#!/usr/bin/env bash
# Build the Lumilio Photos Windows portable app directory. Must run inside an
# MSYS2 MINGW64 shell with go, gcc, pkgconf, libvips, libraw and ntldd
# installed (see the desktop-windows CI job / release workflow).
#
# Prerequisites (staged before running):
#   desktop/resources/postgres/17/windows-amd64   PostgreSQL + pgvector (artifact)
#   desktop/resources/ffmpeg/{ffmpeg,ffprobe}.exe fetch-resources.ps1
#   desktop/resources/exiftool/exiftool.exe       fetch-resources.ps1
#   web/dist                                      vp build output
#
# Output: desktop/build/windows/Lumilio Photos/ — zip it for distribution.
#   lumilio-photos.exe + mingw64 DLL closure
#   resources/{postgres,ffmpeg,exiftool,web[,lib/vips-modules-*]}
set -euo pipefail

VERSION="${LUMILIO_VERSION:-0.0.0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESKTOP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT="$(cd "$DESKTOP_DIR/.." && pwd)"
RESOURCES_SRC="$DESKTOP_DIR/resources"

APPDIR="$DESKTOP_DIR/build/windows/Lumilio Photos"
EXE="$APPDIR/lumilio-photos.exe"

echo "==> Cleaning previous build"
rm -rf "$APPDIR"
mkdir -p "$APPDIR"

echo "==> Building Go binary (windows/amd64, CGo via mingw64)"
# -H windowsgui: no console window for the tray app.
( cd "$DESKTOP_DIR" && CGO_ENABLED=1 \
    go build -ldflags "-X server/internal/version.Version=$VERSION -X main.buildVersion=$VERSION -H windowsgui" -o "$EXE" . )

echo "==> Collecting mingw64 DLL closure"
collect_dlls() { # binary destdir
  ntldd -R "$1" | grep -i 'mingw64' | awk '{print $3}' | sort -u | while read -r dll; do
    cp -n "$(cygpath -u "$dll")" "$2/" 2>/dev/null || true
  done
}
collect_dlls "$EXE" "$APPDIR"

echo "==> Staging bundled runtime resources"
mkdir -p "$APPDIR/resources"
stage() { # src dest
  if [ ! -e "$1" ]; then
    echo "    WARNING: missing $1 — bundle will fall back to PATH at runtime" >&2
    return
  fi
  mkdir -p "$(dirname "$2")"
  cp -R "$1" "$2"
}
stage "$RESOURCES_SRC/postgres" "$APPDIR/resources/postgres"
stage "$RESOURCES_SRC/ffmpeg" "$APPDIR/resources/ffmpeg"
stage "$RESOURCES_SRC/exiftool" "$APPDIR/resources/exiftool"
# License texts (committed, also embedded in the binary for the setup window).
stage "$DESKTOP_DIR/licenses" "$APPDIR/resources/licenses"

echo "==> Staging web SPA"
WEB_DIST="$ROOT/web/dist"
if [ -f "$WEB_DIST/index.html" ]; then
  mkdir -p "$APPDIR/resources/web"
  cp -R "$WEB_DIST/." "$APPDIR/resources/web/"
else
  echo "    WARNING: $WEB_DIST missing — app will run API-only" >&2
fi

# libvips dynamic modules (e.g. magick for BMP) live outside the DLL closure;
# stage them plus their own DLL deps when present.
MODDIR="$(ls -d /mingw64/lib/vips-modules-* 2>/dev/null | head -n1 || true)"
if [ -n "$MODDIR" ]; then
  echo "==> Staging libvips modules ($MODDIR)"
  mkdir -p "$APPDIR/resources/lib"
  cp -R "$MODDIR" "$APPDIR/resources/lib/"
  for mod in "$MODDIR"/*.dll; do
    [ -e "$mod" ] && collect_dlls "$mod" "$APPDIR"
  done
fi

echo "==> Built: $APPDIR"
