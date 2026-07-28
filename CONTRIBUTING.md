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
- Make
- libvips, libraw, FFmpeg, and ExifTool
- Rust and `wasm-pack` when rebuilding browser WASM packages
- Docker Engine with Compose 2.23.1+ for delivery validation and E2E

Native dependency installation varies by platform. When CGo or media libraries
are involved, prefer the root Make targets; they preserve the build environment
required by the project.

## Setup and Development

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
make setup
make dev
```

`make setup` installs the Go, Web, and documentation dependencies, ensures
`wasm-pack` and the Swag CLI are available, and installs the repository commit
hook. It also generates the complete development manifest under
`.local/dev/config/server.toml`.

`make dev` starts:

- Web development server: `http://localhost:6657`
- Go API: `http://localhost:6680`

The browser talks only to the Vite origin; the development server proxies
`/api` to the Go API. SQLite runs inside the Go process and requires no database
container. All runtime artifacts owned by `make dev` live under `.local/dev/`:
app-private state, including the catalog, indexes, logs, secrets, cloud state,
and backups, lives under `state/`; portable development media lives under
`storage/`. Dependency and test-asset caches remain outside this tree and are
not reset with the development instance.

## Common Commands

| Command | Purpose |
| --- | --- |
| `make dev` | Start the Server and Web development processes |
| `make server-dev` | Start only the Go API |
| `make web-dev` | Start only the Web development server |
| `make test` | Run the main Server and Web quality gates |
| `make server-test` | Run architecture guards and Go Server tests |
| `make web-test` | Run frontend type, lint, boundary, and unit checks |
| `make desktop-test` | Build the desktop panel and run desktop tests |
| `make compose-test` | Validate production and E2E Compose files |
| `make dto` | Regenerate OpenAPI, frontend API types, and API documentation |
| `make config-examples` | Regenerate the configuration schema and TOML examples |
| `make dev-clean` | Delete rebuildable development indexes and logs |
| `make dev-reset` | Delete development application state while preserving media and caches |
| `make dev-purge CONFIRM=dev-purge` | Delete the complete development instance, including media |

Reset and purge refuse to run while the development Server is listening. They
also require the fixed `.local/dev/.lumilio-dev-root` marker and reject symlink
roots; purge additionally requires the exact confirmation value shown above.

## Testing

Run the quality gates that match the scope of your change:

```bash
make server-test
make web-test
make desktop-test
make compose-test
```

`make test` includes only `server-test` and `web-test`; it does not include the
desktop app or browser E2E. Follow the “Test layers” section in
[FRONTEND.md](site/docs/internal/FRONTEND.md) when choosing frontend test
file names and runners, including GPU and WebGL capability tests.

Browser tests use an isolated Compose environment:

```bash
cd web
vp run e2e:up
vp run e2e:seed
vp run e2e:test --grep @smoke --workers=1
vp run e2e:down
```

The repository also provides narrower root targets:

```bash
make web-browser-test
make web-auth-hardening-test
make web-video-semantic-test
make web-backup-recovery-test
```

Versioned demo and E2E media comes from the
[`Lumilio-Assets`](https://github.com/EdwinZhanCN/Lumilio-Assets) repository.
The selected revision is pinned in `assets.lock.json`:

```bash
cd web
vp run assets:sync
vp run assets:sync -- --profile=e2e
```

Assets are hash-verified and stored only in the ignored
`.cache/lumilio-assets/` directory.

## Generated Code

Update generated artifacts through their source tools; never hand-edit them:

- After changing backend DTOs or API annotations, run `make dto`.
- After changing configuration profiles, schema comments, or examples, run
  `make config-examples`.
- After changing the SQL schema or queries, run
  `cd server && sqlc generate`.
- For frontend i18n, first write `t("key", "default")` in code, then run
  `cd web && vp exec i18next-cli extract`, and finally fill the Chinese value in
  the generated JSON.

If frontend code needs an `as` cast around an API response, the backend DTO,
`@Success` annotation, or generated type is usually stale. Fix the contract and
run `make dto` instead of casting around it.

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
