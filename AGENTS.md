# Agent Guide

This is the compact entry point for coding agents working in Lumilio Photos.
Human setup, commands, and commit conventions live in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Principles

Lumilio Photos is local-first: preserve original media, keep repository and
application-state ownership explicit, make ML/AI optional, and prefer boring
configuration that boots and diagnoses cleanly.

## Repository Map

- `server/`: Go API and embedded SQLite runtime. `server/cmd/main.go` is the thin
  entry point; bootstrap lives in `server/app`, configuration in
  `server/config`, business logic in `server/internal`, and migrations in
  `server/migrations`.
- `web/`: React 19 and TypeScript on Vite+. Feature code lives in
  `web/src/features`; shared runtime code belongs in `web/src/lib`,
  `web/src/components`, or `web/src/contexts`.
- `desktop/`: Wails v3 desktop app for macOS and Windows. It is a separate Go
  module that runs `server/app` and SQLite in-process.
- `wasm/`: Rust WebAssembly crates; checked-in browser bundles live in
  `web/src/wasm`.
- `deploy/`: Linux production Compose files and reverse-proxy examples.
- `site/docs/`: VitePress user documentation under `en/` and `zh-cn/`.
  Engineering notes under `site/docs/internal/` are excluded from the
  VitePress build.

## Documentation Routing

Before substantive changes, read the
[system map](site/docs/internal/architecture.md) and check
[active execution plans](site/docs/internal/exec-plans/active/).
Then read only the references relevant to the change:

- Backend: [BACKEND.md](site/docs/internal/BACKEND.md).
- Frontend: [FRONTEND.md](site/docs/internal/FRONTEND.md) and
  [web/ARCHITECTURE.md](web/ARCHITECTURE.md).
- UI or product behavior: [DESIGN.md](site/docs/internal/DESIGN.md) and
  [core beliefs](site/docs/internal/core-beliefs.md).
- Test or demo media: [test-assets.md](site/docs/internal/test-assets.md).
- Frontend tooling or architecture docs:
  [vite-plus.md](site/docs/internal/vite-plus.md) and
  [docts.md](site/docs/internal/docts.md).
- Known debt: [tech-debt-tracker.md](site/docs/internal/exec-plans/tech-debt-tracker.md).

Completed plans under `site/docs/internal/exec-plans/completed/` are
historical records, not required reading.

## Non-Negotiable Rules

- Prefer root Make targets. Use `make server-test` for backend changes,
  `make web-test` for frontend changes, `make desktop-test` for desktop changes,
  and `make compose-test` for deployment changes. `make test` runs the Server
  and Web gates.
- Follow the frontend “Test layers” taxonomy in
  [FRONTEND.md](site/docs/internal/FRONTEND.md); do not invent test-file
  conventions.
- API contracts are OpenAPI-first. Never hand-edit
  `web/src/lib/http-commons/schema.d.ts` or cast around a stale response type.
  Fix the backend DTO or annotation and run `make dto`.
- The Server requires a complete schema-versioned TOML manifest. Do not add
  code defaults, consumer fallbacks, automatic config search, or ordinary
  environment overrides for runtime-immutable fields.
- Never commit secrets. TOML contains explicit secret-file paths only; secret
  values and secret-path overrides do not belong in environment variables.
- Keep generated files generated and record the command used. Format Go with
  `gofmt`; follow Vite+ fmt/lint for TypeScript.
- Frontend i18n is extract-then-fill: write `t("key", "default")`, run
  `vp exec i18next-cli extract`, then fill the generated Chinese value. Never
  hand-edit translation keys.
- Follow `web/ARCHITECTURE.md` for state ownership, thin routes, workflow
  placement, public entries, and dependency direction.
- Feature documentation uses a root `doc.ts`; every `{@link X}` must have a
  matching `import type`, and the generated sibling `doc.md` is never edited by
  hand.

## Execution Plans

Keep unfinished plans in `site/docs/internal/exec-plans/active/`. Move
completed records to `site/docs/internal/exec-plans/completed/`, retaining
only the goal, final contracts, validation boundaries, and useful decisions.
