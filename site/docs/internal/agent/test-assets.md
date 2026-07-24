# Test & Demo Assets

Test/demo media is **not** stored in this repo. It lives in an external git-LFS
repository (`github.com/EdwinZhanCN/Lumilio-Assets`); this repo only pins a
revision and materializes assets on demand with integrity checks. Never commit
media files here.

## Pin & profiles

- Root `assets.lock.json` (schemaVersion 1) pins `repository`, `revision` (full
  40-char SHA), `release`, default `profile`, and `manifestSha256` (integrity of
  the source `assets.json` catalog). To change the fixture set, bump these — a
  mismatch fails the sync loudly.
- The source repo holds `assets.json` (catalog: `id`, `media/...` path, `sha256`,
  `bytes`) and `profiles/<name>.json` (a list of asset IDs). Two profiles:
  `smoke` (minimal, for e2e) and `demo` (full image pool).

## Sync — `vp run assets:sync [--profile <name>]`

`web/scripts/assets-sync.mjs`. Shallow-fetches the pinned revision with
`GIT_LFS_SKIP_SMUDGE=1`, verifies `assets.json` against `manifestSha256`,
sparse-checkouts only the selected profile's media + manifest, `git lfs pull`s
just those files, verifies each file's `sha256`+`bytes`, and atomically
materializes into `.cache/lumilio-assets/<revision>/<profile>/`. The cache is
validated (revision+profile+manifest) and reused, so re-runs are cheap.

## Seed a running instance

Both seeders drive the real setup/repository/upload HTTP APIs; they do not touch
the DB directly.

- **`vp run demo:seed`** (`web/scripts/demo-seed.mjs`) — local demo. Syncs the
  `demo` profile, runs `/api/v1/setup`, creates/logs in admin
  `lumilio-demo` / `Lumilio-Demo-2026!`, creates a dedicated `Lumilio Demo`
  repository, uploads via `POST /api/v1/assets` against `http://localhost:6680`
  (`LUMILIO_DEMO_BASE_URL`), then waits for ingestion. Flags: `--concurrency`
  (1–8), `--timeout` (seconds).
- **`vp run e2e:seed`** — `assets:sync` (smoke) + `e2e/support/seed.mjs` into the
  e2e stack (base `:16657`, admin `e2e-admin`). The `docker-compose.e2e.yml`
  stack is managed by `vp run e2e:up | e2e:down | e2e:logs`.

## Gotchas

- Seeders wait for **ingestion only, not ML**. `search_embeddings` / semantic
  search populate asynchronously afterward and only when a Lumen Hub is online —
  verify those after the ML workers drain, not right when a seed finishes.
- `server/tools/uploadbench` deliberately **excludes** ML/AI processing; it is a
  pipeline benchmark, not a way to validate embeddings.
- First-run readiness: business endpoints (incl. `GET /repositories`) return
  `409 app_not_initialized` until the instance is fully bootstrapped — db
  credential rotated + admin + exactly one **primary** repository. `demo:seed`
  self-bootstraps this: on a fresh instance it creates the demo repository as the
  primary via the ungated `POST /repositories`; when a primary already exists it
  adds a separate regular demo repository.
