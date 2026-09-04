---
name: lumilio-select-checks
description: Use before pushing, opening a PR, or claiming checks pass in
  Lumilio Photos — maps the outgoing diff to the narrowest task targets that
  cover it instead of reflexively running the full suite.
---

# Select Checks For The Outgoing Diff

There is no universal local baseline. Every behavior change runs the narrowest
check that would fail for its regression; CI owns the exhaustive matrix
(`.github/workflows/ci.yml` calls module targets, plus `ci:architecture` /
`ci:site` / `ci:desktop:*`). Do not repeat a check that already passed, and
do not run the full suite for a scoped change. This is guidance, not a script.

## Inspect the diff

```sh
git status --short --branch
git diff --stat <base>...HEAD
```

## Evidence map

Run every row the diff touches, nothing more:

| Diff area | Check |
| --- | --- |
| `server/**` Go code | `task server:test`; `gofmt` on changed files |
| `server/**` SQL schema or queries | `task server:sqlc`, then `task server:test` |
| DTO, handler annotation, endpoint | [lumilio-api-contract-change](../lumilio-api-contract-change/SKILL.md) |
| `server/tools/fakelumen/**` or `web/e2e/compose.record.yml` | `cd server && go test ./tools/fakelumen/`; the E2E slice that records through it |
| `web/src/**` | focused `vp test run <file>` while editing; gate with `task web:test` |
| Bundling or production-only browser paths | `vp run test:bundle` from `web/` |
| `web/src/features/*/doc.ts` | [lumilio-feature-doc](../lumilio-feature-doc/SKILL.md); `task web:docs` |
| User-facing copy | [lumilio-frontend-i18n](../lumilio-frontend-i18n/SKILL.md) |
| `desktop/**` | `task desktop:test` |
| `deploy/**` or any compose file | `task compose:test` |
| Module boundaries, cross-module wiring | `task architecture:check` |
| `site/**` docs | `task ci:site` |
| `assets.lock.json` / `lumen.lock.json` | [lumilio-pin-reconcile](../lumilio-pin-reconcile/SKILL.md) |
| Generated artifacts (`schema.d.ts`, OpenAPI, `doc.md`, config examples) | `task verify:generated` |
| Taskfiles or workflows | [lumilio-add-task-target](../lumilio-add-task-target/SKILL.md); path filters update in the same PR |

Browser E2E slices (`task web:test:browser`, `web:test:auth-hardening`,
`web:test:auth-totp`, `web:test:agent-trust`, `web:test:agent-runtime`,
`web:test:video-semantic`, `web:test:backup-recovery`) need Docker and the E2E stack
([lumilio-e2e-environment](../lumilio-e2e-environment/SKILL.md)). Run only the
slice whose behavior the diff reaches; CI runs the matching slices.

`task test` (architecture + Server + Web) is for genuinely cross-cutting
changes, not a default. Reproduce a CI Server failure with
`task server:test:ci`.

## Handle failures

A failing relevant check blocks the push: fix it or report the blocker. Never
push hoping CI disagrees. If a failure looks environment-specific, record the
exact command and mismatch and confirm the non-platform evidence before
claiming it.

## Report

Report only the commands actually run and their results, plus which rows were
deliberately skipped and why.
