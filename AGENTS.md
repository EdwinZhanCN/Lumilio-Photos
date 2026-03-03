# Repository Guidelines

## Project Structure & Module Organization
- `server/`: Go API service. Entry point is `server/cmd/main.go`; business logic lives in `server/internal/*`; SQL migrations are in `server/migrations`; generated OpenAPI files are in `server/docs`.
- `web/`: React 19 + TypeScript frontend. Feature code is organized under `web/src/features/*`; shared utilities are in `web/src/lib`, `web/src/components`, and `web/src/contexts`.
- `wasm/`: Rust WebAssembly crates (`*-wasm`) used by web and plugin flows.
- `plugins/`: Studio plugin packages (for example `plugins/lumilio-border-plugin`).
- `docs/`: VitePress documentation site.

## Build, Test, and Development Commands
- `make setup`: install Go/Node dependencies and local tooling.
- `make dev`: start DB, Go server, and web app together.
- `make server-dev` / `make web-dev`: run one side only.
- `make server-test`: run backend tests (`go test ./...`).
- `cd web && npm run lint && npm run type-check && npm run test`: frontend quality gate.
- `make dto`: regenerate OpenAPI + frontend schema types after API contract changes.
- `npm run docs:dev` (root): run docs locally.

## Coding Style & Naming Conventions
- Go: always format with `gofmt` (tabs, standard import grouping). Keep package names lowercase and focused.
- TypeScript/React: follow existing 2-space indentation and ESLint/Oxlint rules. Prefer `@/...` path aliases in `web/`.
- Naming: React components use `PascalCase`; hooks use `useXxx`; tests use `*.test.ts(x)` and Go uses `*_test.go`.
- Do not hand-edit generated files such as `web/src/lib/http-commons/schema.d.ts`.

## Testing Guidelines
- Frontend tests run on Vitest; keep tests close to features (for example `web/src/features/studio/.../*.test.tsx`).
- Backend uses Go `testing` + `testify`.
- No enforced global coverage threshold is defined; for non-trivial changes, run `cd web && npm run test:coverage` and include results in PR notes.

## Commit & Pull Request Guidelines
- Follow Conventional Commit style used in history: `feat(scope): ...`, `fix(scope): ...`, `docs: ...`, `chore(repo): ...`.
- PRs should include: concise problem/solution summary, impacted paths, test commands run, and screenshots for UI changes.
- If API contracts changed, include generated artifacts from `make dto` (`server/docs/*` and `web/src/lib/http-commons/schema.d.ts`).

## Security & Configuration Tips
- Never commit secrets; use local `.env.development` files.
- Media pipeline features depend on external tools (`exiftool`, `ffmpeg`, `ffprobe`, `dcraw`). Verify availability before debugging processing failures.
- For frontend-specific guardrails, also follow `web/AGENTS.md`.
