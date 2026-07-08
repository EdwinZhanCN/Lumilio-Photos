# Agent Guide

This file is the short entry point for humans and coding agents working in Lumilio Photos. Keep it compact; longer notes live under `site/docs/internal/` (built by VitePress but kept out of nav/sidebar and the search index, so internal-only).

## Overview

- `server/`: Go API service. Entrypoint is `server/cmd/main.go` (thin); the bootstrap lives in `server/app` (`app.Run(ctx)`); application config is `server/config`; business logic lives in `server/internal/*`; migrations live in `server/migrations`.
- `web/`: React 19 + TypeScript frontend on Vite+. Feature code lives under `web/src/features/*`; shared pieces live in `web/src/lib`, `web/src/components`, and `web/src/contexts`.
- `desktop/`: Wails v3 macOS app (separate Go module, `replace server => ../server`). Bundles a private PostgreSQL and runs `server/app` in-process; the React UI is served over HTTP at `localhost:6680`. See `desktop/README.md`.
- `wasm/`: Rust WebAssembly crates for browser-side hashing/export/studio/thumbnail flows; checked-in JS/WASM bundles live under `web/src/wasm`.
- `site/docs/`: VitePress docs site. Public product/user docs under `en/` and `zh-cn/`; internal engineering/harness docs live under `site/docs/internal/` (`internal/agent/` = harness notes, `internal/frontend/` = frontend architecture) — built but excluded from nav/sidebar/search.

The system is local-first: preserve original media, keep repository/storage semantics explicit, make ML/AI optional, and prefer boring configuration that boots cleanly in Docker and local dev.

## Documentation

### MUST READ BEFORE ANY CHANGES

- `site/docs/internal/agent/architecture.md`: system map, backend/frontend boundaries, config/runtime notes.
- `site/docs/internal/agent/BACKEND.md`: backend runtime, package map, config, queues, storage, API contracts.
- `site/docs/internal/agent/FRONTEND.md`: frontend runtime, toolchain, routes, state boundaries, API usage.
- `site/docs/internal/agent/DESIGN.md`: product and interface guidance for app work.
- `site/docs/internal/agent/core-beliefs.md`: decision principles for product and engineering tradeoffs.
- `site/docs/internal/agent/exec-plans/tech-debt-tracker.md`: small known debt that should not be forgotten.
- `site/docs/internal/agent/vite-plus.md`: frontend Vite+ setup and command mapping.
- `site/docs/internal/agent/docts.md`: the `doc.ts` architecture-doc convention (`@module` + `{@link}` backed by `import type`) and the `docts/link-needs-import` lint rule.

### TRACK THE CHANGES

- `site/docs/internal/agent/exec-plans/active/`: current execution plans.
- `site/docs/internal/agent/exec-plans/completed/`: completed execution records.

### READ BEFORE YOU MAKE ANY PLANS

- `site/docs/internal/agent/PLAN-MODE.md`: plan mode description


## Usage Rules

- Use root `make` targets for daily work and validation: `make setup`, `make dev`, `make server-dev`, `make web-dev`, `make test`, `make dto`.
- Desktop-specific targets: `make desktop-dev`, `make desktop-test`, `make desktop-build`.
- If you are in a sandbox without host environment, like cloud/container, use `make setup` to set up the local dev environment. Do not use your own cli tooling if possible.
- Backend quality gate: prefer `make server-test`. Do not bypass it with `cd server && go test ./...` unless there is a concrete reason and you preserve the Makefile environment (notably `CGO_LDFLAGS_ALLOW` / `CGO_CFLAGS_ALLOW` for local media dependencies).
- Frontend quality gate: prefer `make web-test`. Direct `cd web && vp check --no-fmt --no-lint && vp lint && vp test` is acceptable when you are intentionally running only the web gate.
- API contracts are OpenAPI-first. Do not hand-edit `web/src/lib/http-commons/schema.d.ts`; change backend annotations and run `make dto`. An `as`-cast on an API response (`query.data?.data as {...}`) is a red flag: the DTO/`@Success` annotation is missing or `make dto` is stale — fix the contract, never cast around it. If generated `data` is `Record<string, never>` or `unknown` for an endpoint that returns payload data, that is a contract failure and must be fixed in backend DTO/annotation/codegen before frontend work proceeds; do not add compatibility shims for stale DTOs. See [FRONTEND.md](site/docs/internal/agent/FRONTEND.md) "API Contract".
- Runtime app defaults belong in TOML (`server/config/server*.toml`). Env files are for bootstrap, machine-specific overrides, and secrets.
- Do not commit secrets. `LUMILIO_SECRET_KEY` is a key file path, not raw secret text.
- Go code must be formatted with `gofmt`. TypeScript should follow Vite+ lint/fmt rules and prefer `@/...` imports.
- `vp fmt` writes files by default. Generated/vendored frontend artifacts must stay excluded through `web/vite.config.ts` `fmt.ignorePatterns` (notably `src/wasm/**`, `src/features/*/doc.md`, and generated OpenAPI/client code).
- i18n keys are **extract-then-fill, never hand-written**: write `t("key", "default")` in code → run `vp exec i18next-cli extract` → fill zh values in the generated JSON. Do NOT manually add/restructure/delete keys in `translation.json`. See [FRONTEND.md](site/docs/internal/agent/FRONTEND.md) for details.
- Frontend server state belongs in TanStack Query; feature-local interactive UI state can use Zustand; Context is for cross-cutting app state.
- Keep generated files generated. If a generated artifact changes, include the command that produced it in your notes.
- Document a feature with a `doc.ts` at its root (`@module` comment, markdown prose, `{@link}` to real symbols). Every `{@link X}` MUST be `import type`-d in the same file — tsc plus the `docts/link-needs-import` rule enforce it. The sibling `doc.md` is generated; never hand-edit it. See [docts.md](site/docs/internal/agent/docts.md).
- Commit messages convention, use this pattern: feat: …, fix: …., chore: …, refactor: …. and docs: … etc.If something domain specific and worth noting, then use pattern like: feat(assets): …
