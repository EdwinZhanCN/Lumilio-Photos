# Vite+ local setup and cleanup

Lumilio Photos uses the Vite+ alpha toolchain for the frontend in `web/`. Vite+ provides the `vp` command, delegates package installs through the package manager declared by the project, and replaces the day-to-day Vite/Vitest/Oxlint commands.

> Vite+ is still alpha software. If a `vp` command fails after an upstream alpha release, run `vp upgrade`, refresh the project dependencies with `vp install`, and check the Vite+ troubleshooting guide before changing application code.

## One-time install on an existing nvm/pnpm machine

From a shell that can reach the Vite+ installer and npm registry:

```bash
# 1. Keep your existing nvm installation, but use the project Node first.
cd /path/to/Lumilio-Photos/web
nvm install
nvm use

# 2. Make sure pnpm is available for Vite+ package-manager delegation.
corepack enable
corepack prepare pnpm@11.2.2 --activate

# 3. Install or update the global Vite+ CLI.
curl -fsSL https://vite.plus | bash
exec "$SHELL" -l
vp help

# 4. Install the frontend dependencies through Vite+.
vp install

# 5. Validate the migrated frontend.
vp check
vp test
vp build
```

## Cleanup after migrating from direct nvm/pnpm workflows

Run this once if your checkout has stale Vite/Vitest artifacts from the old toolchain:

```bash
cd /path/to/Lumilio-Photos/web

# Remove installed packages and generated frontend output.
rm -rf node_modules dist coverage .vite .vitest

# Optional: prune pnpm's global content-addressable store.
pnpm store prune

# Optional: clear Vite+ caches if an alpha upgrade behaves oddly.
rm -rf ~/.vite-plus/cache

# Rehydrate with the Vite+ wrapper.
vp install
```

You do **not** need to uninstall nvm or pnpm. Vite+ can manage Node versions itself, but keeping nvm installed is harmless for developers who also work on non-Vite+ projects. Prefer entering this repo with `nvm use` (or `vp env`, once you choose to let Vite+ own Node switching globally) and then run frontend commands through `vp`.

## Command mapping

| Old command | New command |
| --- | --- |
| `pnpm install` | `vp install` |
| `pnpm dev` / `vite` | `vp dev` |
| `pnpm build` / `vite build` | `vp build` |
| `pnpm preview` / `vite preview` | `vp preview` |
| `pnpm lint` / `oxlint` | `vp lint` |
| `pnpm lint:fix` / `oxlint --fix` | `vp lint --fix` |
| `pnpm type-check` | `vp check --no-fmt --no-lint` |
| `pnpm test` / `vitest` | `vp test` |
| `pnpm test:watch` / `vitest --watch` | `vp test watch` |
| `pnpm test:coverage` / `vitest run --coverage` | `vp test run --coverage` |

For full-stack local development from the repository root, keep using:

```bash
make setup
make dev
```

`make setup` now installs frontend dependencies through `vp install`, and `make web-dev` starts the frontend with `vp dev --host --port 6657`.
