# Decision: SQLite write concurrency convergence

Status: implemented — closed for implementation and diagnosis on 2026-08-25.

## Problem

SQLite permits concurrent WAL readers and one writer. A Go `sql.DB` capped at
one connection doubles as a process-local semaphore: every operation routed to
it waits for the sole connection even when the SQL is read-only. The catalog
and River queues share that one writer, so a long application transaction,
unbounded batch, or high-frequency no-op queue producer starves foreground
requests and checkpoints. The original repository-scan wedge was one instance;
the same class of defect — writer starvation, unbounded projection publishes,
checkpoint starvation — motivated this convergence.

## Decision

The live catalog owns exactly one physical read/write connection and a bounded
query-only WAL reader pool; the reader pool is the default for every
non-transactional query, including startup inspection and background planning
reads. The writer surface is explicit: a mutation either executes one bounded
statement or enters a named, measured write transaction through
`internal/db/catalogtx` — the sole application transaction capability, with
compile-time operation names and observed admission, body, commit, outcome,
and cancellation. Raw source-level transaction boundaries exist only in the
capability/driver implementation and schema migration; `architecturecheck`
closes the inventory as an AST guard.

One catalog write transaction contains database statements and in-memory
validation only. Filesystem, media, network, native-tool, sleep, and River work
happen before it begins or after it commits. Background batches stop by row
count, byte count, cancellation, and wall time; no worker holds a transaction
while processing an unbounded result set. Cancellation rolls the complete
mutation back and produces a typed failure or retryable River error; it never
leaves a durable operation in a non-terminal state.

Worker concurrency is a compute/I/O limit, not permission for concurrent
catalog writers. Periodic jobs may not generate steady no-op SQLite churn;
wakeups are signal/coalescer or durable-pending-state driven. Queue uniqueness
includes every state needed to prevent queued followers racing a running job;
self-chaining work yields between bounded pages. Checkpointing is explicit:
`wal_autocheckpoint` is disabled and a passive checkpoint monitor acts above a
WAL-size threshold, because long-lived reader snapshots can starve checkpoints
and automatic checkpoint cost must not land unpredictably on a foreground
commit.

Repository scanning is governed by the Repository Observation Engine decision
(2026-08-22): bounded, durable controller turns instead of one long worker
lifetime.

## Alternatives considered

- **Multiple ordinary catalog writers arbitrated by `busy_timeout`** —
  rejected: SQLite still serializes WAL writers, and lock races replace
  deterministic process-local ordering.
- **River in a second database** — rejected: every business transition would
  need an outbox plus coordinated backup, restore, migration, and generation
  ownership.
- **Weaken atomic domain invariants to shorten transactions** — rejected.
- **Request retries around arbitrary writes** — rejected except for
  transactions whose complete effect is idempotent and revision-fenced.
- **Hide starvation behind longer HTTP, SQLite, or River timeouts** — rejected.
