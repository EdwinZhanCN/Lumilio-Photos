# Decision: Observe repositories through resumable facts and content Locations

Status: implemented

## Problem

Repository discovery was a path-keyed, whole-tree reconciliation pass. It held
complete-tree state in memory, coupled catalog identity to `assets.storage_path`,
and concentrated reconciliation in a long database/worker lifetime. Large or
interrupted scans could starve foreground traffic, wedge behind stale queue
state, repeat work proportional to the repository, or confuse a rename,
replacement, duplicate occurrence, and disappearance. A watcher alone could
not repair missed events or prove absence, while preserving both the old and a
new catalog model would make every correctness rule ambiguous during cutover.

## Decision

Repository originals are authoritative primary data. SQLite is a rebuildable,
durable observation and product-metadata projection; no catalog operation may
delete or overwrite an original merely because a path or change hint vanished.

ROE stores physical hierarchy in `repository_nodes`. A node has stable graph
identity; its current parent and normalized name project a path rather than
being its identity. Native file identity is a volume-scoped reconciliation hint,
not content proof. Immutable `content_objects` are unique by full BLAKE3, size,
and algorithm. SQL enforces one Asset per owner/content pair, while versioned
`asset_locations` permit every exact physical occurrence to bind to that Asset.
Different owners retain distinct Assets backed by the shared content identity.
Workers receive entity IDs and expected revisions and resolve an active Location
through `RepositoryFS` immediately before accessing media.

A full verification is a resumable protocol: capture `C0`, enumerate bounded
directory pages, capture fixed `C1`, apply change hints through that boundary,
re-verify dirty child sets, then finalize. Enumeration is never called an atomic
filesystem snapshot. Only an error-free, caught-up authoritative child set may
close an absent Location. Offline storage, access denial, cancellation, volume
replacement, malformed cursor, journal gap, watcher overflow, or incomplete
coverage may publish positive observations but must preserve unproven existing
Locations. A periodic authoritative verifier remains mandatory on every
platform.

The controller persists desired and applied epochs, run/frontier state, leases,
coverage, observations, and cursor identity. Each turn is bounded by rows,
bytes, cancellation, and wall time and performs no filesystem, hashing, native
tool, sleep, or River execution inside a database transaction. A request that
coalesces onto an already queued operation still wakes the current epoch so no
accepted request can remain stranded. River uniqueness covers queued and
running controller jobs; the same job snoozes between turns instead of creating
follower jobs. Cancellation and expired leases resume or converge from durable
state rather than relying on worker lifetime.

Catalog mutations and revision-fenced logical effects commit together in
`repository_outbox`. Bounded leased drains are at-least-once and idempotent;
crashes may repeat an effect but cannot publish a stale binding or duplicate
per-Asset downstream processing. Stable hashing validates before/after tokens
and commits by observation-revision CAS.

Native adapters are advisory and fail closed. Windows uses USN when supported
and `ReadDirectoryChangesW` for ordinary live capture; macOS uses UUID-bound,
device-relative FSEvents streams and takes cursor boundaries only after the
`HistoryDone` barrier; Linux uses a fresh inotify session identity. Cursor or
volume identity changes force authoritative verification. Live adapters ignore
Lumilio-owned marker, lock, permission, and case-probe paths; inotify also
ignores attribute-only noise so application probes cannot create a feedback
loop.

SQLite has one physical writer and a separate query-only WAL reader pool.
Background batches are capped at 256 rows and target at most 25 ms of statement
time. User-visible scans are immutable, inserted-or-coalesced operation receipts
with queued/crawling/catching-up/finalizing/terminal phases and explicit
cancellation; HTTP enqueue completion and Web mutation settlement do not wait
for background completion.

The schema cutover is forward-only and destructive for rebuildable catalog
state. Startup preflight stops incompatible work, verifies an Online Backup,
recovers or quarantines every uncertain staging byte, installs the normalized
schema, removes obsolete jobs and tables, and schedules verification for every
active repository. Failure before commit leaves the old catalog usable;
successful rollback is only the verified backup plus the old binary, which the
new schema-version gate otherwise rejects.

## Alternatives considered

**Optimize the recursive scanner and retain a scan-wide path map** — rejected
because faster enumeration does not remove O(repository) incremental work,
complete-tree memory, long transaction/worker lifetimes, or path-as-identity
ambiguity.

**Treat the watcher or filesystem journal as authority** — rejected because
events are coalesced, can overflow or expire, and cannot by themselves prove a
directory's complete child set. Native feeds reduce steady-state work; bounded
authoritative enumeration proves absence and heals missed hints.

**Keep path-owned Assets or write both catalog models during migration** —
rejected because rename and duplicate-occurrence semantics would remain
ambiguous, every read/write path would need a compatibility branch, and two
authorities could diverge. Rebuildable catalog state instead crosses one
verified maintenance boundary.

**Create one Asset for every exact physical copy** — rejected because exact
byte identity and owner identity are logical, while physical availability is a
many-Location property. Per-copy Assets duplicate metadata and downstream work.

**Use native file identity or a quick fingerprint as dedupe proof** — rejected
because native IDs can be reused or ambiguous and quick fingerprints collide.
Only stable full-content identity authorizes exact convergence.

**Insert a River job for every observation or enqueue follower controller
jobs** — rejected because durable epochs and the outbox already coalesce work;
parallel followers race one repository lease and amplify queue/database load.
One unique, snoozing controller job advances bounded turns.

**Keep the former catalog available after a successful cutover** — rejected
because ordinary fallback or dual-read behavior would conceal migration bugs
and preserve stale path authority. The verified pre-migration backup is the
explicit recovery boundary.
