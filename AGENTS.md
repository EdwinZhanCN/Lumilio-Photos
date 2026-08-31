# Agent Guide

This is the compact entry point for coding agents working in Lumilio Photos.
Human setup, commands, and commit conventions live in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Principles

Lumilio Photos is local-first: preserve original media, keep repository and
application-state ownership explicit, make ML/AI optional, and prefer boring
configuration that boots and diagnoses cleanly.

## Branch And Pull Request Routing

- `dev` is the default integration branch. Create ordinary feature, fix,
  refactor, documentation, and agent-harness branches from the latest `dev`,
  and open every Draft or implementation PR with `dev` as its base.
- `main` is the stable/release branch. Changes normally reach it through an
  intentional `dev` → `main` promotion PR. Target `main` directly only when
  the Issue or a human explicitly requests that exception.
- GitHub's default branch is not a target-selection signal. Verify the base
  explicitly before creating or updating a PR; never infer `main` merely
  because GitHub presents it as the default.

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
  Engineering notes under `docs/` are excluded from the
  VitePress build.

## Documentation Routing

Before substantive changes, read the
[system map](docs/architecture.md) and check
[active execution plans](docs/exec-plans/active/).
Then read only the references relevant to the change:

- Backend: [BACKEND.md](docs/BACKEND.md).
- Frontend: [FRONTEND.md](docs/FRONTEND.md) and
  [web/ARCHITECTURE.md](web/ARCHITECTURE.md).
- UI or product behavior: [DESIGN.md](docs/DESIGN.md) and
  [core beliefs](docs/core-beliefs.md).
- Test or demo media: [test-assets.md](docs/test-assets.md).
- Frontend tooling or architecture docs:
  [vite-plus.md](docs/vite-plus.md) and
  [docts.md](docs/docts.md).
- Known debt: [tech-debt-tracker.md](docs/exec-plans/tech-debt-tracker.md).
- Harness itself (memories, skills, gates):
  [agent-harness.md](docs/agent-harness.md).

## Agent Memories And Skills

Recurring procedures live in `.agents/skills/lumilio-<name>/SKILL.md`; read
the skill before running its workflow. Current set:

- `select-checks` — map a diff to the narrowest checks
- `api-contract-change` — DTO / OpenAPI / `schema.d.ts`
- `write-a-test` — layer, placement, GPU self-skip
- `integration-spec` — Vitest component/flow specs
- `e2e-spec` — Playwright locators
- `frontend-i18n` — extract-then-fill and the canonical bilingual product terminology registry
- `e2e-environment` — Compose stack, seeds, slices
- `lumen-fixtures` — record/replay Hub inference
- `z-index` — stacking tokens
- `add-task-target` — Taskfile / CI wiring
- `feature-doc` — `doc.ts` / `doc.md`
- `pin-reconcile` — `assets.lock.json` / `lumen.lock.json`
- `exec-plan` — plan lifecycle

Project-coupled decisions live in `.agents/decisions/`; escaped-bug records
live in `.agents/postmortems/`. A non-trivial change updates one memory in
the same PR — the owning decision record, exec plan, or postmortem;
mechanical or local edits are exempt. Formats:
[agent-harness.md](docs/agent-harness.md).

## Non-Negotiable Rules

- Prefer root Task targets for repository workflows. Use `task server:test` for
  backend changes, `task web:test` for frontend changes, `task desktop:test` for
  desktop changes, and `task compose:test` for deployment changes. These are
  local development gates; `task test` intentionally runs only the architecture,
  Server, and Web gates. It does not include Site, Desktop, or browser E2E.
  Map a diff to its narrowest evidence with
  [lumilio-select-checks](.agents/skills/lumilio-select-checks/SKILL.md).
- Workflows call module targets directly (`task web:test`,
  `task server:test:ci`, `task web:e2e:up`). Root `ci:*` names exist only for
  real cross-module orchestration: `task ci:architecture`, `task ci:site`,
  `task ci:desktop:panel`, `task ci:desktop:native`. Do not add a 1:1 `ci:`
  wrapper for a target a workflow can already invoke.
- Keep the Taskfile/workflow boundary explicit. `.github/workflows/*.yml`
  owns triggers, path filters, runner and native dependency setup, credentials,
  caches, Buildx, and artifacts. Taskfiles own repository commands, working
  directories, sequencing, flags, and CI environment variables.
- When adding or changing a CI-relevant Taskfile target, update the affected
  workflow path filters in the same change. Follow
  [lumilio-add-task-target](.agents/skills/lumilio-add-task-target/SKILL.md).
- Follow the frontend test-layer taxonomy in
  [FRONTEND.md](docs/FRONTEND.md); do not invent test-file
  conventions. Placement:
  [lumilio-write-a-test](.agents/skills/lumilio-write-a-test/SKILL.md).
- API contracts are OpenAPI-first. Never hand-edit
  `web/src/lib/http-commons/schema.d.ts` or cast around a stale response type.
  Fix the backend DTO or annotation and run `task dto`
  ([lumilio-api-contract-change](.agents/skills/lumilio-api-contract-change/SKILL.md)).
  `task verify:generated` is the CI freshness gate for OpenAPI, `schema.d.ts`,
  feature `doc.md`, and config examples.
- The Server requires a complete schema-versioned TOML manifest. Do not add
  code defaults, consumer fallbacks, automatic config search, or ordinary
  environment overrides for runtime-immutable fields.
- Never commit secrets. TOML contains explicit secret-file paths only; secret
  values and secret-path overrides do not belong in environment variables.
- Keep generated files generated and record the command used. Format Go with
  `gofmt`; follow Vite+ fmt/lint for TypeScript.
- Frontend i18n is extract-then-fill. Define, change, and apply every
  human-facing product term through the canonical bilingual terminology
  registry, including Storage Location / 存储位置, Repository / 资源库, and
  Lumen capability labels. Procedure and registry:
  [lumilio-frontend-i18n](.agents/skills/lumilio-frontend-i18n/SKILL.md).
- Follow `web/ARCHITECTURE.md` for state ownership, thin routes, workflow
  placement, public entries, and dependency direction.
- Feature documentation uses a root `doc.ts`; every `{@link X}` must have a
  matching `import type`, and the generated sibling `doc.md` is never edited by
  hand
  ([lumilio-feature-doc](.agents/skills/lumilio-feature-doc/SKILL.md)).

## Execution Plans

Keep unfinished plans in `docs/exec-plans/active/`; follow
[lumilio-exec-plan](.agents/skills/lumilio-exec-plan/SKILL.md) for creation
criteria, the skeleton, and maintenance. Completing a plan extracts its
durable decisions into `.agents/decisions/` and deletes the plan file; there
is no completed archive — git history is the record.
