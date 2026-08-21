# Tech Debt Tracker

Keep this list short. Each item must describe current behavior, name a concrete
owner path, and explain the user or release impact. Completed history belongs in
the relevant exec plan, not in this file.

Last aligned with the codebase: 2026-08-20.

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
