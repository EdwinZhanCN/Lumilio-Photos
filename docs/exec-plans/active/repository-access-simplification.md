# Repository access: a maintenance barrier instead of a read/write lease

Status: active, created 2026-10-28. Nothing in this plan has landed. The
mitigation for [postmortem 0004](../../../.agents/postmortems/0004-repository-lease-deadlocks.md)
breaks the four cycles found on 2026-10-28 and bounds every lease wait to
`RepositoryLeaseWait` (30 s). Any cycle that is still possible now fails one
operation with `ErrRepositoryBusy`; it no longer wedges the Repository. The
owner will decide whether this plan lands before or after rc.1. Until it
lands, the mitigation is what ships.

Goal: an ordinary media handle never waits for a Repository. It is either
granted at once or refused with `ErrRepositoryBusy`. The few lifecycle
operations that must exclude readers do so with an explicit maintenance
barrier. That barrier refuses new handles and waits a bounded time for the
handles already open to close. Nothing waits for any lock or for SQLite's
writer while it holds something another waiter needs, and the order of every
remaining blocking resource is written down. This plan does not redesign
storage lifecycle semantics, the catalog, or the execution governor.

## Why (2026-10-28, `dev` `ab1c3a45` plus the mitigation)

- `RepositoryAccessCoordinator` is a per-Repository FIFO read/write gate that
  prefers writers. It is not reentrant, and Storage Locations have a second
  gate of the same kind. Every `RepositoryFS` and `RepositoryFile` holds a
  read lease until Close.
  - About 25 production call sites open handles. They are in `scan`,
    `trash`, `recovery`, `sourcing`, the upload session and staging code, the
    face service, ML loaders, video frames, artifact cleanup, and the HTTP
    media handlers.
  - Fourteen lifecycle call sites take write leases:
    - add, initialize, and open a Repository;
    - adopt the primary;
    - remove and rename;
    - Repository reconnect and register-copy;
    - Storage Location reconnect and delete;
    - the reconciler.
- Writer preference means that one queued lifecycle operation makes every
  later handle wait. Combined with non-reentrancy, any nested acquisition
  deadlocks. So does any wait for another resource while a lease is held.
  Each new caller can reintroduce one of these cycles, and code review cannot
  see it. The 2026-08-31 governor cycle and the 2026-10-28 upload, face, and
  register-copy cycles all have this shape.
- The audit left seven paths that hold a handle across an unbounded or
  unrelated wait. The mitigation bounds them; it does not fix them.
  - Scan turns (`scan/walk.go`, `scan/hash.go`) keep the handle while they
    wait for the commit coordinator.
  - Trash and recovery maintenance (`trash/service.go`,
    `recovery/service.go`) do the same.
  - `AssetHandler.BatchUploadAssets` streams the HTTP request body while it
    holds a staging handle.
  - `SessionManager.openRepository` is called while the global `sm.mu` is
    held.
  - Video frame extraction (`processors/video_frames_task.go`) keeps the
    handle through inference.
  - Cloud import (`sourcing/materializer.go`) keeps the handle through
    downloads.
  - Lifecycle operations wait for SQLite's writer while they hold write
    leases.
- The catalog already has the barrier this plan needs.
  `RepositoryReachabilityMaintenance` is set by Repository reconnect and by
  Storage Location reconnect. `openOriginalReasons` and
  `RepositoryFSFactory.OpenContext` refuse it as `AdmissionBusy`, and the
  reconciler skips it. What is missing is an in-process count of open handles,
  so that the barrier can drain them instead of queueing.

## Non-goals

- Lifecycle semantics. Journaled `LifecycleReceipt`s, idempotency keys,
  startup recovery, reconnect admission, and marker refresh with guarded
  replace stay as decided in
  [storage command boundary](../../../.agents/decisions/2026-10-26-storage-command-boundary.md).
- Portable user facts. Writing ratings, albums, or people into Repository
  sidecars would revise the settled answer "promise nothing portable" in that
  record. That is a separate decision for the owner. If it is made, a sidecar
  assumes a writable Repository: a failed write is retried and is never
  redirected
  ([writable storage](../../../.agents/decisions/2026-10-28-repositories-require-writable-storage.md)).
- A catalog per Repository (see Decisions).
- Execution governor capacity and demand
  ([execution budget](../../../.agents/decisions/2026-08-31-execution-budget-demand.md)).
  This plan only enforces that handles are not held across an admission.
- Schema changes. The maintenance reachability state already exists.

## Fixed contracts

- **No write after commit.** No original or private write lands in a tree
  after a reconnect, register-copy, remove, or Storage Location change of that
  tree commits.
- **Open handles drain.** A lifecycle operation that changes a path or
  identity starts only after every handle already open on the affected
  Repositories has closed, or it returns busy within its deadline.
- **Every wait has a deadline.** No production code waits on a lease, a
  barrier, the governor, or SQLite's writer with `context.Background()`.
  `AcquireMutation`, `AcquireRootRead`, and the context-free `Open` either
  disappear or carry a bound.
- **The catalog transaction does no file I/O and takes no lock.** A commit
  coordinator `Apply` callback touches only the transaction. Files are read
  or written before the transaction or after it commits.

## Decisions (frozen 2026-10-28; extract at completion)

- **Readers fail fast; they never queue.** Opening a handle increments a
  per-Repository in-flight count under a short mutex. If the Repository is in
  maintenance, the open returns `ErrRepositoryBusy` at once. Every caller
  that opens a handle treats busy as retryable: River jobs snooze, and HTTP
  returns a busy Problem. Today only the lifecycle commands map
  `ErrRepositoryBusy` (`host_action.go`, `repository_actions.go`); Phase 2
  adds the mapping for media paths.
- **Lifecycle operations use a barrier, not a write lease.**
  - An operation that changes a path or identity enters maintenance in memory
    and in the catalog row. It then waits, with a deadline, for the in-flight
    count to reach zero, does its work, commits, and leaves maintenance.
    Those operations are remove, Repository reconnect, register-copy, and
    Storage Location reconnect and delete.
  - Because the barrier is a state and not a held lock, the operation may
    wait for SQLite's writer without forming a cycle: no reader waits on it.
  - Lifecycle operations still exclude one another through a separate
    per-Repository mutex with a bounded wait. Handles never touch that mutex.
- **Some operations need no barrier.**
  - Rename changes only the marker name, with a guarded replace.
  - Add, initialize, open, and adopt-primary act on an identity that has no
    handles yet.
  - The reconciler writes reachability and the config cache.
  - These operations take only the lifecycle mutex.
- **The central catalog stays the only runtime truth.** A SQLite catalog per
  Repository was considered and rejected:
  - Search, people, albums, events, and the Agent work across Repositories.
    They would need `ATTACH` fan-out and would lose cross-Repository
    indexes.
  - WAL mode gives no atomic commit across database files.
  - SQLite on removable or network storage risks lock and durability
    failures.
  - Migrations would run once per file, and owner partitioning would split
    across files.
- **Rejected alternatives.**
  - *Keep the lease and its bounds (the mitigation).* Every new caller can
    still close a cycle. The bound turns each cycle into a 30-second stall
    and a busy error instead of a correct answer.
  - *Make the lease reentrant by carrying it in the context.* That hides the
    nesting and still cycles with SQLite's writer and the governor.
  - *No in-process exclusion, as in Immich and PhotoPrism, where in-flight
    work fails after a path moves.* Lumilio promises that no write lands in
    a tree after reconnect or remove commits, so open handles have to drain.
    The barrier keeps that promise with readers that never wait.

## Execution phases

### Phase 0 — Lock the failures

Before any refactor, add one deterministic test for each audited path that
can still stall. Each test queues a lifecycle mutation while the path holds
its handle and asserts that the mutation completes. Each test must fail or
time out on the current code:

- a scan turn waiting for the commit coordinator;
- trash maintenance and recovery maintenance;
- `BatchUploadAssets` while it streams the request body;
- `SessionManager` while `sm.mu` is held;
- video frame inference;
- cloud import downloads;
- a lifecycle operation waiting for SQLite's writer while a handle waits for
  that operation.

Add one stress test. It runs upload merge, a scan, face apply, and the
reconciler tick against the same Repository, with reconnect and remove in
parallel, and asserts that nothing ever blocks past its deadline.

### Phase 1 — Lock order and bounded waits

- Write the order of blocking resources into
  [`docs/architecture.md`](../../architecture.md). The resources are runtime
  ownership, the lifecycle mutex, the maintenance barrier, governor
  admission, the upload session mutex, `clusterRebuildMu`, and SQLite's
  writer. Also write the rule: never wait while holding something another
  waiter needs.
- Delete the context-free acquisition APIs.
- Add a registered architecture behavior test that rejects
  `context.Background()` passed to a lease, barrier, or writer wait in
  non-test code.

### Phase 2 — Handles that never wait, and the maintenance barrier

- Replace `repositoryAccessLock` with a per-Repository gate holding a
  maintenance flag and an in-flight count, plus a lifecycle mutex. Treat
  Storage Locations the same way: the barrier on a Location covers its child
  Repositories.
- Map `ErrRepositoryBusy` from a handle open to a retry. River workers
  snooze, and media handlers return the busy Problem. Do this before any
  open stops waiting.
- Move the barrier operations listed in Decisions onto the gate.
- Move rename, add, initialize, open, adopt-primary, and reconcile onto the
  lifecycle mutex alone.
- Keep the catalog `maintenance` row in step with the in-memory flag, so
  that startup recovery still finds interrupted operations.

### Phase 3 — Nothing waits inside the catalog transaction

- Cut face crops before `ApplyFaceResultsTx` and pass in the prepared bytes.
  Remove the `RepositoryFS` open inside the transaction.
- Replace `clusterRebuildMu`, which is held while waiting for the writer,
  with ordering that SQLite's writer enforces. For example, the rebuild runs
  as writer transactions, and apply checks a rebuild generation.
- Audit every `commit.Coordinator` `Apply` callback for file I/O and for any
  wait.

### Phase 4 — Short handle lifetimes

- Close handles before any wait on the commit coordinator, the governor, the
  network, inference, or an HTTP body. This covers each path in Phase 0.
- Make `SessionManager` open handles outside `sm.mu`.

### Phase 5 — Queue recovery after a forced stop

- At startup, once runtime ownership proves that no other process owns the
  QueueDB, return River jobs left `running` to `available`. Do not wait an
  hour for River's stuck-job rescue.
- Record this behavior beside the
  [disposable macro queue](../../../.agents/decisions/2026-08-30-catalog-truth-disposable-macro-queue.md)
  decision.

## Validation boundaries

- Every Phase 0 test and the stress test pass. Under the stress test, a
  goroutine dump shows no goroutine waiting for a Repository or a Storage
  Location.
- Each lifecycle operation either completes or returns busy within its
  deadline while handles are open. No original or private write lands in a
  tree after its reconnect, remove, or register-copy commits.
- The architecture test rejects an unbounded wait, and the lock order in
  `docs/architecture.md` names every blocking resource that remains.
- After a forced kill during processing, no River job stays `running` once
  startup completes.
- `task server:test`, `task atlas:check`, and the `storage-recovery` E2E
  slice are green.
