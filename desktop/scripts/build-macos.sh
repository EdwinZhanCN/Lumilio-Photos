#!/usr/bin/env bash
#
# Build the Lumilio Photos macOS .app bundle (and optionally a DMG).
#
# This deliberately does NOT use `wails3 build` because the UI is served by the
# in-process Go API server (not Wails' asset server — a WebAuthn requirement),
# so the bundle is just: Go binary + bundled native runtime (PostgreSQL, ffmpeg,
# exiftool) + the libvips dylib tree. Everything is ad-hoc signed (free, no Apple
# Developer account) per the exec plan's signing strategy.
#
# Prerequisites (staged before running — see desktop/resources/README.md):
#   desktop/resources/postgres/16/<darwin-arch>/bin   PostgreSQL + pgvector
#   desktop/resources/ffmpeg/{ffmpeg,ffprobe}         static ffmpeg build
#   desktop/resources/exiftool/exiftool               exiftool standalone
# Build tools:
#   brew install dylibbundler vips
#
# Usage:
#   desktop/scripts/build-macos.sh [arm64|amd64] [--dmg]
set -euo pipefail

ARCH="${1:-arm64}"
MAKE_DMG="false"
for arg in "$@"; do
  [ "$arg" = "--dmg" ] && MAKE_DMG="true"
done

case "$ARCH" in
  arm64) GOARCH="arm64" ;;
  amd64) GOARCH="amd64" ;;
  *) echo "unknown arch: $ARCH (use arm64 or amd64)" >&2; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DESKTOP_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ROOT="$(cd "$DESKTOP_DIR/.." && pwd)"
RESOURCES_SRC="$DESKTOP_DIR/resources"
PLATFORM="darwin-$GOARCH"

APP_NAME="Lumilio Photos"
BUNDLE_ID="com.edwinzhan.lumilio-photos"
VERSION="${LUMILIO_VERSION:-0.0.0}"

BUILD_DIR="$DESKTOP_DIR/build"
APP="$BUILD_DIR/$APP_NAME.app"
MACOS_DIR="$APP/Contents/MacOS"
RES_DIR="$APP/Contents/Resources"
FRAMEWORKS_DIR="$APP/Contents/Frameworks"
EXE="$MACOS_DIR/lumilio-photos"

echo "==> Cleaning previous bundle"
rm -rf "$APP"
mkdir -p "$MACOS_DIR" "$RES_DIR" "$FRAMEWORKS_DIR"

echo "==> Building Go binary ($PLATFORM)"
# Add an LC_RPATH pointing at the bundled Frameworks dir so the libvips tree
# (whose install names dylibbundler rewrites to @rpath) resolves at runtime. On
# macOS the rpath must go through the external linker (clang) — the Go linker's
# own -r flag is ELF-only and rejects the @executable_path token.
( cd "$DESKTOP_DIR" && CGO_ENABLED=1 GOOS=darwin GOARCH="$GOARCH" \
    go build -ldflags "-extldflags=-Wl,-rpath,@executable_path/../Frameworks" -o "$EXE" . )

echo "==> Writing Info.plist"
cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>$APP_NAME</string>
  <key>CFBundleDisplayName</key><string>$APP_NAME</string>
  <key>CFBundleExecutable</key><string>lumilio-photos</string>
  <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

echo "==> Staging bundled runtime resources"
stage() { # src dest
  if [ ! -e "$1" ]; then
    echo "    WARNING: missing $1 — bundle will fall back to PATH at runtime" >&2
    return
  fi
  mkdir -p "$(dirname "$2")"
  cp -R "$1" "$2"
}
stage "$RESOURCES_SRC/postgres/16/$PLATFORM" "$RES_DIR/postgres/16/$PLATFORM"
stage "$RESOURCES_SRC/ffmpeg/ffmpeg"          "$RES_DIR/ffmpeg/ffmpeg"
stage "$RESOURCES_SRC/ffmpeg/ffprobe"         "$RES_DIR/ffmpeg/ffprobe"
stage "$RESOURCES_SRC/exiftool"               "$RES_DIR/exiftool"

echo "==> Staging web SPA"
WEB_DIST="$ROOT/web/dist"
if [ ! -d "$WEB_DIST" ] && command -v vp >/dev/null 2>&1; then
  echo "    building web frontend (vp build)"
  ( cd "$ROOT/web" && vp build )
fi
if [ -d "$WEB_DIST" ]; then
  mkdir -p "$RES_DIR/web"
  cp -R "$WEB_DIST/." "$RES_DIR/web/"
else
  echo "    WARNING: $WEB_DIST not found; app will run API-only (no UI). Run 'cd web && vp build'." >&2
fi

echo "==> Bundling libvips dylib tree (dylibbundler)"
if command -v dylibbundler >/dev/null 2>&1; then
  dylibbundler -od -b \
    -x "$EXE" \
    -d "$FRAMEWORKS_DIR/" \
    -p "@executable_path/../Frameworks/"
else
  echo "    WARNING: dylibbundler not installed; skipping (run: brew install dylibbundler)" >&2
fi

echo "==> Ad-hoc signing (free, no Apple Developer account)"
# install_name rewrites invalidate signatures, so re-sign every dylib first.
if [ -d "$FRAMEWORKS_DIR" ]; then
  find "$FRAMEWORKS_DIR" -type f -name "*.dylib" -exec codesign --force -s - {} \; 2>/dev/null || true
fi
# Sign bundled executables too, then the whole bundle.
find "$RES_DIR" -type f -perm +111 -exec codesign --force -s - {} \; 2>/dev/null || true
codesign --force --deep -s - "$APP"

echo "==> Built: $APP"

# make_plain_dmg writes a basic compressed DMG that still contains an
# Applications symlink (the drag-drop target), used when create-dmg is missing or
# its Finder styling fails (e.g. headless). Uses $DMG / $DMG_SRC / $APP_NAME.
make_plain_dmg() {
  ln -sf /Applications "$DMG_SRC/Applications"
  hdiutil create -volname "$APP_NAME" -srcfolder "$DMG_SRC" -ov -format UDZO "$DMG"
  echo "==> Built (plain): $DMG"
}

if [ "$MAKE_DMG" = "true" ]; then
  DMG="$BUILD_DIR/Lumilio-Photos-$GOARCH.dmg"
  DMG_SRC="$BUILD_DIR/dmg-src"
  DMG_BG="$DESKTOP_DIR/packaging/dmg/background.png" # optional artwork (660x400)
  echo "==> Creating DMG: $DMG"
  rm -rf "$DMG" "$DMG_SRC"
  mkdir -p "$DMG_SRC"
  cp -R "$APP" "$DMG_SRC/"

  if command -v create-dmg >/dev/null 2>&1; then
    # create-dmg builds the classic "drag to Applications" window: it adds the
    # Applications symlink (--app-drop-link) and positions the icons / window /
    # background via Finder. The window styling needs a GUI session (a local Mac
    # or a CI macOS runner); if it fails we fall back to a plain DMG below.
    dmg_args=(
      --volname "$APP_NAME"
      --window-pos 200 120
      --window-size 660 400
      --icon-size 120
      --icon "$APP_NAME.app" 165 200
      --hide-extension "$APP_NAME.app"
      --app-drop-link 495 200
      --no-internet-enable
    )
    if [ -f "$DMG_BG" ]; then
      dmg_args+=(--background "$DMG_BG")
    else
      echo "    note: no $DMG_BG — DMG will use positioned icons without arrow artwork" >&2
    fi
    if create-dmg "${dmg_args[@]}" "$DMG" "$DMG_SRC"; then
      echo "==> Built: $DMG"
    else
      echo "    create-dmg failed (no GUI session?); writing a plain DMG with an Applications link" >&2
      make_plain_dmg
    fi
  else
    echo "    create-dmg not installed (brew install create-dmg) — plain DMG with Applications link" >&2
    make_plain_dmg
  fi
fi
