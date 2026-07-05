#!/usr/bin/env bash
#
# Download the bundled media tools (ffmpeg, ffprobe, exiftool) into
# desktop/resources/ with pinned versions and SHA-256 verification. PostgreSQL is
# NOT fetched here — it must be a relocatable from-source build (see
# .github/workflows/build-postgres.yml and desktop/resources/README.md).
#
# Defaults target Apple Silicon (arm64). For Intel, override the URLs and
# checksums via env, e.g.:
#   FFMPEG_URL=https://www.osxexperts.net/ffmpeg81intel.zip \
#   FFMPEG_SHA256=<intel-binary-sha256> ... desktop/scripts/fetch-resources.sh
#
# Re-running is safe: an already-present, checksum-matching binary is skipped.
set -euo pipefail

# --- Pinned sources (override via env to bump versions / switch arch) ----------
FFMPEG_URL="${FFMPEG_URL:-https://www.osxexperts.net/ffmpeg81arm.zip}"
FFMPEG_SHA256="${FFMPEG_SHA256:-9a08d61f9328e8164ba560ee7a79958e357307fcfeea6fe626b7d66cdc287028}"

FFPROBE_URL="${FFPROBE_URL:-https://www.osxexperts.net/ffprobe81arm.zip}"
FFPROBE_SHA256="${FFPROBE_SHA256:-aab17ac7379c1178aaf400c3ef36cdb67db0b75b1a23eeef2cb9f658be8844e6}"

EXIFTOOL_VERSION="${EXIFTOOL_VERSION:-13.59}"
# exiftool.org no longer serves files directly; official downloads redirect to
# SourceForge (curl -L follows the mirror redirect, the SHA pin still verifies).
EXIFTOOL_URL="${EXIFTOOL_URL:-https://sourceforge.net/projects/exiftool/files/Image-ExifTool-${EXIFTOOL_VERSION}.tar.gz/download}"
EXIFTOOL_SHA256="${EXIFTOOL_SHA256:-668ea3acececb7235fbd0f4900e72d5f12c9b07e5c778fd36cb1e9b5828fd65a}"

# ------------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RESOURCES="$(cd "$SCRIPT_DIR/.." && pwd)/resources"
FFMPEG_DIR="$RESOURCES/ffmpeg"
EXIFTOOL_DIR="$RESOURCES/exiftool"

TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT

sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }

verify() { # file expected
  local got
  got="$(sha256_of "$1")"
  if [ "$got" != "$2" ]; then
    echo "ERROR: checksum mismatch for $(basename "$1")" >&2
    echo "  expected $2" >&2
    echo "  got      $got" >&2
    echo "  (version bumped upstream? update the pinned URL+SHA, or override via env)" >&2
    exit 1
  fi
}

# fetch_static_binary <name> <zip-url> <expected-binary-sha256>
# Downloads a zip whose payload is a single static binary named <name>, verifies
# the extracted binary, and installs it into resources/ffmpeg/.
fetch_static_binary() {
  local name="$1" url="$2" want="$3"
  local dest="$FFMPEG_DIR/$name"
  if [ -f "$dest" ] && [ "$(sha256_of "$dest")" = "$want" ]; then
    echo "  $name: already present and verified — skipping"
    return
  fi
  echo "  $name: downloading $url"
  local work="$TMPROOT/$name"
  mkdir -p "$work"
  curl -fsSL "$url" -o "$work/payload.zip"
  unzip -o -j "$work/payload.zip" -d "$work" >/dev/null
  if [ ! -f "$work/$name" ]; then
    echo "ERROR: '$name' binary not found inside $url" >&2
    exit 1
  fi
  verify "$work/$name" "$want"
  mkdir -p "$FFMPEG_DIR"
  mv -f "$work/$name" "$dest"
  chmod +x "$dest"
  xattr -dr com.apple.quarantine "$dest" 2>/dev/null || true
  echo "  $name: installed → ${dest#"$RESOURCES"/} ($(sha256_of "$dest" | cut -c1-12)…)"
}

fetch_exiftool() {
  if [ -x "$EXIFTOOL_DIR/exiftool" ] && [ -d "$EXIFTOOL_DIR/lib" ]; then
    echo "  exiftool: already present (exiftool + lib/) — skipping"
    return
  fi
  echo "  exiftool: downloading $EXIFTOOL_URL"
  local work="$TMPROOT/exiftool"
  mkdir -p "$work"
  curl -fsSL "$EXIFTOOL_URL" -o "$work/exiftool.tar.gz"
  verify "$work/exiftool.tar.gz" "$EXIFTOOL_SHA256"
  tar xzf "$work/exiftool.tar.gz" -C "$work"
  local src
  src="$(find "$work" -maxdepth 1 -type d -name 'Image-ExifTool-*' | head -1)"
  if [ -z "$src" ] || [ ! -f "$src/exiftool" ] || [ ! -d "$src/lib" ]; then
    echo "ERROR: extracted exiftool tree (exiftool + lib/) not found" >&2
    exit 1
  fi
  mkdir -p "$EXIFTOOL_DIR"
  rm -rf "$EXIFTOOL_DIR/lib" "$EXIFTOOL_DIR/exiftool"
  cp -R "$src/exiftool" "$src/lib" "$EXIFTOOL_DIR/"
  chmod +x "$EXIFTOOL_DIR/exiftool"
  xattr -dr com.apple.quarantine "$EXIFTOOL_DIR" 2>/dev/null || true
  echo "  exiftool: installed → exiftool/{exiftool,lib} (v$EXIFTOOL_VERSION)"
}

echo "==> Fetching bundled media tools into $RESOURCES"
fetch_static_binary ffmpeg "$FFMPEG_URL" "$FFMPEG_SHA256"
fetch_static_binary ffprobe "$FFPROBE_URL" "$FFPROBE_SHA256"
fetch_exiftool

echo "==> Done. PostgreSQL+pgvector is not fetched here — build it from source"
echo "    (.github/workflows/build-postgres.yml) into resources/postgres/17/<platform>/."
