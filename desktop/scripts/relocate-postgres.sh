#!/usr/bin/env bash
#
# Make a from-source PostgreSQL tree relocatable on macOS.
#
# PostgreSQL's build bakes the build-machine --prefix into the install names of
# its client tools (initdb/createdb/pg_isready/pg_dump → libpq.5.dylib) and into
# the dylib ids themselves. When the tree is copied to another machine (or into
# the .app bundle) those absolute paths no longer exist and dyld fails with
# "Library not loaded: <build-prefix>/lib/libpq.5.dylib".
#
# This rewrites every absolute reference to a bundled lib into a load-relative
# path (@executable_path/../lib for bin tools, @loader_path[/..] for dylibs) and
# ad-hoc re-signs (install_name_tool invalidates signatures; arm64 requires a
# valid one). Idempotent.
#
# Usage: relocate-postgres.sh <postgres platform dir containing bin/ and lib/>
#   e.g. relocate-postgres.sh desktop/resources/postgres/18/darwin-arm64
set -euo pipefail

ROOT="$(cd "${1:?usage: relocate-postgres.sh <dir with bin/ and lib/>}" && pwd)"
[ -d "$ROOT/bin" ] && [ -d "$ROOT/lib" ] || {
  echo "ERROR: expected bin/ and lib/ under $ROOT" >&2
  exit 1
}

have_lib() { find "$ROOT/lib" -name "$1" -print -quit | grep -q .; }

# rel_prefix <file>: the load-relative prefix that points at <ROOT>/lib.
rel_prefix() {
  case "$1" in
    "$ROOT"/bin/*) echo "@executable_path/../lib" ;;
    "$ROOT"/lib/*/*) echo "@loader_path/.." ;; # e.g. lib/postgresql/<ext>.dylib
    *) echo "@loader_path" ;;                   # lib/<name>.dylib
  esac
}

fix_one() {
  local f="$1" prefix ref base
  prefix="$(rel_prefix "$f")"
  case "$f" in
    *.dylib) install_name_tool -id "@rpath/$(basename "$f")" "$f" 2>/dev/null || true ;;
  esac
  while IFS= read -r ref; do
    case "$ref" in
      /*/lib/*.dylib)
        base="$(basename "$ref")"
        if have_lib "$base"; then
          install_name_tool -change "$ref" "$prefix/$base" "$f" 2>/dev/null || true
        fi
        ;;
    esac
  done < <(otool -L "$f" | awk 'NR>1 {print $1}')
}

echo "==> Rewriting install names under $ROOT"
for f in "$ROOT"/bin/*; do
  [ -f "$f" ] && fix_one "$f"
done
while IFS= read -r f; do
  fix_one "$f"
done < <(find "$ROOT/lib" -type f -name '*.dylib')

echo "==> Ad-hoc re-signing"
{ find "$ROOT/lib" -type f -name '*.dylib'; find "$ROOT/bin" -type f; } |
  while IFS= read -r f; do codesign --force -s - "$f" 2>/dev/null || true; done

echo "==> Done. Verify with: $ROOT/bin/initdb --version"
