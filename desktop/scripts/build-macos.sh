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
#   desktop/resources/postgres/18/<darwin-arch>/bin   PostgreSQL + pgvector (+ contrib pg_trgm)
#   desktop/resources/ffmpeg/{ffmpeg,ffprobe}         static ffmpeg build
#   desktop/resources/exiftool/exiftool               exiftool standalone
# Build tools:
#   brew install dylibbundler vips
#
# Usage:
#   desktop/scripts/build-macos.sh [arm64|amd64] [--dmg]
set -euo pipefail

# Homebrew's libraw_r.pc emits `-Xpreprocessor`, which Go's cgo flag allowlist
# rejects. Allow it so the libraw binding (server/internal/utils/raw) builds.
export CGO_LDFLAGS_ALLOW="-Xpreprocessor"
export CGO_CFLAGS_ALLOW="-Xpreprocessor"

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
APP_ICON_NAME="AppIcon"
APP_ICON="$DESKTOP_DIR/packaging/icons/$APP_ICON_NAME.icns"

BUILD_DIR="$DESKTOP_DIR/build"
APP="$BUILD_DIR/$APP_NAME.app"
MACOS_DIR="$APP/Contents/MacOS"
RES_DIR="$APP/Contents/Resources"
FRAMEWORKS_DIR="$APP/Contents/Frameworks"
EXE="$MACOS_DIR/lumilio-photos"

echo "==> Cleaning previous bundle"
rm -rf "$APP"
mkdir -p "$MACOS_DIR" "$RES_DIR" "$FRAMEWORKS_DIR"

echo "==> Ensuring control panel bundle (desktop/panel/dist, embedded via go:embed)"
PANEL_DIST="$DESKTOP_DIR/panel/dist"
# CI stages a prebuilt panel-dist artifact and sets LUMILIO_PANEL_DIST_PREBUILT=1;
# local builds always rebuild so a stale dist is never embedded.
if [ "${LUMILIO_PANEL_DIST_PREBUILT:-}" = "1" ] && [ -f "$PANEL_DIST/index.html" ]; then
  echo "    using prebuilt $PANEL_DIST"
else
  if ! command -v vp >/dev/null 2>&1; then
    echo "    ERROR: vp not found; install Vite+ tooling (the Go binary embeds panel/dist)." >&2
    exit 1
  fi
  ( cd "$DESKTOP_DIR/panel" && vp install && vp run build )
fi

echo "==> Building Go binary ($PLATFORM)"
# Add an LC_RPATH pointing at the bundled Frameworks dir so the libvips tree
# (whose install names dylibbundler rewrites to @rpath) resolves at runtime. On
# macOS the rpath must go through the external linker (clang) — the Go linker's
# own -r flag is ELF-only and rejects the @executable_path token.
( cd "$DESKTOP_DIR" && CGO_ENABLED=1 GOOS=darwin GOARCH="$GOARCH" \
    go build -ldflags "-X server/internal/version.Version=$VERSION -X main.buildVersion=$VERSION -extldflags=-Wl,-rpath,@executable_path/../Frameworks" -o "$EXE" . )

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
  <key>CFBundleIconFile</key><string>$APP_ICON_NAME</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>LSUIElement</key><true/>
  <key>NSLocalNetworkUsageDescription</key>
  <string>Lumilio Photos discovers Lumen ML servers on your local network via mDNS to enable optional AI features (semantic search, face recognition, OCR).</string>
  <key>NSBonjourServices</key>
  <array>
    <string>_lumen._tcp</string>
  </array>
</dict>
</plist>
PLIST

echo "==> Staging app icon"
if [ ! -f "$APP_ICON" ]; then
  echo "    ERROR: missing $APP_ICON" >&2
  exit 1
fi
cp "$APP_ICON" "$RES_DIR/$APP_ICON_NAME.icns"

echo "==> Staging bundled runtime resources"
PG_SRC="$RESOURCES_SRC/postgres/18/$PLATFORM"
for tool in postgres initdb pg_ctl pg_isready createdb pg_dump pg_restore psql; do
  if [ ! -x "$PG_SRC/bin/$tool" ]; then
    echo "    ERROR: missing required PostgreSQL tool: $PG_SRC/bin/$tool" >&2
    echo "    Build/download the postgres-$PLATFORM artifact before packaging." >&2
    exit 1
  fi
done
PG_EXTENSION_DIR="$PG_SRC/share/extension"
if [ -d "$PG_SRC/share/postgresql/extension" ]; then
  PG_EXTENSION_DIR="$PG_SRC/share/postgresql/extension"
fi
for extension in vector pg_trgm; do
  if [ ! -f "$PG_EXTENSION_DIR/$extension.control" ]; then
    echo "    ERROR: missing required PostgreSQL extension: $extension" >&2
    exit 1
  fi
done
stage() { # src dest
  if [ ! -e "$1" ]; then
    echo "    WARNING: missing $1 — bundle will fall back to PATH at runtime" >&2
    return
  fi
  mkdir -p "$(dirname "$2")"
  cp -R "$1" "$2"
}
PG_BUNDLE_DIR="$RES_DIR/postgres/18/$PLATFORM"
stage "$PG_SRC"                                "$PG_BUNDLE_DIR"
# Artifacts produced before the relocation gate may still contain the GitHub
# runner's absolute pg-dist prefix. Rewrite the staged copy so local packaging
# remains safe without mutating the downloaded resource cache.
"$SCRIPT_DIR/relocate-postgres.sh" "$PG_BUNDLE_DIR"
stage "$RESOURCES_SRC/ffmpeg/ffmpeg"          "$RES_DIR/ffmpeg/ffmpeg"
stage "$RESOURCES_SRC/ffmpeg/ffprobe"         "$RES_DIR/ffmpeg/ffprobe"
stage "$RESOURCES_SRC/exiftool"               "$RES_DIR/exiftool"
# License texts (committed, also embedded in the binary for the setup window).
stage "$DESKTOP_DIR/licenses"                 "$RES_DIR/licenses"

echo "==> Staging web SPA"
WEB_DIST="$ROOT/web/dist"
# CI stages a prebuilt web/dist artifact and sets LUMILIO_WEB_DIST_PREBUILT=1;
# local builds always rebuild so a stale dist is never bundled.
if [ "${LUMILIO_WEB_DIST_PREBUILT:-}" = "1" ] && [ -f "$WEB_DIST/index.html" ]; then
  echo "    using prebuilt $WEB_DIST"
else
  if ! command -v vp >/dev/null 2>&1; then
    echo "    ERROR: vp not found; install Vite+ tooling or run make setup before desktop-build." >&2
    exit 1
  fi
  echo "    building web frontend (vp build)"
  ( cd "$ROOT/web" && vp build )
fi
if [ -d "$WEB_DIST" ]; then
  mkdir -p "$RES_DIR/web"
  cp -R "$WEB_DIST/." "$RES_DIR/web/"
else
  echo "    ERROR: $WEB_DIST not found after vp build." >&2
  exit 1
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

echo "==> Staging libvips dynamic modules"
find_vips_modules_dir() {
  local prefix dir
  if command -v pkg-config >/dev/null 2>&1; then
    prefix="$(pkg-config --variable=prefix vips 2>/dev/null || true)"
    if [ -n "$prefix" ]; then
      for dir in "$prefix"/lib/vips-modules-*; do
        if [ -d "$dir" ]; then
          printf '%s\n' "$dir"
          return 0
        fi
      done
    fi
  fi
  if command -v brew >/dev/null 2>&1; then
    prefix="$(brew --prefix vips 2>/dev/null || true)"
    if [ -n "$prefix" ]; then
      for dir in "$prefix"/lib/vips-modules-*; do
        if [ -d "$dir" ]; then
          printf '%s\n' "$dir"
          return 0
        fi
      done
    fi
  fi
  return 1
}
VIPS_MODULES_SRC="$(find_vips_modules_dir || true)"
if [ -n "$VIPS_MODULES_SRC" ]; then
  VIPS_MODULES_DEST="$RES_DIR/lib/$(basename "$VIPS_MODULES_SRC")"
  mkdir -p "$VIPS_MODULES_DEST"
  # Stage only the dynamic libvips modules we actually need:
  #   vips-heif.dylib   -> HEIC/HEIF/AVIF load (iPhone photos)
  #   vips-magick.dylib -> BMP and other long-tail raster formats libvips has no
  #                        native loader for (decoded via ImageMagick).
  # Poppler/OpenSlide are intentionally skipped. The dylib-closure pass below
  # walks $RES_DIR/lib too, so each staged module's dependency tree (e.g.
  # libMagickCore) is pulled into Frameworks/ and its load paths rewritten.
  for mod in vips-heif vips-magick; do
    if [ -f "$VIPS_MODULES_SRC/$mod.dylib" ]; then
      cp "$VIPS_MODULES_SRC/$mod.dylib" "$VIPS_MODULES_DEST/$mod.dylib"
    else
      echo "    WARNING: $mod.dylib not found in $VIPS_MODULES_SRC" >&2
    fi
  done
else
  echo "    WARNING: libvips modules not found; HEIC/AVIF/BMP plugin loaders may be unavailable" >&2
fi

echo "==> Completing bundled dylib closure"
# dylibbundler can rewrite a dependency to the bundle path without copying every
# indirect dylib. Walk the resulting Frameworks tree and pull in any missing
# Homebrew dylibs by basename, then rewrite absolute Homebrew references to
# loader-relative paths. This is intentionally conservative: system dylibs stay
# system dylibs.
BREW_PREFIXES=()
if command -v brew >/dev/null 2>&1; then
  BREW_PREFIXES+=("$(brew --prefix)")
fi
BREW_PREFIXES+=("/opt/homebrew" "/usr/local")

find_homebrew_dylib() { # basename
  local name="$1" prefix candidate
  for prefix in "${BREW_PREFIXES[@]}"; do
    [ -n "$prefix" ] || continue
    for candidate in "$prefix/lib/$name" "$prefix"/Cellar/*/*/lib/"$name"; do
      if [ -f "$candidate" ]; then
        printf '%s\n' "$candidate"
        return 0
      fi
    done
  done
  return 1
}

ensure_framework_dylib() { # basename
  local name="$1" src dest
  dest="$FRAMEWORKS_DIR/$name"
  [ -f "$dest" ] && return 1
  src="$(find_homebrew_dylib "$name" || true)"
  if [ -z "$src" ]; then
    echo "    WARNING: could not find Homebrew dylib $name" >&2
    return 1
  fi
  echo "    adding $name"
  cp -L "$src" "$dest"
  chmod 755 "$dest" 2>/dev/null || true
  install_name_tool -id "@rpath/$name" "$dest" 2>/dev/null || true
  return 0
}

rel_ref_for() { # file basename
  case "$1" in
    "$EXE")
      printf '@rpath/%s\n' "$2"
      return
      ;;
  esac
  local dir sub prefix
  dir="$(dirname "$1")"
  case "$dir" in
    "$RES_DIR"/lib/vips-modules-*)
      printf '@loader_path/../../../Frameworks/%s\n' "$2"
      return
      ;;
  esac
  prefix="@loader_path"
  if [ "$dir" != "$FRAMEWORKS_DIR" ]; then
    sub="${dir#$FRAMEWORKS_DIR/}"
    while [ -n "$sub" ] && [ "$sub" != "$dir" ]; do
      prefix="$prefix/.."
      case "$sub" in
        */*) sub="${sub%/*}" ;;
        *) sub="" ;;
      esac
    done
  fi
  printf '%s/%s\n' "$prefix" "$2"
}

delete_bundle_rpaths() { # file
  local f="$1" rpath
  for rpath in "@executable_path/../Frameworks/" "@executable_path/../Frameworks"; do
    while otool -l "$f" | grep -F "path $rpath " >/dev/null 2>&1; do
      install_name_tool -delete_rpath "$rpath" "$f" 2>/dev/null || break
    done
  done
}

closure_targets() {
  printf '%s\n' "$EXE"
  if [ -d "$FRAMEWORKS_DIR" ]; then
    find "$FRAMEWORKS_DIR" -type f -name "*.dylib"
  fi
  if [ -d "$RES_DIR/lib" ]; then
    find "$RES_DIR/lib" -type f -name "*.dylib"
  fi
}

rewrite_one_dylib_refs() { # file
  local f="$1" ref base newref changed=1
  case "$f" in
    *.dylib)
      delete_bundle_rpaths "$f"
      install_name_tool -id "@rpath/$(basename "$f")" "$f" 2>/dev/null || true
      ;;
  esac
  while IFS= read -r ref; do
    base="$(basename "$ref")"
    case "$ref" in
      /opt/homebrew/*|/usr/local/*)
        ensure_framework_dylib "$base" && changed=0
        newref="$(rel_ref_for "$f" "$base")"
        if install_name_tool -change "$ref" "$newref" "$f" 2>/dev/null; then
          changed=0
        fi
        ;;
      @executable_path/../Frameworks/*.dylib|@rpath/*.dylib|@loader_path/*.dylib)
        if [ ! -f "$FRAMEWORKS_DIR/$base" ]; then
          ensure_framework_dylib "$base" && changed=0
        fi
        newref="$(rel_ref_for "$f" "$base")"
        if [ "$ref" != "$newref" ] && install_name_tool -change "$ref" "$newref" "$f" 2>/dev/null; then
          changed=0
        fi
        ;;
    esac
  done < <(otool -L "$f" | awk 'NR>1 {print $1}')
  return "$changed"
}

for _ in 1 2 3 4 5 6 7 8; do
  changed="false"
  while IFS= read -r dylib; do
    rewrite_one_dylib_refs "$dylib" && changed="true"
  done < <(closure_targets)
  [ "$changed" = "false" ] && break
done

missing_dylibs="false"
while IFS= read -r f; do
  while IFS= read -r ref; do
    case "$ref" in
      @executable_path/../Frameworks/*.dylib|@rpath/*.dylib|@loader_path/*.dylib|@loader_path/*/*.dylib|@loader_path/*/*/*.dylib|@loader_path/*/*/*/*.dylib)
        base="$(basename "$ref")"
        if [ ! -f "$FRAMEWORKS_DIR/$base" ]; then
          echo "    ERROR: missing bundled dylib $base referenced by $f" >&2
          missing_dylibs="true"
        fi
        ;;
    esac
  done < <(otool -L "$f" | awk 'NR>1 {print $1}')
done < <(closure_targets)
if [ "$missing_dylibs" = "true" ]; then
  exit 1
fi

echo "==> Ad-hoc signing (free, no Apple Developer account)"
# install_name rewrites invalidate signatures, so re-sign every dylib first.
while IFS= read -r dylib; do
  [ "$dylib" = "$EXE" ] && continue
  chmod 755 "$dylib" 2>/dev/null || true
  codesign --force -s - "$dylib" 2>/dev/null || true
done < <(closure_targets)
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
