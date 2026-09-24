# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-09-23.

## Product paths

- **Linux bind-mount capacity grouping is unexecuted without mount privileges.**
  Owner: `server/internal/storage/storage_path_info_linux_test.go`
  `TestInspectStoragePathProvesSharedCapacityGroupAcrossBindMount`. The test
  creates a real `MS_BIND` mount and asserts one statfs-derived capacity group;
  it skips without `CAP_SYS_ADMIN`. Darwin/Windows CI never compile it, and
  Docker Desktop virtiofs/osxfs bind topologies are not this fixture.

- **A manual scan right after a file lands can skip it until the next
  periodic verification.** Owner: `server/internal/storage/repository_fs.go`
  (the `settling` skip) and the Repository verification scheduler. The
  verifier deliberately skips files modified within
  `repository_scan.settle_seconds` and ends the run `partial` with a retryable
  `repository/scan-incomplete` problem, but nothing schedules a follow-up, so
  a user who drops a file and clicks Scan within the window must rescan by
  hand or wait for the next interval. Fix: queue one delayed verification when
  a crawl reports settling skips. Found by #207.

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

## Found during release hardening (2026-09-24)

- **Share UI controls have no accessible names.** Owner:
  `web/src/features/assets/flows/export/AssetExportDialog.tsx` (icon-only
  Share, Studio, Add to album buttons; tooltip text is not a name) and
  `web/src/features/share/flows/public/PublicShareGrid.tsx` (tiles are buttons whose
  only content is an `alt=""` image). Screen-reader users cannot identify
  these controls; E2E has to go through the gallery bulk action instead.
- **`LoginPage.signIn` waits only the default 5s for the post-login URL.**
  Owner: `web/e2e/pages/login.page.ts`. Every spec that signs in inherits it;
  it timed out under heavy CPU throttling (never yet in CI). Give it an
  explicit, commented budget like `PLAYBACK_START_TIMEOUT` if it appears in CI.
- **One unexplained pause during music-agent audition.** Owner:
  `web/e2e/specs/music-agent.spec.ts:163` and
  `web/src/features/music/state/MusicPlayerProvider.tsx`. Seen once on an idle
  local stack (a real `pause` at 0.067s between filling and clicking Refine);
  not reproduced in ~125 runs, no code path found. Capture a trace if it
  recurs.
- **`useMergePeople` casts its response.** Owner:
  `web/src/features/people/api/usePeople.ts` (`as
  Promise<PersonCorrectionResponse>`). A cast around an API response usually
  means a stale DTO or `@Success` annotation; check the merge handler's
  OpenAPI annotation and regenerate instead.
- **Pre-commit fails on a commit whose only Markdown file is a generated
  `doc.md`.** Owner: `web/` lint-staged/`vp staged` config. `vp fmt` ignores
  `doc.md`, so the Markdown task errors with "Expected at least one target
  file"; the only workaround today is `--no-verify`.
