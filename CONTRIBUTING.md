# Contributing

Thank you for contributing to Lumilio Photos. This guide is for contributors
who need to modify, test, or build the project locally. Start with the
[English README](README.en.md) and
[user documentation](site/docs/en/user-manual/introduction/installation.md)
for product installation and usage.

## Before You Start

Read [AGENTS.md](AGENTS.md) first. It is the engineering documentation entry
point and identifies the authoritative references for the backend, frontend,
desktop app, test assets, and execution plans. Check
`site/docs/internal/exec-plans/active/` before making changes so your work
does not conflict with an active plan.

The project is local-first: preserve original media, keep media storage and
application state ownership explicit, make AI optional, and prefer simple
configuration that boots and diagnoses cleanly.

## Prerequisites

- Go 1.25+
- [Vite+](https://viteplus.dev/) and its supported Node.js runtime
- [Task](https://taskfile.dev/) (v3.52+; see `version: '3'` in `taskfile.yml`)
- libvips, libraw, FFmpeg, and ExifTool
- Rust when working on Rust components; run `task wasm:setup` only when rebuilding browser WASM packages
- Docker Engine with Compose 2.23.1+ for delivery validation and E2E

Native dependency installation varies by platform. When CGo or media libraries
are involved, prefer the root Task targets; they preserve the build environment
required by the project.

## Setup and Development

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
task setup
task dev
```

`task setup` installs the Go, Web, and documentation dependencies, ensures
the Swag CLI is available, and installs the repository commit hook. Browser
WASM tooling is intentionally opt-in through `task wasm:setup`. It also generates the complete development manifest under
`.local/dev/config/server.toml`.

`task dev` starts:

- Web development server: `http://localhost:6657`
- Go API: `http://localhost:6680`

The browser talks only to the Vite origin; the development server proxies
`/api` to the Go API. SQLite runs inside the Go process and requires no database
container. All runtime artifacts owned by `task dev` live under `.local/dev/`:
app-private state, including the catalog, indexes, logs, secrets, cloud state,
and backups, lives under `state/`; portable development media lives under
`storage/`. Dependency and test-asset caches remain outside this tree and are
not reset with the development instance.

## Common Commands

| Command | Purpose |
| --- | --- |
| `task dev` | Start the Server and Web development processes |
| `task server:dev` | Start only the Go API |
| `task web:dev` | Start only the Web development server |
| `task test` | Run the repository architecture, Server, and Web gates |
| `task server:test` | Run the Go Server tests |
| `task web:test` | Run frontend type, lint, boundary, and unit checks |
| `task desktop:test` | Run the Wails desktop race-test gate |
| `task compose:test` | Validate production and E2E Compose files |
| `task ci:architecture` | Run the architecture and Compose CI gates |
| `task ci:server` | Run the clean-cache Server CI gate |
| `task ci:web` | Install Web dependencies with the lockfile and run Web checks |
| `task ci:site` | Install the documentation site with the lockfile and build it |
| `task ci:desktop:panel` | Install and build the Desktop control-panel frontend |
| `task ci:desktop:native` | Test Server and Desktop modules, then compile Desktop |
| `task dto` | Regenerate OpenAPI, frontend API types, and API documentation |
| `task config:examples` | Regenerate the configuration schema and TOML examples |
| `task dev:clean` | Delete rebuildable development indexes and logs |
| `task dev:reset` | Delete development application state while preserving media and caches |
| `task dev:purge` | Delete the complete development instance, including media (confirms interactively) |

Reset and purge refuse to run while the development Server is listening. They
also require the fixed `.local/dev/.lumilio-dev-root` marker and reject symlink
roots; purge additionally requires the exact confirmation value shown above.

## Testing

Run the quality gates that match the scope of your change:

```bash
task server:test
task web:test
task desktop:test
task compose:test
```

`task test` includes the architecture guards, `server:test`, and `web:test`; it
does not include the desktop app or browser E2E. Follow the “Test layers”
section in
[FRONTEND.md](site/docs/internal/FRONTEND.md) when choosing frontend test
file names and runners, including GPU and WebGL capability tests.

The root `ci:*` targets are the commands used by GitHub Actions. Run them from
the repository root when reproducing a workflow failure:

```bash
task ci:architecture
task ci:server
task ci:web
task ci:site
task ci:desktop:panel
task ci:desktop:native
```

Browser tests use an isolated Compose environment and the Web Taskfile targets:

```bash
task web:e2e:up
task web:test:browser
task web:e2e:down
```

The repository also provides narrower root targets:

```bash
task web:test:browser
task web:test:auth-hardening
task web:test:video-semantic
task web:test:backup-recovery
```

The matching CI E2E contracts are `task ci:web:e2e:browser`,
`task ci:web:e2e:auth-hardening`, `task ci:web:e2e:video-semantic`, and
`task ci:web:e2e:backup-recovery`. Playwright installation is exposed through
`task ci:web:playwright:install` and `task ci:web:playwright:install:deps`.

Versioned demo and E2E media comes from the
[`Lumilio-Assets`](https://github.com/EdwinZhanCN/Lumilio-Assets) repository.
The selected revision is pinned in `assets.lock.json`:

```bash
cd web
vp run assets:sync
vp run assets:sync -- --profile=e2e
```

Assets are hash-verified and stored only in the ignored
`.cache/lumilio-assets/` directory. Lock updates go through the manual
`assets:reconcile` workflow (one PR per trigger); `task assets:check` is the
CI gate that validates the lock against the pinned release.

The Lumen Hub release consumed by Desktop and Compose is pinned in
`lumen.lock.json` (Renovate updates the `release` tag in a dedicated PR;
`task lumen:sync` refreshes the derived fields and regenerates the Desktop
catalog `desktop/internal/lumen/release_catalog.go`, and `task lumen:check`
verifies catalog, `SHA256SUMS`, and consumer builds in CI). See
`site/docs/internal/lumen-catalog.md`.

## Generated Code

Update generated artifacts through their source tools; never hand-edit them:

- After changing backend DTOs or API annotations, run `task dto`.
- After changing configuration profiles, schema comments, or examples, run
  `task config:examples`.
- After changing the SQL schema or queries, run
  `cd server && sqlc generate`.
- For frontend i18n, first write `t("key", "default")` in code, then run
  `cd web && vp exec i18next-cli extract`, and finally fill the Chinese value in
  the generated JSON.

If frontend code needs an `as` cast around an API response, the backend DTO,
`@Success` annotation, or generated type is usually stale. Fix the contract and
run `task dto` instead of casting around it.

## Code and Documentation Conventions

- Format Go code with `gofmt`.
- Follow Vite+ formatting and lint rules for TypeScript, and prefer `@/`
  imports.
- Follow [web/ARCHITECTURE.md](web/ARCHITECTURE.md) for frontend ownership,
  state placement, and cross-feature dependencies.
- A complete schema-versioned TOML manifest is required Server input. Do not add
  implicit configuration search, ordinary environment overrides, or consumer
  fallbacks.
- Do not commit secrets, development databases, test caches, or local media.
- Keep generated `doc.md`, OpenAPI types, and WASM bundles generated.

Use Conventional Commits-style messages:

```text
feat: …
fix: …
refactor: …
chore: …
docs: …
feat(assets): …
```

Before committing, run the checks appropriate to the change and confirm that
generated artifacts have no unintended drift.
