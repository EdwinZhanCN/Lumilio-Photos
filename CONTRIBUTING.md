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
`docs/exec-plans/active/` before making changes so your work
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

To exercise the current checkout in the production Linux container shape, run
the standalone development Compose file:

```bash
docker compose -f deploy/compose/dev.compose.yml up -d
```

This builds the current Server and Web sources instead of pulling the published
image. It uses host networking and Docker-managed named volumes for media and
app-state, so it also works when a local IDE controls a remote Linux Docker
daemon.

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
| `task ci:architecture` | Run the architecture, Compose, and lock CI gates |
| `task verify:generated` | Regenerate OpenAPI, types, doc.md, config examples; fail on drift |
| `task ci:site` | Install the documentation site with the lockfile and build it |
| `task ci:desktop:panel` | Install and build the Desktop control-panel frontend |
| `task ci:desktop:native` | Test Server and Desktop modules, then compile Desktop |
| `task dto` | Regenerate OpenAPI, frontend API types, and API documentation |
| `task config:examples` | Regenerate the configuration schema and TOML examples |
| `task lumen:record` | Start the E2E stack with fakelumen recording against a real Hub |
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
[FRONTEND.md](docs/FRONTEND.md) when choosing frontend test
file names and runners. Placement:
[lumilio-write-a-test](.agents/skills/lumilio-write-a-test/SKILL.md).

Workflows call module targets directly. Reproduce a CI failure with the same
name the workflow uses:

```bash
task ci:architecture
task verify:generated
task server:test:ci
task web:install:ci
task web:test
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

Authentication changes also have a fresh-environment first-admin regression;
run it before the seeded hardening suite:

```bash
task web:test:auth-totp
task web:test:auth-hardening
```

The repository also provides narrower root targets:

```bash
task web:test:browser
task web:test:auth-totp
task web:test:auth-hardening
task web:test:video-semantic
task web:test:backup-recovery
```

The matching CI E2E jobs call the same `web:*` targets. Playwright installation
is `task web:playwright:install` and `task web:playwright:install:deps`.

Versioned demo and E2E media comes from the
[`Lumilio-Assets`](https://github.com/EdwinZhanCN/Lumilio-Assets) repository.
The selected revision is pinned in `assets.lock.json`. Sync, seed, and pin
updates: [lumilio-pin-reconcile](.agents/skills/lumilio-pin-reconcile/SKILL.md),
[lumilio-e2e-environment](.agents/skills/lumilio-e2e-environment/SKILL.md).

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
`docs/lumen-catalog.md`.

## Generated Code

Update generated artifacts through their source tools; never hand-edit them:

- After changing backend DTOs or API annotations, run `task dto`
  ([lumilio-api-contract-change](.agents/skills/lumilio-api-contract-change/SKILL.md)).
- After changing configuration profiles, schema comments, or examples, run
  `task config:examples`.
- After changing the SQL schema or queries, run `task server:sqlc`.
- For frontend i18n, follow
  [lumilio-frontend-i18n](.agents/skills/lumilio-frontend-i18n/SKILL.md).
- After editing a feature `doc.ts`, run `task web:docs`.
- `task verify:generated` regenerates the checked-in artifacts above and
  fails if any of them drifted.

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
