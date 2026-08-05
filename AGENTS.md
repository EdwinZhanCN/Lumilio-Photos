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

## Canonical Lumen Capability Terminology

The following four user-facing capability names are exact product terms. Use
them consistently in the Web UI, Desktop UI, CLI TUI, documentation, READMEs,
release notes, and deployment tools.

| Internal service | Simplified Chinese | English |
| --- | --- | --- |
| `siglip` | `图像语义分析` | `Image Semantic Analysis` |
| `face` | `人物识别` | `Person Recognition` |
| `ocr` | `OCR文字识别` | `OCR Text Recognition` |
| `bioclip` | `BioCLIP物种识别` | `BioCLIP Species Recognition` |

Do not rename these capabilities as `语义搜索` / `Semantic Search`, `人脸识别`
/ `Face Recognition`, bare `OCR`, or bare `物种识别` / `Species Recognition`.
Descriptions may explain that a capability enables natural-language search,
face processing, text extraction, or species classification, but the capability
label itself must use the exact term above. Protocol task names, model names,
database fields, and API identifiers remain unchanged.

## Non-Negotiable Rules

- Prefer root Task targets for repository workflows. Use `task server:test` for
  backend changes, `task web:test` for frontend changes, `task desktop:test` for
  desktop changes, and `task compose:test` for deployment changes. These are
  local development gates; `task test` intentionally runs only the architecture,
  Server, and Web gates. It does not include Site, Desktop, or browser E2E.
- The root `ci:*` targets are the CI contracts and must be runnable from the
  repository root: `task ci:architecture`, `task ci:server`, `task ci:web`,
  `task ci:site`, `task ci:desktop:panel`, and `task ci:desktop:native`.
  Web CI slices use `task ci:web:playwright:*` and
  `task ci:web:e2e:*`; use the corresponding `web:*` targets for local,
  module-scoped runs.
- Keep the Taskfile/workflow boundary explicit. `.github/workflows/*.yml`
  owns triggers, path filters, runner and native dependency setup, credentials,
  caches, Buildx, and artifacts. Taskfiles own repository commands, working
  directories, sequencing, flags, and CI environment variables. Workflows
  should invoke root `task ci:*` contracts rather than reimplementing repository
  commands inline.
- When adding or changing a CI-relevant Taskfile target, update the affected
  workflow path filters in the same change. In particular, Web E2E filters must
  include `web/taskfile.yml`, and Site/Server/Web filters must include the root
  `taskfile.yml` when its orchestration changes.
- Follow the frontend “Test layers” taxonomy in
  [FRONTEND.md](site/docs/internal/FRONTEND.md); do not invent test-file
  conventions.
- API contracts are OpenAPI-first. Never hand-edit
  `web/src/lib/http-commons/schema.d.ts` or cast around a stale response type.
  Fix the backend DTO or annotation and run `task dto`.
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
