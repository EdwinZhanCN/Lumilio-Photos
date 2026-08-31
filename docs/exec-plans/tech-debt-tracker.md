# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-08-25.

## Product paths

- **No platform p99 certification for the SQLite runtime.**
  Owner: a future release/performance certification on a pinned Linux host.
  The write-concurrency implementation has deterministic coverage and the
  full Server gate, but steady-state p99 was never certified on any platform,
  so release notes must not claim p99 latency targets for the catalog runtime.

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
