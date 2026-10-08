#!/usr/bin/env bash
# Repository bootstrap for the Lumilio Photos Cloud Agent environment.
#
# Runs after the repository is checked out. Idempotent: safe to re-run against
# cached or partially prepared state. Durable, source-derived setup only; no
# long-running servers (those live in environment.json "terminals").
set -euo pipefail

# Toolchains provided by the base image (.cursor/Dockerfile).
export PATH="/usr/local/go/bin:${HOME}/go/bin:${HOME}/.local/share/vite-plus/bin:${PATH}"

# Activate the Vite+-managed Node.js runtime and package manager.
if [ -f "${HOME}/.config/vite-plus/env" ]; then
    # shellcheck disable=SC1091
    . "${HOME}/.config/vite-plus/env"
fi

# Installs Go, Web, and docs dependencies; generates the development TOML
# manifest under .local/dev/config/server.toml; installs the Swag CLI and the
# repository commit hook. See CONTRIBUTING.md and taskfile.yml `setup`.
task setup
