# Postmortem 0004: Repository leases deadlocked uploads and enrichment

## Executive summary

Every `RepositoryFS` holds a read lease on its Repository until Close, and
lifecycle operations take that Repository's write lease. The lease is FIFO,
prefers writers, cannot be re-entered, and most callers waited on it with
`context.Background()`. Since #177 (shipped in `v26.1.0-beta.1`) the storage
reconciler has taken the write lease for every Repository once a minute, so a
writer was regularly queued. Any caller that took a second lease, or waited
for SQLite's single writer while holding one, could close a cycle. The cycle
then wedged every later media handle on that Repository without a timeout. On
2026-10-28, demo preparation on `dev` hit two different cycles within hours.
An audit found two more. The fix is a mitigation: the four cycles are broken,
and every lease wait now has a bound, so any cycle that remains becomes a
retryable `ErrRepositoryBusy` instead of a wedge. Removing the lease is the
[repository access plan](../../docs/exec-plans/active/repository-access-simplification.md).

## What broke

- **Uploads stalled.** `ChunkMerger.MergeChunks` opened the merged
  destination, which took a read lease, and then opened every chunk through
  `OpenStagingFile`, which took a second read lease. When the reconciler's
  tick queued its write lease between those two acquisitions, the chunk's
  read lease waited behind the writer, and the writer waited for the
  destination's read lease. Thumbnails, ML loads, and scans then queued behind
  the same writer. The first goroutine dump showed seven goroutines in
  `repositoryAccessLock.acquire`, one of them blocked for 30 minutes.
- **Analysis stalled after the restart.** The reconciler held the write lease
  while it refreshed the Repository config cache through `UpdateRepository`.
  It did this on every tick, even when nothing had changed, so it waited for
  SQLite's single writer. The commit coordinator held that writer in
  `Transact` and was running `ApplyFaceResultsTx`. That call reached
  `applyFaceResultsTx`, which opened the Repository to cut face crops and
  waited behind the reconciler's write lease. Every catalog writer waited
  behind the coordinator. Agent replies on the same server also stalled, and
  they worked again after the patched restart.
- **Two more cycles were found by audit, not in production.**
  - `RegisterRepositoryCopy` held a Storage Location read lease and called
    `AddRepository`, which took the same root lease again. That second
    acquisition queues behind any pending Location mutation, and the mutation
    waits for the first lease.
  - A face cluster rebuild holds `clusterRebuildMu` exclusively while it waits
    for SQLite's writer, and `ApplyFaceResultsTx` took the read side of that
    mutex inside the writer's transaction.
- **Forced restarts orphaned 32 jobs.** Killing a wedged process left 32 River
  jobs `running`. River rescues stuck jobs only after an hour, so they stayed
  stuck until `river.sqlite3` was reset by hand.

## Why every net missed it

- **No documented lock order.** Nothing records an order among the Repository
  lease, the Storage Location lease, the execution governor, the upload
  `SessionManager` mutex, `clusterRebuildMu`, and SQLite's writer, so a
  reviewer had nothing to check a new acquisition against.
  [`docs/architecture.md`](../../docs/architecture.md) names the single
  writer, but not what may be held while waiting for it.
- **The rule existed only for one case.** The
  [execution budget decision](../decisions/2026-08-31-execution-budget-demand.md)
  recorded a lease and governor cycle and forbade carrying a lease across an
  admission boundary. Nothing generalized that rule to "never wait for
  anything while holding a lease", and no check enforced it.
- **The lease looks like a file handle.** Opening a sibling staging file,
  cutting a face crop, or registering a copy reads as ordinary I/O. Nothing in
  those APIs says that they wait on a FIFO writer-preferring gate. Nothing
  says that a caller already holding the gate will deadlock.
- **Tests never had a writer queued.** Storage tests exercised each lifecycle
  operation and each media path, but none queued a mutation while a handle was
  open and then nested a second acquisition. The once-a-minute reconciler
  never runs in unit tests, and short E2E runs rarely overlap its tick with an
  upload merge or a face crop.
- **Waits had no deadline.** `RepositoryFSFactory.Open` waited with
  `context.Background()`, so the cycle produced no error, no log line, and no
  failed job. It showed up only as a frozen UI. Diagnosing it took a SIGQUIT
  goroutine dump.

## Guardrails added

- [`TestOpenStagingSiblingDoesNotWaitBehindQueuedMutation`](../../server/internal/storage/staging_manager_test.go)
  queues a mutation while a merge destination is open and proves that the
  chunk opens through `RepositoryFile.OpenStagingSibling` on the root the
  destination already holds.
- [`TestReconcileRepositorySteadyStateTakesNoLifecycleLease`](../../server/internal/storage/repo_reconcile_test.go)
  proves that a pass that would change nothing takes no write lease and does
  not write.
  [`TestReconcileRepositoryGivesUpOnABusyRepository`](../../server/internal/storage/repo_reconcile_test.go)
  proves that a pass gives up on a busy Repository and does not wait for it.
- [`TestApplyFaceResultsTxRefusesDuringClusterRebuild`](../../server/internal/service/face_carryover_test.go)
  proves that applying face results inside the writer's transaction returns
  `ErrFaceClusterRebuildInProgress` instead of waiting for a rebuild.
  Inside that transaction, face crops wait at most `faceCropLeaseWait` for the
  lease.
- `RegisterRepositoryCopy` now acquires the new identity's mutation itself
  and calls `addRepository` with its locks already held, so it never takes the
  root lease twice
  ([`repo_reconnect.go`](../../server/internal/storage/repo_reconnect.go)).
- [`RepositoryLeaseWait`](../../server/internal/storage/repository_fs.go)
  bounds `RepositoryFSFactory.Open` to 30 seconds and returns
  `ErrRepositoryBusy` when the bound expires. Each new test was run against
  the code before the fix and failed there. All of them run under
  `task server:test`.
- Seven long-hold paths found by the audit are bounded by
  `RepositoryLeaseWait` but not fixed: scan turns, trash and recovery
  maintenance, batch upload streaming, the upload `SessionManager`, HTTP media
  handlers, video frame inference, and cloud import. They remain open work in
  the [repository access plan](../../docs/exec-plans/active/repository-access-simplification.md),
  together with the documented lock order, readers that never wait, and
  startup requeue of orphaned `running` jobs.
