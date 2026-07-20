# ADR-005 asset synchronization

## Goal

Pin the published `Lumilio-Assets` revision in Lumilio Photos and provide the
ADR-005 `vp run assets:sync` entrypoint. The task fetches only the selected
profile's Git LFS objects into an ignored cache and verifies both the catalog and
media hashes.

## Implemented

1. Added root `assets.lock.json`, pinning `assets-v1.0.0` commit
   `ea484668bcc03e04d7c96a41bf904e3aab9c254e` and the exact manifest hash.
2. Added a dependency-free Node synchronization task under `web/scripts/`.
3. Exposed `vp run assets:sync` and `vp run assets:sync:test`.
4. Added catalog/profile/path/integrity checks and atomic cache replacement.
5. Documented contributor usage in both root READMEs.

## Evidence

- Focused Node tests: 3 passed.
- `make web-test`: 48 files and 152 tests passed.
- Cold-cache smoke sync: 3 media files materialized.
- Warm-cache smoke sync: existing files revalidated without download.
- Cold-cache e2e sync: 7 media files materialized.
- Both profiles used the pinned revision and verified every selected SHA-256.
