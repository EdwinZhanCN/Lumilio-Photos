# Agent Guide

This file is the short entry point for humans and coding agents working in Lumilio Photos. Keep it compact; longer harness notes live under `docs/agent/` and are excluded from the public VitePress site.

## Overview

- `server/`: Go API service. Startup is `server/cmd/main.go`; application config is `server/config`; business logic lives in `server/internal/*`; migrations live in `server/migrations`.
- `web/`: React 19 + TypeScript frontend on Vite+. Feature code lives under `web/src/features/*`; shared pieces live in `web/src/lib`, `web/src/components`, and `web/src/contexts`.
- `wasm/`: Rust WebAssembly crates used by the web and plugin flows.
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

- Use root `make` targets for daily work: `make setup`, `make dev`, `make server-dev`, `make web-dev`, `make test`, `make dto`.
- Backend quality gate: `make server-test` or `cd server && go test ./...`.
- Frontend quality gate: `make web-test` or `cd web && vp check --no-fmt --no-lint && vp lint && vp test`.
- API contracts are OpenAPI-first. Do not hand-edit `web/src/lib/http-commons/schema.d.ts`; change backend annotations and run `make dto`.
- Runtime app defaults belong in TOML (`server/config/server*.toml`). Env files are for bootstrap, machine-specific overrides, and secrets.
- Do not commit secrets. `LUMILIO_SECRET_KEY` is a key file path, not raw secret text.
- Go code must be formatted with `gofmt`. TypeScript should follow Vite+ lint/fmt rules and prefer `@/...` imports.
- Frontend server state belongs in TanStack Query; feature-local interactive UI state can use Zustand; Context is for cross-cutting app state.
- Keep generated files generated. If a generated artifact changes, include the command that produced it in your notes.
