# Event System Convergence and Correctness

Status: active. Event correction algebra, frontend convergence, and the typed
owner-wide rebuild commit path are landed. The seven-media fixture,
legacy-data recovery run, and repository gates remain before this plan can
complete.

Primary owners: `server/internal/event`, `server/internal/queue`,
`server/internal/sourcing`, `server/internal/service`, `server/internal/api`, and
`web/src/features/events`.

This plan reopens contracts the earlier media-semantic-organization plan had
recorded as complete (that plan is deleted; git history retains it). The
shipped Event model is documented in [BACKEND.md](../../BACKEND.md); this plan
is the authority for the unfinished correctness work.

## Goal

Make Events a convergent owner-scoped derived topology whose membership atom is
`media_item_id`, with Repository Browse Scope applied only as a read projection.
Every fact change, rebuild, manual correction, API consumer, and UI count must
resolve through one consistent lifecycle.

The work is complete only when:

- late EXIF, GPS, logical-media, Stack, and deletion changes automatically
  converge without a manual repair step;
- a failed or stale rebuild cannot change published Events or report success;
- merge, split, move, add, and remove remain stable after arbitrary rebuilds;
- list, detail, gallery, search, share, relations, and Agent tools resolve the
  same authorized Event member set;
- header counts, covers, and gallery results use the same Browse Scope; and
- rebuild state is observable and its UI wording is truthful.

## Non-goals

- Do not make Event identity repository-scoped.
- Do not use ML, faces, OCR, semantic embeddings, or LLM output to determine
  Event membership.
- Do not tune `events-v1` time or distance thresholds unless a correctness test
  proves that the policy itself is wrong.
- Do not implement incremental time-window rebuilding in this plan. Until a
  benchmark justifies it, rebuilding one dirty owner is the canonical behavior.
- Do not redesign People. Shared resolver and Browse Scope primitives may be
  reused, but Event correctness must not depend on a People rewrite.

## Baseline failures to preserve as regressions

The first implementation slice must encode these observed failures as tests
before changing production behavior:

1. Seven logical media captured within approximately five minutes initially
   produced one published Event member because Event rebuilding ran before EXIF
   extraction and the later capture-time update did not invalidate Events.
2. Publishing a changed partition failed on the global
   `event_media_items.media_item_id` unique index because new memberships were
   inserted before old memberships were removed.
3. The failed River job retained claimed dirty rows; its retry claimed zero rows
   and completed successfully even though no new Event generation was
   published.
4. `/api/v1/assets/list` and `/api/v1/assets/search` accepted `event_id` but the
   active browse/search path ignored it, so an Event detail gallery could show
   unrelated Browse Scope assets.
5. A rename or hide-only Event patch persisted the generated cover as a manual
   cover override.
6. Remove, move, split, and merge corrections could be undone by a later
   rebuild or make reconciliation fail.

## Fixed domain contracts

### Topology and projection

- Canonical Event construction is owner-wide and repository-independent.
- Canonical membership contains logical `media_item_id` values, never a
  snapshot of physical asset IDs.
- Owner authorization is not inherited from generic library browsing: an
  administrator may use a nil owner scope for global asset reads, but every
  Event-filtered asset query must carry the Event's authenticated owner. Nil
  owner scope must never be used to resolve an Event.
- Repository Browse Scope is an `AND` projection applied after owner and Event
  authorization. It never changes Event identity, reconciliation, or rebuild
  input.
- Event detail responses expose both canonical and projected display counts
  when a repository scope is active. The projected count, cover, and gallery
  must come from the same resolved set.
- Eligibility and displayability are separate named policies. Trash, missing
  primaries, and unavailable components cannot be handled differently by each
  consumer.

### Dirty state

- Replace range claims as the correctness authority with owner revisions:
  `source_revision` records known fact changes and `published_revision` records
  the revision represented by published Events.
- `source_revision > published_revision` is the only definition of pending
  rebuild work.
- A single `MarkEventFactsChangedTx` boundary increments the owner revision;
  the owning mutation publishes a typed projection envelope in that same
  transaction, and the domain-outbox dispatcher later publishes the
  `rebuild_projection_batch` macro command.
- The following mutations must call that boundary:
  - logical media creation and permanent deletion;
  - effective capture time, capture offset, timezone, or GPS change;
  - JPEG+RAW, Live Photo, primary/component, and other logical-media rewrites;
  - automatic or manual Stack create, extend, remove, and delete;
  - trash and restore changes that affect eligibility or displayability; and
  - manual Event membership corrections.
- If envelope persistence fails, the fact mutation and revision update roll
  back together. A later QueueDB insertion failure leaves the committed
  envelope retryable and does not roll back product facts.
- Existing `event_dirty_ranges` may be migrated for recovery, but no code may
  continue treating unused ranges as incremental computation windows.

### Rebuild and publish

One rebuild follows this protocol:

1. Acquire a renewable owner-specific rebuild lease. Different owners may run
   concurrently; the same owner may not.
2. Read candidates, constraints, published Events, and presentation state from
   one stable SQLite read snapshot and record its expected `source_revision`.
3. Compute segmentation and reconciliation without holding a write
   transaction.
4. Pre-validate membership uniqueness, cover membership, direct redirects, and
   correction invariants.
5. In one SQLite write transaction, verify the expected revision and lease,
   replace the complete owner membership set in delete-before-insert order,
   apply Event state and redirects, and advance `published_revision`.
6. Commit once. Any validation, lease, revision, or SQL failure leaves the
   previous published topology unchanged.

Additional rules:

- Generate one `derivation_run_id` per rebuild, not one per member.
- A stale revision is not an error and may not publish; the queued follower
  rebuilds the newest revision.
- A failed worker always releases its lease and remains retryable. Zero newly
  claimed work is never evidence that an earlier failed publish succeeded.
- Rebuild work uses the closed `rebuild_projection_batch` macro and the same
  owner revision/follower rules; there is no separate scheduler queue.
- At most one pending follower exists per owner while a rebuild is running.
- Resume advances desired state and publishes the macro command atomically.
- Persist rebuild runs with queued/running/succeeded/failed/stale state,
  requested and published revisions, timestamps, counts, and a stable error
  code. API responses must not expose raw internal errors.

### Identity, redirects, and presentation

- Add a terminal `retired` Event state. An empty owner or an Event losing its
  final member is valid convergence, not a reconciliation error.
- A redirect is created only when old and new partitions have a real identity
  relationship. Zero-overlap Events are retired without an unrelated redirect.
- Redirects are direct, acyclic, owner-scoped, and point only to an active
  target. Database enforcement applies to both insert and update paths.
- For an automatic split, the greatest-overlap successor retains the old ID.
  For a merge, one deterministic survivor retains its ID and losers redirect
  directly to it.
- Title and hidden state remain on the surviving identity. A cover override
  follows the successor containing that media; otherwise it is cleared and a
  generated cover is selected.
- Presentation-only patches do not increment the topology revision unless they
  change membership or eligibility.

### Manual corrections

- API commands express exact logical-media intent; handlers do not directly
  manipulate incidental segmentation details.
- Replace the current hard-label behavior with exact member assignments plus
  explicit join/cut boundaries. Assigning one media must not absorb its entire
  chronological segment unless the selected logical-media set requires it.
- `must_link` joins only the selected logical media or complete selected Stack;
  it does not lock every chronological item between two endpoints.
- Merge records a durable union with an explicit survivor.
- Split records a durable partition boundary and returns both resulting Event
  identities.
- Move/add affects exactly the selected logical media set.
- Remove persists enough partition intent to keep those members outside the
  source Event and assigns them to a valid successor Event; it may not leave an
  unowned logical media or an impossible redirect.
- Batch commands are atomic and accept an expected Event version plus an
  idempotency key. Partial sequential mutation is not a supported contract.
- For every command, `command -> rebuild -> rebuild` must produce the same
  partition and presentation state as the first successful rebuild.

### One Event set resolver

Create one public owner-aware Event set resolver that returns:

- canonical active Event identity after redirect resolution;
- ordered logical media IDs;
- displayable representative assets;
- canonical and repository-projected counts; and
- canonical and projected cover candidates.

The following paths must use it rather than handwritten Event membership SQL:

- Event list, detail, ordered assets, and relations;
- Assets list, text search, semantic search, and fused search;
- immutable share snapshots;
- Agent `lookup_events` and `filter_assets(event_id)`;
- Person-to-Event and Event-to-Person/Album/Location relations; and
- frontend Event picker and viewer navigation.

An invalid, redirected-without-target, or cross-owner Event resolves as typed
not found. It must never fall back to the owner library.

## API contract changes

- `POST /api/v1/events/rebuild` enqueues work and returns
  `202 Accepted` with a `run_id`; it no longer publishes synchronously.
- Remove the ignored `from` and `to` request fields while rebuilding is
  owner-wide. A future incremental implementation requires a separate plan.
- `GET /api/v1/events/rebuild/status` reports paused, pending, queued, running,
  last success, last failure, revisions, and progress.
- `PATCH /api/v1/events/rebuild/state` resumes by transactionally enqueueing
  dirty work.
- Event asset pagination implements its declared cursor or removes the cursor
  field; the final OpenAPI contract may not advertise a nonfunctional cursor.
- Detail and list use the same optional repository projection and return
  explicit total/projected counts.
- Merge, split, move/add, and remove receive atomic batch DTOs with typed
  validation, not-found, version-conflict, and correction-conflict errors.
- All rebuild status/state routes receive OpenAPI annotations. Generated
  TypeScript DTOs are updated through `task dto`; generated schema files are
  never edited by hand.

## Frontend contract

- Event server state remains owned by query hooks; components receive typed
  view models and command callbacks.
- The index and maintenance surface poll or subscribe to rebuild-run state.
  Publishing a revision invalidates Event list, detail, gallery, relations, and
  picker queries together.
- A detail page does not claim that its own changes are waiting merely because
  the owner has unrelated dirty work. Library-wide work uses library-wide copy.
- Header count, cover, gallery, selection, viewer navigation, and share preview
  use the same resolved repository projection.
- Bulk operations call one atomic API rather than sequential requests.
- Event picker search is server-side and is not limited to the first 100 loaded
  Events.
- Date formatting uses the Event capture timezone/offset contract rather than
  silently grouping in the browser timezone.
- Event pages contain no mock fallback data.

## Execution phases

### Phase 0 — Lock the failures

- Add backend integration fixtures for the seven-media late-EXIF incident,
  membership migration unique collision, failed-job retry, and correction
  rebuild convergence.
- Add API tests proving `event_id` is enforced by list and every search mode.
- Add a frontend integration assertion that header and gallery counts match for
  All Repositories and a single-repository scope.
- If Event write operations are exposed in a production build before Phase 4,
  guard merge/split/move/add/remove behind a temporary capability flag with an
  explicit deletion condition.

Exit: all observed failures reproduce deterministically in tests.

### Phase 1 — Revision and macro lifecycle

- Migrate owner dirty state to source/published revisions and add rebuild-run
  observability.
- Introduce `MarkEventFactsChangedTx` and wire every fact mutation listed above.
- Use the closed `rebuild_projection_batch` macro with owner lease, follower
  dedupe, pause/resume, stale lease, cancellation, and failure cleanup.
- Add a recovery scan that re-arms every unpaused owner whose source revision is
  newer than its published revision.

Exit: no fact change can commit without either a deduplicated queued rebuild or
a rolled-back mutation, and failed jobs remain truthfully pending.

### Phase 2 — Atomic topology publish

- Load rebuild input from one read snapshot.
- Correct segmentation/reconciliation edge cases for empty owners, zero
  overlap, direct redirects, and user-state retention.
- Replace membership in safe global order inside one revision-checked
  transaction and validate covers before commit.
- Add `retired` lifecycle handling and repair existing redirect chains.
- Provide a one-time recovery path that releases legacy claims and rebuilds
  affected owners through the new protocol.

Exit: fault injection at every compute/publish boundary leaves either the old
complete topology or the new complete topology, never a mixture.

### Phase 3 — Resolver and API convergence

- Implement the shared Event set resolver and remove duplicate resolver SQL.
- Route Event, Assets browse/search, share, relations, and Agent consumers
  through it.
- Make rebuild asynchronous, complete status/state OpenAPI contracts, and fix
  Event asset pagination.
- Replace N+1 Event list projection with set-based queries.

Exit: every consumer returns the same authorized media set for active,
redirected, scoped, empty, invalid, and cross-owner cases.

### Phase 4 — Correction algebra

- Replace hard-label constraint interpretation with exact assignment and
  explicit cut/join semantics.
- Reimplement merge, split, move/add, and remove as atomic versioned commands.
- Define presentation inheritance and cover repair for every split/merge case.
- Add property-style tests that apply commands followed by repeated rebuilds.

Exit: all manual operations are fixed points and cannot create empty active
Events, invalid covers, redirect chains, or duplicate memberships.

### Phase 5 — Frontend convergence

- Adopt total/projected counts and the shared repository projection throughout
  Event detail and gallery flows.
- Add rebuild-run polling/status UX and truthful pending copy.
- Replace sequential bulk requests and client-only Event picker search.
- Complete timezone handling, loading/error states, and query invalidation.
- Remove the temporary write capability flag after Phase 4 gates pass.

Exit: no Event UI surface can display a count, cover, or member set from a
different scope than its gallery.

### Phase 6 — Recovery and gates

- Run the legacy-data recovery once against a fixture containing stuck claims,
  redirects, manual covers, and membership collisions.
- Regenerate OpenAPI/TypeScript and i18n artifacts with their canonical tools.
- Update architecture docs to name the revision lifecycle and shared resolver.
- Complete this plan per [README.md](README.md): extract durable decisions to
  `.agents/decisions/`, then delete this file.

Exit: all validation below passes and the seven-media reproduction publishes
one Event with seven projected members.

## Validation boundaries

### Domain invariants

- Each eligible logical media belongs to exactly one active Event.
- Every active Event has at least one canonical member.
- Every selected cover belongs to the resolved Event and projection.
- Redirects are direct, acyclic, same-owner, and target active Events.
- Rebuild is deterministic and idempotent for identical facts and corrections.
- A stale or failed run cannot advance `published_revision`.
- A successful run leaves no older pending owner revision.

### Required scenarios

- upload before and after EXIF extraction;
- capture time/GPS moving across Event boundaries;
- JPEG+RAW and Live Photo logical-media convergence;
- burst and manual Stack create/extend/remove/delete;
- trash, restore, permanent deletion, final member, and empty owner;
- automatic and manual split/merge, move/add, and remove followed by rebuild;
- title, hidden state, and cover retention across identity changes;
- unique membership migration and redirect-chain repair;
- enqueue rollback, retry, lease loss, stale recovery, pause/resume, one follower,
  and concurrent different owners;
- active, redirected, invalid, and cross-owner Event IDs in list/search/share,
  relations, and Agent tools; and
- All Repositories and single-repository count/gallery parity.

### Repository gates

Run from the repository root:

```text
task ci:architecture
task server:test
task web:test
task web:test:browser
task test
```

The Events real-service regression remains in the existing `@smoke` browser
slice. Any changed CI-relevant Taskfile target must update the corresponding
workflow path filters in the same change.

## Progress

- [x] Cross-layer audit and production-data reproduction.
- [x] Target topology and correctness contracts frozen in this plan.
- [ ] Phase 0 — Lock the failures (the production regressions are covered by
  targeted API/domain tests; the full seven-media fixture remains to be added).
- [x] Phase 1 — Revision and queue lifecycle.
- [x] Phase 2 — Atomic topology publish.
- [x] Phase 3 — Resolver and API convergence (including Browse Scope, shares,
  relations, and Agent Event asset resolution).
- [x] Phase 4 — Correction algebra (landed 2026-08-25 with the repository
  lifecycle work): merge/split/move/add/remove are atomic named transactions
  in `server/internal/event/service.go`;
  `TestRebuildOwnerPublishesAndRetainsStableIdentity` covers corrections
  followed by rebuild with redirect resolution.
- [x] Phase 5 — Frontend convergence (landed 2026-08-25): EventPicker search,
  merge/move/add-media modals, `useEventBulkActions`, and total/projected
  counts; the temporary write capability flag is removed.
- [ ] Phase 6 — Recovery and gates (OpenAPI/i18n regeneration and architecture
  docs are done; the legacy-data recovery run remains).

Validation evidence for this implementation slice:

- `task ci:architecture` passed.
- `task server:test:ci` passed with `go test -tags=sqlite_fts5 ./...`.
- `task web:test` passed (330 tests, 6 skipped).
- `task dto` regenerated OpenAPI, TypeScript, and ReDoc artifacts.
- The E2E fixture image is pinned to the repository Go toolchain (`1.25.12`);
  the previous image/toolchain mismatch was removed before the next browser run.
- 2026-08-25: phase status aligned with the committed code (correction algebra
  and frontend convergence verified against `server/internal/event` and
  `web/src/features/events`); repository gates were not re-run on this date.

## Decision log

- 2026-08-10: Events remain owner-wide; Repository Browse Scope is a read
  projection, matching the intended People topology boundary.
- 2026-08-10: Full-owner rebuild is retained until profiling justifies real
  incremental computation; unused dirty ranges are not kept as pretend windows.
- 2026-08-10: SQLite transaction rollback plus revision-checked complete-set
  replacement is the atomic publication boundary; partial in-place publication
  is forbidden.
- 2026-08-10: All Event consumers share one resolver. Similar SQL with similar
  tests is not considered the same authority.
- 2026-08-10: Manual corrections are durable partition intent and must be fixed
  points under rebuild.
