# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-09-22.

## Product paths

- **Linux bind-mount capacity grouping is unexecuted without mount privileges.**
  Owner: `server/internal/storage/storage_path_info_linux_test.go`
  `TestInspectStoragePathProvesSharedCapacityGroupAcrossBindMount`. The test
  creates a real `MS_BIND` mount and asserts one statfs-derived capacity group;
  it skips without `CAP_SYS_ADMIN`. Darwin/Windows CI never compile it, and
  Docker Desktop virtiofs/osxfs bind topologies are not this fixture.

- **Music embedded covers are not materialized as thumbnails.** Owner:
  `server/internal/processors/audio_helpers.go` and
  `web/src/features/music/components/MusicArtwork.tsx`. Bandcamp audio retains
  embedded artwork, but the audio pipeline does not generate image thumbnails;
  assigning an audio asset as album cover produces a 404. Demo albums use the
  placeholder until cover extraction and thumbnail generation are implemented.
- **Event rebuild still lacks a seven-media late-EXIF fixture and a legacy-data
  recovery run.** Owner: `server/internal/event`. Targeted API and domain tests
  cover the observed production failures, but there is no fixture of seven
  logical media whose capture times arrive after first publish, and no one-shot
  recovery against stuck claims, redirect chains, manual covers, and membership
  collisions. A late EXIF update can still miss an end-to-end proof that one
  Event publishes seven projected members; catalogs with leftover dirty-range
  claims can remain operator work rather than a gated recovery.
- **Video semantic E2E does not prove reprocess completion or persisted
  per-video frame counts.** Owner: `web/e2e/specs/video-semantic-regression.spec.ts`
  and the indexing `queued_jobs` field. Rebuilds (backfill and semantic reset)
  now wait on their own receipt via `GET /api/v1/assets/indexing/rebuild/{receipt_id}`
  before reading coverage, because coverage read earlier still counts the
  previous run's vectors (the reset deletes them only when the scheduler
  applies its first page). Per-asset reprocess receipts have no read endpoint,
  so that step still asserts acceptance only. `queued_jobs` remains a global
  backlog even when queried with a repository filter, so a slice cannot treat
  queue-idle or that counter as proof that this repository's video finished.
