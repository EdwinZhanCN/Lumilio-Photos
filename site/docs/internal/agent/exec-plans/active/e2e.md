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
  moved to `src/lib/upload/uploadTransport.test.ts` (Vitest `unit` project, Node); the
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

## Worker isolation

The three-layer model is complete. `e2e/support/seed.mjs` only leaves a migrated
database, a bootstrap admin, and the instance's single primary repository. The
worker-scoped `workspace` fixture then provisions each Playwright worker through
the real API: register `e2e-w{index}`, promote it to admin, create the
`E2E Worker {index}` repository, and place the scan fixture with
`docker compose cp`. The fixture is lazy, so a spec that does not take
`workspace` — `capabilities` — provisions nothing.

Two details follow from the product's existing rules rather than working around
them. Self-service registration only makes the first account an admin, and
repository and scan endpoints require admin, so the bootstrap admin promotes
each worker user through `PATCH /users/:id`; multi-admin is already supported,
including a last-active-admin guard and a role selector in settings. Worker
repositories are `regular` because `repositories_one_primary_idx` allows one
primary per instance.

Verified with four workers uploading concurrently: each upload enqueued its own
task into its own repository, so duplicate detection is scoped per repository
rather than globally by content hash. The smoke profile is deliberately minimal
— three assets — so workers share source bytes and separate themselves by
filename, repository, and owner instead.

Note that `assets.json` catalogues the entire asset repository while
`assets:sync` only materialises what `profiles/smoke.json` references. Resolve
assets through the profile, not the catalogue, or you will reference files that
were never downloaded.

## Demo seed

`vp run demo:seed` materialises the pinned `demo` profile — the 225-photo pool —
into a running instance through the real setup, repository and upload APIs. It
shares profile resolution and verification with the E2E seed by calling
`syncAssets` from `scripts/assets-sync.mjs`; only the profile differs. Point it
with `LUMILIO_DEMO_BASE_URL` (default `http://localhost:6680`); credentials and
the repository name are overridable through `LUMILIO_DEMO_*`.

Media lands in its own `Lumilio Demo` repository so it never mixes with a real
library, an existing repository is reused rather than recreated, and a second
run is a no-op once the assets are present.

This supersedes `server/tools/devseed`, which read the 225 photos from
`demo/seed/library` — a directory that is gitignored and untracked, so the tool
only worked on a machine that happened to have it.

### What seeding demo data into an E2E instance exposed

Running the suite against an instance that already held the demo library failed
`upload` and `scan`, and the reason corrected an assumption: **the gallery is not
partitioned by owner**. A worker user could see all 233 assets, including the
225 uploaded by the bootstrap admin. Per-worker users therefore do not isolate
what a spec observes; the isolation comes from per-worker repositories and
filenames.

Both specs now scope the gallery to their own repository through
`GalleryPage.scopeTo`. Without it they passed only because the E2E database is
otherwise empty, so the asset happened to render on the first page — an implicit
dependency on ambient state. Note the scope select is rendered twice for the
responsive layouts, so the locator filters to the visible one.

## CI caching

The web job caches the Playwright browser and the E2E image layers in the free
per-repo GitHub Actions cache — no registry or account tier. Playwright browsers
restore from `~/.cache/ms-playwright` keyed on the lockfile, with `install-deps`
still run for the apt libraries. Image layers use the `type=gha` buildkit
backend, enabled by a docker-container buildx builder plus the ACTIONS_* runtime
env, and wired per service with distinct scopes through the CI-only
`docker-compose.e2e.ci.yml` (layered on via `LUMILIO_E2E_COMPOSE_EXTRA`; local
runs build without a cache backend).

Measured on the web job: "Start isolated E2E environment", which includes
`compose up --build`, dropped from 428s cold to 100s warm, and the Playwright
browser download (249 MB) became a 4s cache restore. The first run on a branch
is still cold — it populates the cache — so the speedup shows from the second
run on.

Asset caching was intentionally skipped: the smoke profile is three files, so
its LFS pull is already seconds. Revisit if a heavier profile enters CI.

## Remaining work

- Retire `server/tools/devseed` once nobody depends on it.

