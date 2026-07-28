#!/bin/sh

set -eu

command_name=${1:-}
repository_input=${2:-}
marker_magic=lumilio-dev-root-v1

fail() {
	printf 'dev-state: %s\n' "$*" >&2
	exit 1
}

usage() {
	printf 'usage: %s <init|clean|reset|purge> <repository-root>\n' "$0" >&2
	exit 2
}

[ -n "$command_name" ] && [ -n "$repository_input" ] || usage
[ "$#" -eq 2 ] || usage

repository_root=$(CDPATH= cd -- "$repository_input" 2>/dev/null && pwd -P) ||
	fail "repository root does not exist: $repository_input"
local_root=$repository_root/.local
dev_root=$local_root/dev
marker=$dev_root/.lumilio-dev-root
state_root=$dev_root/state
storage_root=$dev_root/storage

exists() {
	[ -e "$1" ] || [ -L "$1" ]
}

reject_symlink() {
	if [ -L "$1" ]; then
		fail "refusing symlink path: $1"
	fi
}

assert_repository() {
	[ -f "$repository_root/Makefile" ] ||
		fail "not a Lumilio Photos repository: $repository_root"
	[ -d "$repository_root/server/config" ] ||
		fail "missing server/config under $repository_root"
	[ -d "$repository_root/web" ] ||
		fail "missing web under $repository_root"
	reject_symlink "$local_root"
	reject_symlink "$dev_root"
}

verify_marker() {
	[ -f "$marker" ] && [ ! -L "$marker" ] ||
		fail "missing regular development marker: $marker"
	actual_marker=$(cat "$marker")
	[ "$actual_marker" = "$marker_magic" ] ||
		fail "unexpected development marker content in $marker"
}

ensure_root() {
	assert_repository
	umask 077
	if ! exists "$local_root"; then
		mkdir "$local_root"
	elif [ ! -d "$local_root" ]; then
		fail "development parent is not a directory: $local_root"
	fi
	if exists "$dev_root"; then
		[ -d "$dev_root" ] ||
			fail "development root is not a directory: $dev_root"
		verify_marker
		return
	fi
	mkdir "$dev_root"
	printf '%s\n' "$marker_magic" >"$marker"
	verify_marker
}

prepare_runtime_directories() {
	for directory in "$dev_root/config" "$state_root" "$storage_root"; do
		reject_symlink "$directory"
	done
	umask 077
	mkdir -p "$dev_root/config" "$state_root" "$storage_root"
	chmod 700 "$dev_root/config" "$state_root" "$storage_root"
}

assert_server_stopped() {
	command -v curl >/dev/null 2>&1 ||
		fail "curl is required to prove the development server is stopped"
	if curl --noproxy '*' --silent --max-time 1 --output /dev/null \
		http://127.0.0.1:6680/api/v1/health/ready
	then
		fail "the development server is still listening on 127.0.0.1:6680"
	fi
}

describe_target() {
	target=$1
	if exists "$target"; then
		size=$(du -sh "$target" 2>/dev/null | awk '{print $1}')
		printf '  %s (%s)\n' "$target" "${size:-unknown size}"
	fi
}

remove_dev_child() {
	target=$1
	case "$target" in
		"$dev_root"/*) ;;
		*) fail "refusing path outside development root: $target" ;;
	esac
	reject_symlink "$target"
	if exists "$target"; then
		rm -rf "$target"
	fi
}

init_environment() {
	ensure_root
	prepare_runtime_directories
	printf 'Development root: %s\n' "$dev_root"
}

clean_environment() {
	ensure_root
	assert_server_stopped
	reject_symlink "$state_root"
	printf 'Removing rebuildable development outputs:\n'
	describe_target "$state_root/indexes"
	describe_target "$state_root/logs"
	remove_dev_child "$state_root/indexes"
	remove_dev_child "$state_root/logs"
}

reset_environment() {
	ensure_root
	assert_server_stopped
	printf 'Removing development application state; media storage is preserved:\n'
	describe_target "$state_root"
	remove_dev_child "$state_root"
}

purge_environment() {
	assert_repository
	if [ ! -d "$dev_root" ]; then
		printf 'No unified development environment exists at %s\n' "$dev_root"
		return
	fi
	verify_marker
	assert_server_stopped
	[ "${CONFIRM_DEV_PURGE:-}" = "dev-purge" ] ||
		fail "set CONFIRM=dev-purge when invoking make dev-purge"
	printf 'Removing the complete development environment, including media:\n'
	describe_target "$dev_root"
	reject_symlink "$state_root"
	reject_symlink "$storage_root"
	rm -rf "$dev_root"
	rmdir "$local_root" 2>/dev/null || true
}

case "$command_name" in
	init) init_environment ;;
	clean) clean_environment ;;
	reset) reset_environment ;;
	purge) purge_environment ;;
	*) usage ;;
esac
