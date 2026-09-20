# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-09-19.

## Product paths

- **AgentBoard has no mobile column reflow.** Owner:
  `web/src/features/lumilio/flows/board/AgentBoard.tsx`. It renders one
  persisted 12-column layout at every width, so phone columns compress into
  narrow slivers. Add a client-only narrow-screen remap or a separately
  persisted breakpoint layout, then verify it against a live backend without
  corrupting the canonical desktop layout.
- **Agent confirmation does not reconcile after a post-commit disconnect.**
  Owner: `web/src/features/lumilio/state/chatStore.ts`. If the effect commits
  but the receipt SSE is lost, reloading the client does not query the durable
  effect status and can leave the outcome ambiguous until the user inspects
  the affected resource. Reconcile pending confirmation identity through the
  scoped effect-status endpoint without replaying the mutation.
- **Populated Person Recognition relations can be dropped by timestamp decoding.**
  Owner: `server/internal/api/dto/asset_dto.go` and
  `server/internal/db/repo/queries/relationships.sql`. SQLite relation JSON
  emits Unix-microsecond integers for face-result timestamps, while
  `AssetFaceResultDTO` currently unmarshals them directly into `time.Time`.
  Mirror the internal aggregate conversion now used by OCR and add a focused
  relation test before relying on `include_faces=true` in a user-facing flow.
- **Event rebuild still lacks a seven-media late-EXIF fixture and a legacy-data
  recovery run.** Owner: `server/internal/event`. Targeted API and domain tests
  cover the observed production failures, but there is no fixture of seven
  logical media whose capture times arrive after first publish, and no one-shot
  recovery against stuck claims, redirect chains, manual covers, and membership
  collisions. A late EXIF update can still miss an end-to-end proof that one
  Event publishes seven projected members; catalogs with leftover dirty-range
  claims can remain operator work rather than a gated recovery.
- **Video semantic E2E does not prove operation-scoped completion or persisted
  per-video frame counts.** Owner: `web/e2e/specs/video-semantic-regression.spec.ts`
  and the indexing `queued_jobs` field. Backfill/reprocess currently assert
  request acceptance. `queued_jobs` remains a global backlog even when queried
  with a repository filter, so a slice cannot treat queue-idle or that counter
  as proof that this repository's video finished.

