# Decision: Events are an owner-wide derived topology of logical media

Status: implemented. Topology and revision contracts frozen 2026-08-10;
correction algebra and frontend convergence landed 2026-08-25. Owners:
`server/internal/event` and `web/src/features/events`.

## Problem

Events were rebuilt as a side effect of other pipelines, published with
partial membership, and read through handwritten SQL per consumer. Late EXIF,
logical-media rewrites, and Stack changes did not invalidate the topology.
Failed River jobs could report success while retaining claimed dirty rows.
Repository Browse Scope leaked into identity. Manual merge/split/move/add/
remove could be undone by the next rebuild. Header counts, covers, and
galleries could disagree.

## Decision

Canonical Event construction is owner-wide and repository-independent.
Membership atoms are logical `media_item_id` values, never a snapshot of
physical asset IDs. Repository Browse Scope is an `AND` projection applied
after owner and Event authorization. It never changes Event identity,
reconciliation, or rebuild input. Detail responses expose canonical and
projected counts from the same resolved set.

`source_revision` records known fact changes; `published_revision` records the
revision represented by published Events. `source_revision > published_revision`
is the only definition of pending rebuild work. `MarkEventFactsChangedTx` is
the single factual invalidation boundary and must run in the same catalog
transaction as the media, Stack, metadata, trash, or correction mutation.
Event user state (title override, cover override, hidden) is a
reconciliation input that every rebuild snapshot copies and republishes, so a
`PATCH /api/v1/events/{id}` advances `source_revision` too; otherwise a
rebuild prepared before the edit passes the revision check and silently
reverts it (found 2026-09-23 through a flaky `events.spec.ts` while late EXIF
kept a rebuild in flight).
`event_dirty_ranges` is a recovery ledger, not an incremental computation
window. Rebuild work uses the closed `rebuild_projection_batch` macro.

One rebuild reads candidates, constraints, and presentation from one SQLite
snapshot, computes without holding a writer, then replaces the complete owner
membership set in delete-before-insert order inside a revision- and
lease-checked write transaction. A stale revision does not publish. A failed
worker releases its lease and remains retryable; zero newly claimed work is
never evidence that an earlier failed publish succeeded. Empty owners and
Events that lose their final member converge to the terminal `retired` state.
Redirects are direct, acyclic, same-owner, and target active Events.

Manual corrections are exact logical-media assignments plus explicit join/cut
boundaries, not chronological hard labels. Merge, split, move/add, and remove
are atomic versioned commands. `command → rebuild → rebuild` is a fixed point.

Every Event consumer — list, detail, Assets browse/search, shares, relations,
Agent tools, and the frontend picker — uses one owner-aware resolver.
`POST /api/v1/events/rebuild` enqueues work and returns `202 Accepted`; it
does not publish synchronously. Frontend Event server state stays in TanStack
Query. `useEventRebuildStatus` polls only while a revision is pending.
Header count, cover, gallery, selection, viewer navigation, and share preview
use the same resolved repository projection.

ML, faces, OCR, embeddings, and LLM output do not determine Event membership.
Incremental time-window rebuilding is out of scope until a benchmark justifies
it.

## Alternatives considered

**Make Event identity repository-scoped** — rejected. The same logical media
can occur in more than one Repository; identity follows the owner.

**Use ML or semantic similarity for membership** — rejected. Events are a
deterministic time/distance topology with user corrections. Semantic
organization is a different product.

**Keep unused dirty ranges as pretend incremental windows** — rejected. They
are not computation windows and invite a second lifecycle authority.

**Publish membership in place, or insert before delete** — rejected. Partial
publication and the global `event_media_items.media_item_id` unique index
produced mixed topologies and unique collisions.

**Treat River attempt exhaustion or a zero-claim retry as product success** —
rejected. Catalog source/published revisions are the lifecycle authority.

**Handwritten Event membership SQL per consumer** — rejected. Similar SQL with
similar tests is not the same authority.

**Hard-label chronological segments as corrections** — rejected. Assigning one
media must not absorb its entire time window unless the selected set requires
it.

**Synchronous rebuild in the HTTP handler** — rejected. Rebuild is owner-wide
compute. The API enqueues and reports run state.

**Nil owner scope when resolving an Event** — rejected. An administrator may
omit owner on generic library reads, but every Event-filtered query carries
the Event's authenticated owner.
