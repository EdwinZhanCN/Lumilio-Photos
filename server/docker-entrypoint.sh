#!/bin/sh
set -eu

ensure_app_writable() {
    target=$1
    label=$2
    mode=${3:-}

    if ! mkdir -p "$target"; then
        echo "lumilio: cannot create $label directory at $target" >&2
        exit 1
    fi
    if ! gosu app test -w "$target"; then
        chown app:app "$target" 2>/dev/null || true
    fi
    if ! gosu app test -w "$target"; then
        echo "lumilio: $label directory is not writable by container uid 10001: $target" >&2
        echo "lumilio: fix the bind-mount owner/permissions; the active SQLite catalog must stay in app-state" >&2
        exit 1
    fi
    if [ -n "$mode" ] && ! chmod "$mode" "$target"; then
        echo "lumilio: cannot set required permissions $mode on $label directory: $target" >&2
        exit 1
    fi
}

ensure_app_writable /data/storage "media storage"
ensure_app_writable /data/app-state "application state" 0700

exec gosu app "$@"
