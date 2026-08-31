# Test & Demo Assets

Test/demo media is **not** stored in this repo. It lives in an external git-LFS
repository (`github.com/EdwinZhanCN/Lumilio-Assets`); this repo only pins a
revision and materializes assets on demand with integrity checks. Never commit
media files here.

Pin, reconcile, and verify procedure:
[lumilio-pin-reconcile](../../../.agents/skills/lumilio-pin-reconcile/SKILL.md).
E2E seed and stack lifecycle:
[lumilio-e2e-environment](../../../.agents/skills/lumilio-e2e-environment/SKILL.md).

## Pin & profiles

Root `assets.lock.json` (schemaVersion 1) pins `repository`, `revision` (full
40-char SHA), `release`, default `profile`, and `manifestSha256` (integrity of
the source `assets.json` catalog). Only `release` is chosen by hand; reconcile
writes `release` + `revision` + `manifestSha256` together. Downgrades are
rejected. `task assets:check` is the offline CI gate.

The source repo holds `assets.json` (catalog: `id`, `media/...` path, `sha256`,
`bytes`), `profiles/<name>.json` (a list of asset IDs), and publishes
`release.json` with every release. Profiles: `smoke` (minimal, for e2e),
`demo` (full image pool), and `e2e` (the deterministic test set).

Sync materializes into `.cache/lumilio-assets/<revision>/<profile>/`. The cache
is validated (revision+profile+manifest) and reused.

## Seed contract

Seeders drive the real setup-status, repository, and upload HTTP APIs; they do
not touch the catalog directly. They wait for **ingestion only, not ML**.
`search_embeddings` / semantic search populate asynchronously afterward and
only when a Lumen Hub (or fakelumen) is online. Business endpoints return
`409 app_not_initialized` until admin + exactly one primary repository exist.
