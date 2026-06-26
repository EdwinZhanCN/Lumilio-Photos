# Agent Guide

This file is the short entry point for humans and coding agents working in Lumilio Photos. Keep it compact; longer harness notes live under `docs/agent/` and are excluded from the public VitePress site.

## Overview

- `server/`: Go API service. Entrypoint is `server/cmd/main.go` (thin); the bootstrap lives in `server/app` (`app.Run(ctx)`); application config is `server/config`; business logic lives in `server/internal/*`; migrations live in `server/migrations`.
- `web/`: React 19 + TypeScript frontend on Vite+. Feature code lives under `web/src/features/*`; shared pieces live in `web/src/lib`, `web/src/components`, and `web/src/contexts`.
- `desktop/`: Wails v3 macOS app (separate Go module, `replace server => ../server`). Bundles a private PostgreSQL and runs `server/app` in-process; the React UI is served over HTTP at `localhost:6680`. See `desktop/README.md`.
- `wasm/`: Rust WebAssembly crates for browser-side hashing/export/studio/thumbnail flows; checked-in JS/WASM bundles live under `web/src/wasm`.
- `docs/`: product/user documentation site. Internal harness docs only belong in `docs/agent/`.

The system is local-first: preserve original media, keep repository/storage semantics explicit, make ML/AI optional, and prefer boring configuration that boots cleanly in Docker and local dev.

## Documentation (MUST READ BEFORE ANY CHANGES)

- `docs/agent/architecture.md`: system map, backend/frontend boundaries, config/runtime notes.
- `docs/agent/BACKEND.md`: backend runtime, package map, config, queues, storage, API contracts.
- `docs/agent/FRONTEND.md`: frontend runtime, toolchain, routes, state boundaries, API usage.
- `docs/agent/DESIGN.md`: product and interface guidance for app work.
- `docs/agent/core-beliefs.md`: decision principles for product and engineering tradeoffs.
- `docs/agent/exec-plans/active/`: current execution plans.
- `docs/agent/exec-plans/completed/`: completed execution records.
- `docs/agent/exec-plans/tech-debt-tracker.md`: small known debt that should not be forgotten.
- `docs/agent/vite-plus.md`: frontend Vite+ setup and command mapping.

## Usage Rules

- Use root `make` targets for daily work and validation: `make setup`, `make dev`, `make server-dev`, `make web-dev`, `make test`, `make dto`.
- Desktop-specific targets: `make desktop-dev`, `make desktop-test`, `make desktop-build`.
- If you are in a sandbox without host environment, like cloud/container, use `make setup` to set up the local dev environment. Do not use your own cli tooling if possible.
- Backend quality gate: prefer `make server-test`. Do not bypass it with `cd server && go test ./...` unless there is a concrete reason and you preserve the Makefile environment (notably `CGO_LDFLAGS_ALLOW` / `CGO_CFLAGS_ALLOW` for local media dependencies).
- Frontend quality gate: prefer `make web-test`. Direct `cd web && vp check --no-fmt --no-lint && vp lint && vp test` is acceptable when you are intentionally running only the web gate.
- API contracts are OpenAPI-first. Do not hand-edit `web/src/lib/http-commons/schema.d.ts`; change backend annotations and run `make dto`. An `as`-cast on an API response (`query.data?.data as {...}`) is a red flag: the DTO/`@Success` annotation is missing or `make dto` is stale — fix the contract, never cast around it. If generated `data` is `Record<string, never>` or `unknown` for an endpoint that returns payload data, that is a contract failure and must be fixed in backend DTO/annotation/codegen before frontend work proceeds; do not add compatibility shims for stale DTOs. See [FRONTEND.md](docs/agent/FRONTEND.md) "API Contract".
- Runtime app defaults belong in TOML (`server/config/server*.toml`). Env files are for bootstrap, machine-specific overrides, and secrets.
- Do not commit secrets. `LUMILIO_SECRET_KEY` is a key file path, not raw secret text.
- Go code must be formatted with `gofmt`. TypeScript should follow Vite+ lint/fmt rules and prefer `@/...` imports.
- i18n keys are **extract-then-fill, never hand-written**: write `t("key", "default")` in code → run `vp exec i18next-cli extract` → fill zh values in the generated JSON. Do NOT manually add/restructure/delete keys in `translation.json`. See [FRONTEND.md](docs/agent/FRONTEND.md) for details.
- Frontend server state belongs in TanStack Query; feature-local interactive UI state can use Zustand; Context is for cross-cutting app state.
- Keep generated files generated. If a generated artifact changes, include the command that produced it in your notes.
