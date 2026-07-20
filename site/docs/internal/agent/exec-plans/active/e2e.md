# ADR-005 E2E baseline

## Goal

Implement the ADR-005 Chromium smoke baseline with real first-party services,
profile-pinned assets, isolated state, and Playwright diagnostics.

## Implemented

- Added the isolated `docker-compose.e2e.yml` environment with a dedicated
  PostgreSQL volume and ignored storage directory.
- Added `vp run e2e:up`, `e2e:down`, `e2e:logs`, `e2e:seed`, and `e2e:test`.
- The seed resolves the locked smoke profile, initializes the real API, creates
  the worker admin and primary repository, and places the scan fixture in the
  isolated repository.
- Added `web/e2e/{fixtures,pages,specs,support}` and three `@smoke` specs for
  login/library entry, real UI upload, and real repository scan.
- Removed the custom production browser runner and the interim first-party API
  mock fixture.
- Re-homed the removed runner's capability gates: BLAKE3 worker coverage stays
  in `src/workers/hash.test.ts`; upload SSE/fallback transitions moved to
  `src/lib/upload/uploadLifecycle.browser.test.ts` (Vitest browser project,
  renamed from `hash-contract` to `browser`); upload non-2xx error mapping
  moved to `src/lib/upload/uploadTransport.test.ts` (happy-dom); the
  cross-origin-isolation header check moved to
  `e2e/specs/capabilities.spec.ts`, now asserted against the real Compose web
  service (Caddy) instead of a Vite preview.
- CI installs project-pinned Chromium, starts the isolated environment, runs
  smoke, uploads trace/screenshots/video/service logs on failure, and always
  tears the environment down.

- Locators follow the ADR-005 locator strategy: `playwright.config.ts` pins
  `locale: "en-US"`, `e2e/support/i18n.ts` resolves accessible names from the
  same `en` bundle the app renders, and `e2e/support/seed.mjs` writes
  `.cache/e2e/seed.json` so specs read seeded state instead of restating it.
  No spec contains a UI copy literal.
- `Field` now generates the label/input id with `useId` and passes it through
  context, so `TextInput` and `PasswordField` pair automatically. The unused
  `useFieldId` helper and the never-passed `htmlFor` prop were removed.
- `SideBar` renders a `<nav>` landmark instead of a bare `<div>`.
- `e2e:up` tears the environment down first. The bootstrap password is
  regenerated per run but PostgreSQL only applies it when initializing an empty
  data directory, so reusing a volume failed authentication; every `up` now
  starts from the empty database each job is supposed to get.

## Evidence

- Playwright discovery lists 4 smoke tests in 4 independent spec files.
- Compose configuration resolves successfully.
- `make web-test` passes: 50 files and 156 tests; typecheck, lint, and source
  boundaries pass.

Verified against a real Docker daemon on 2026-07-20:

- Cold `vp run e2e:up` and `vp run e2e:seed` from a removed volume.
- Each stateful spec alone in its own cold environment: `login` 1.6s,
  `scan` 2.6s, `upload` 7.0s. Upload runs in 2.5s against a warm library and
  7.0s cold, and the server log shows a fresh `Task ... enqueued for processing
  file upload-001.jpg` plus three thumbnail writes each cold run, so the pass is
  real work rather than leftover state.
- The full Chromium `@smoke` subset: 4 passed.
- Repeated clean cycles, and back-to-back `e2e:up` without an intervening
  `e2e:down`.

Verified on CI in PR #155, run 29767782431: `make web-test` plus a serial
Chromium `@smoke` run, 4 passed, on a cold cache with no asset, browser, or
Docker layer caching. Teardown succeeds.

Artifact readability was proved by three real failures rather than a staged one.
Each was diagnosed by downloading `test-results/services.log` from the failed
run, which named the exact cause, and the fourth run was clean.

## Linux-only failures this surfaced

None of these reproduce on macOS, where Docker's file sharing maps ownership
loosely. All three were host/container uid conflicts on the same bind mount.

1. The bootstrap password was written 0600 by the host, so the server, running
   as uid 10001, could not read its own secret. Plain Compose bind-mounts secret
   files as-is and has no uid/gid/mode option, so the file is now 0644. It is
   random per run and lives in an ignored cache directory.
2. The server could not create its storage layout under a host-owned directory.
3. Teardown could not remove `/data/storage/.secrets`, which the server creates
   with restrictive permissions.

Opening directory modes only relocated the conflict, since the container kept
creating paths the host had not anticipated. Storage is now a named volume, so
the two sides share no directory at all: Docker seeds a fresh volume from the
image, so the chowned `/data/storage/primary` is already present, and
`down --volumes` disposes of the tree without the host touching container-owned
files. The seed places the scan fixture with `docker compose cp`.

A `db` volume mounted at the pre-18 `/var/lib/postgresql/data` makes the
PostgreSQL 18 image exit(1) even on a fresh volume. All four compose files were
moved to `/var/lib/postgresql`; developers with volumes created before that
change must recreate them.

## Remaining work

- Per-worker isolation. ADR-005 gives each Playwright worker its own user and
  repository, but the seed creates one admin and one repository shared by every
  worker. CI masks this with `workers: 1`; locally four workers already run
  `scan` and `upload` against the same repository.
- `vp run demo:seed`.
- CI caching for assets, the Playwright browser, and Docker layers. The run
  above downloads and builds all three from scratch.

