---
name: lumilio-e2e-environment
description: Use when running, debugging, seeding, or recording the browser
  E2E stack in Lumilio Photos — Compose lifecycle, seed variants, slice
  selection, fakelumen, and the readiness gotchas that cause false failures.
---

# Run The Browser E2E Environment

The isolated stack is `web/e2e/compose.yml` (CI overlay `compose.ci.yml`,
recording overlay `compose.record.yml`): base URL `http://localhost:16657`,
admin `e2e-admin`. Test media comes from the pinned external assets
repository ([lumilio-pin-reconcile](../lumilio-pin-reconcile/SKILL.md)).
Docker is required. Spec locators:
[lumilio-e2e-spec](../lumilio-e2e-spec/SKILL.md). Lumen boundary:
[lumilio-lumen-fixtures](../lumilio-lumen-fixtures/SKILL.md).

## Lifecycle

```sh
task web:e2e:up        # start; every up tears volumes first (fresh catalog)
vp run e2e:logs        # service logs (from web/)
task web:e2e:down      # stop and discard volumes
```

CI sets `LUMILIO_E2E_COMPOSE_EXTRA=web/e2e/compose.ci.yml` for Buildx layer
cache. Recording sets it to `compose.record.yml` via `task lumen:record`.

Install the project-pinned Chromium once locally with
`task web:playwright:install` (or `vp exec playwright install chromium` from
`web/`). Linux CI uses `--with-deps`.

Rebuild the `lumilio` service after changing frontend source — the container
serves a built image, so edits are otherwise invisible:

```sh
docker compose -f web/e2e/compose.yml -p lumilio-photos-e2e up -d --build lumilio
```

The `@edwinzhancn/docts` package is fetched from GitHub Packages during the
image build; `environment.ts` copies `~/.npmrc` (or `LUMILIO_E2E_NPMRC`) into
`.cache/e2e/npmrc` as a BuildKit secret.

## Seed variants

Both seeders drive the real setup-status, repository, and upload HTTP APIs;
they do not touch the catalog directly. Slice tasks run their own seed; do
not pre-seed unless debugging.

| Command (from `web/` unless noted) | What it does |
| --- | --- |
| `vp run e2e:seed` | `assets:sync` smoke profile, then `e2e/support/seed.ts` |
| `vp run e2e:seed:video-semantic` | `assets:sync --profile e2e`, then the same seeder |
| `vp run demo:seed` | local demo against `:6680`, admin `lumilio-demo`, `demo` profile |

`demo:seed` flags: `--concurrency` (1–8), `--timeout` (seconds). Override the
API with `LUMILIO_DEMO_BASE_URL`.

## Slices

Each regression slice is one Task target wrapping a Playwright tag:

| Target | Tag | Note |
| --- | --- | --- |
| `task web:test:browser` | `@smoke` | general smoke |
| `task web:test:auth-hardening` | `@auth-hardening` | |
| `task web:test:auth-totp` | `@auth-totp` | runs **before** seeding: the browser must own the first-admin ceremony on a fresh instance |
| `task web:test:agent-trust` | `@agent-trust` | deterministic agent trust contract |
| `task web:test:video-semantic` | `@video-regression` | video-semantic seed profile; asserts fakelumen metrics |
| `task web:test:backup-recovery` | `@backup-recovery` | |

Run only the slice the change reaches
([lumilio-select-checks](../lumilio-select-checks/SKILL.md)). CI selects
slices through path filters in `.github/workflows/ci.yml` and calls these
same `web:*` targets — there are no `ci:web:e2e:*` wrappers.

## Gotchas

- Seeders wait for **ingestion only, not ML**. `search_embeddings` and
  semantic search populate asynchronously and only when a Lumen Hub (or
  fakelumen) is online — verify semantic state after ML workers drain, never
  right after a seed returns.
- Business endpoints return `409 app_not_initialized` until the instance has
  an admin plus exactly one primary repository; seeders self-bootstrap this.
  `demo:seed` creates the demo repository as primary on a fresh instance, or
  a separate regular repository when a primary already exists.
- Frontend upload completion means the River ingest job reached a terminal
  state, not that multipart transport returned 2xx.
- `server/tools/uploadbench` deliberately excludes ML — it benchmarks the
  pipeline and cannot validate embeddings.
- fakelumen metrics live at `http://127.0.0.1:16658/metrics`
  (`LUMILIO_E2E_LUMEN_METRICS_URL`). `video-semantic-regression.spec.ts`
  waits on `semantic_image` counts; a recording miss that falls back to the
  builtin vector still increments those counters.

## Report

Report the slice(s) run, the stack state left behind (`up` or torn down),
any seed profile used, and whether fakelumen was in replay or record mode.
