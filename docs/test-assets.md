# Test & Demo Assets

Test/demo media is **not** stored in this repo. It lives in an external git-LFS
repository (`github.com/EdwinZhanCN/Lumilio-Assets`); this repo only pins a
revision and materializes assets on demand with integrity checks. Never commit
media files here.

Pin, reconcile, and verify procedure:
[lumilio-pin-reconcile](../.agents/skills/lumilio-pin-reconcile/SKILL.md).
E2E seed and stack lifecycle:
[lumilio-e2e-environment](../.agents/skills/lumilio-e2e-environment/SKILL.md).

## Pin & profiles

Root `assets.lock.json` (schemaVersion 1) pins `repository`, `revision` (full
40-char SHA), `release`, default `profile`, and `manifestSha256` (integrity of
the source `assets.json` catalog). Only `release` is chosen by hand; reconcile
writes `release` + `revision` + `manifestSha256` together. Downgrades are
rejected. `task assets:check` is the offline CI gate.

The source repo holds `assets.json` (catalog: `id`, `media/...` path, `sha256`,
`bytes`), `profiles/<name>.json` (a list of asset IDs), and publishes immutable Git tags. Profiles: `smoke` (minimal, for e2e),
`demo` (full media pool), and `e2e` (the deterministic test set).

Sync materializes into `.cache/lumilio-assets/<revision>/<profile>/`. The cache
is validated (revision+profile+manifest) and reused.

## Seed contract

Seeders drive the real setup-status, repository, and upload HTTP APIs; they do
not touch the catalog directly. They wait for **ingestion only, not ML**.
`search_embeddings` / semantic search populate asynchronously afterward and
only when a Lumen Hub (or fakelumen) is online. Business endpoints return
`409 app_not_initialized` until admin + exactly one primary repository exist.


## Music corpus

Asset release `assets-v1.3.0` adds 35 free Bandcamp tracks across nine albums. Local
`demo` and browser profiles reference the same original bytes; purchased audio
and a separate private fixture path are excluded. `Lumilio-Assets/MUSIC-SOURCES.md`
and each catalog entry retain artist, source and CC BY attribution.

The compact Music smoke test imports an AAC remaster and original FLAC edition,
checks extracted tags and explicit release grouping through public APIs, and verifies real audio playback
continues across SPA navigation and advances in album order. The wider `e2e`
profile adds ALAC, MP3, Ogg Vorbis, AIFF and a long-track case.

Demo seeding uploads every selected original through the public API and waits
for each returned ingestion receipt to complete successfully. Retrying relies
on server-side duplicate resolution; unrelated assets or a matching total
count cannot mark a seed complete. Receipt polling is batched at the API limit
of 100 IDs and reports failed or timed-out imports.

Bandcamp originals have no stable release ID. After metadata is ready, the demo
seeder groups only its curated fixtures by catalog `source.albumUrl`, creates
Music albums through the public API and assigns ungrouped tracks with optimistic
revisions. Existing assignments are preserved on retry. Audio embedded-cover
thumbnail extraction is not implemented yet; demo albums use placeholder artwork.
