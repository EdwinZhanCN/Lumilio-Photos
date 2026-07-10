# Frontend Production Readiness

Status: active

## Goal

Close the release-blocking frontend reliability, session-isolation, API-contract,
and media-pipeline gaps before the first production release. Large-library
performance work follows once the safety-critical paths are stable.

## Phase 1 — Contract and session safety (completed 2026-07-10)

- Restore a fully typed OpenAPI client dependency chain and remove the generated
  empty-object alternative from required JSON request bodies.
- Add compile-time contract tests before removing existing response casts.
- Introduce one atomic session-reset path that cancels in-flight work and clears
  user-scoped Query, Lumilio, notification, repository, search, and filter state.
- Serialize refresh-token rotation with a single in-flight refresh and make
  authenticated request replay safe for requests with bodies.
- Cover concurrent 401s, mutation replay, and user A → logout → user B isolation.

Implementation record:

- Replaced the checked-in OpenAPI fetch/query copies with official package
  dependencies and added a deterministic required-JSON-body normalization step
  to `make dto`.
- Added compile-time request/response contract tests and removed response casts
  that had hidden generated contract drift.
- Added one session-reset boundary for explicit logout, auth bootstrap failure,
  and refresh exhaustion. It invalidates late refreshes, aborts Lumilio, cancels
  and clears Query state, and clears notification, agent-context, repository,
  search, and filter state.
- Serialized refresh rotation, reused a token already rotated by a concurrent
  request, and replayed requests from pre-fetch clones so mutation bodies remain
  readable.
- Verified with `make dto`, `make web-test` (96 tests), and `cd web && vp build`.
  The authenticated Docker/Desktop release-gate walkthrough remains a release
  validation task after Phase 2 is implemented.

## Phase 2 — Production upload and background jobs (completed 2026-07-10)

- Serve the required COOP/COEP headers from both Caddy and the desktop Go SPA.
- Make hash and transport failures explicit; retain failed files in the upload
  queue and accept only successful HTTP responses as upload results.
- Connect upload progress and repository scan status to their backend lifecycle;
  invalidate asset queries only after materialization or terminal completion.
- Add production-browser smoke tests for `crossOriginIsolated`, BLAKE3 hashing,
  upload failure recovery, and scan/upload status transitions.

Implementation record:

- Added COOP/COEP headers to Caddy, Vite preview, and the desktop Go SPA static
  and fallback responses, with a desktop handler regression test.
- Made fetch/XHR transports reject non-2xx and abort responses, propagated hash
  worker failures, stopped the worker pool on the first failed digest, and kept
  failed `File` objects in the upload editor for retry.
- Added a user-scoped upload-ingest lifecycle endpoint. Upload rows remain in
  `processing` until their River ingest jobs reach terminal state; asset queries
  invalidate only after successful materialization.
- Repository scans now follow the backend scan run through terminal completion
  before clearing their busy state and invalidating repository-aware queries.
- Added a production-build Playwright smoke harness and `make web-browser-test`
  covering cross-origin isolation, BLAKE3 workers, non-2xx upload recovery, and
  upload/scan state transitions.
- Verified with `make dto`, `make server-test`, `make desktop-test`,
  `make web-test` (97 tests), `make web-browser-test`, and a normal
  `cd web && vp build`. Existing WASM resolution and large-chunk build warnings
  remain Phase 3 concerns.

## Phase 3 — Large-library performance

- Replace mount-once galleries with real viewport windowing and bounded query/
  media retention.
- Load map data by visibility and viewport instead of exhausting all GPS pages;
  fix location-cluster pagination used by Trips.
- Add route-level code splitting, with Studio, Map, Lumilio, Monitor, and Settings
  as the first split points. Lazy-mount the expanded ChatDock body.
- Consolidate duplicated server-state ownership for statistics, albums, filter
  options, and search. Split long files only along these behavior boundaries.

## Release gates

- `make web-test` and the separate `make web-browser-test` browser-worker job pass.
- OpenAPI contract tests prove known response/request types and reject unknown
  fields or empty required bodies.
- Docker and Desktop each pass: login → upload → processing complete → browse →
  logout → second-user login with no prior-user state visible.
- A large-library fixture demonstrates bounded gallery DOM/memory, and the main
  entry chunk has an enforced compressed-size budget.
- Authenticated browser walkthrough passes at 375, 768, 1024, and 1440 px in
  light and dark themes; root error recovery and unknown-route handling exist.

## Sequencing

Land each phase as small, independently verified PRs. Do not combine the safety
work with broad visual redesigns or mechanical long-file splitting. A release
candidate may be cut after Phases 1 and 2 pass; Phase 3 should be completed
before claiming high-performance support for very large libraries.

## Critical files for implementation

- `web/src/lib/http-commons/client.ts`
- `web/src/features/auth/AuthProvider.tsx`
- `web/src/features/upload/hooks/useUploadProcess.tsx`
- `web/src/workers/workerClient.ts`
- `web/src/features/assets/hooks/useAssetsView.tsx`
