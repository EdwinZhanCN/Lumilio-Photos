# Decision: Derive execution from Catalog and keep commits typed

Status: implemented — target architecture accepted and the control-plane cutover
landed on 2026-08-31.

## Problem

The current background path represents one product transition as Catalog state,
commands, envelopes, domain-outbox rows, River arguments, generic commit intents,
and finally Catalog state again. The duplicated identity and conversion layers
make QueueDB and retry behavior appear more authoritative than product state,
and the generic commit protocol hides which typed result is being applied.

## Decision

Lumilio Photos is a durable local state machine with a resource-aware executor.
Catalog is the only product truth and derives runnable work. River is a
disposable execution projection: deleting QueueDB may affect latency and
throughput, but never retry count, terminal state, desired/applied generations,
operation state, or the next runnable work.

The canonical runtime path is:

```text
Catalog → Scheduler → WorkKey → River → Execution Governor → Processor
        → Typed Result → Typed Catalog Committer → SQLite
```

The only product-facing representations are Catalog State, Work Identity, and
Computed Result. The bounded Commit Coordinator remains the shared actor for
batching, backpressure, single-writer transactions, and post-commit ACKs, but
its public API is typed (`ApplyAssetStage`, `ApplyRepositoryTurn`,
`ApplyProjection`, `ApplyEnrichment`, and equivalent domain methods). It does
not expose `Family + Subject + Fence + Stage + Payload any`, a handler map, or
JSON-based result coalescing.

Retry and terminal semantics belong to Catalog and are derived from desired /
applied state. River attempt counters may control operational scheduling, but
cannot directly decide a product terminal state. The Domain Outbox,
Envelope/Command/DomainAdapter conversion chain, and mega-Reconciler business
knowledge are removed as their replacements become available; runnable work is
derived from Catalog state in bounded scheduler passes.

Work QoS is a Catalog fact, not a serialized command field. Request-specific
state stores one of the typed `interactive`, `background`, or `maintenance`
classes; other domains derive the class from existing facts such as a manual
repository run. The scheduler projects that class into River's native priority
metadata. Macro arguments contain only immutable work identity, and workers
restore the typed QoS from the River row before entering the execution
governor. Coalesced pending requests retain the most urgent class.

River uniqueness covers only active delivery states. A discarded row does not
block the scheduler from inserting a fresh operational attempt while Catalog
still reports desired work. Finite River attempts therefore bound one delivery
cycle rather than manufacturing a Catalog terminal error.

Every committed Catalog writer transaction emits a coalesced process-local
wake hint to the scheduler. Scheduler-owned repair transactions are excluded
to prevent feedback. The bounded periodic scan remains the correctness and
crash-recovery path; wakeups only remove steady-state polling latency.

The existing Catalog, ROE, fence/revision, SQLite single-writer, Execution
Governor, River macro execution, and crash-safe commit ACK invariants remain.

## Alternatives considered

### Keep the current message protocol and add more validation

Rejected because validation cannot remove duplicate representations or prevent
the protocol from becoming a second lifecycle authority.

### Remove River and execute directly from the Catalog scheduler

Rejected because River remains useful as a disposable, durable execution
projection for process restart, operational visibility, and macro scheduling.

### Replace the bounded coordinator with direct typed writes

Rejected because the coordinator's backpressure, batching, single-writer
serialization, and post-commit acknowledgement are required for SQLite and
crash safety.

### Preserve Domain Outbox as the durable handoff

Rejected because it duplicates Catalog-derived runnable state. The scheduler
must be able to reconstruct all execution work from Catalog without a second
product lifecycle log.

## Implementation evidence

- `commit.Coordinator` now exposes typed Catalog apply methods; its bounded
  queue remains the shared batching, single-writer, backpressure, and
  post-commit ACK actor.
- River retry/discard state is operational only. No production River error
  handler writes Catalog terminal state; terminal markers are written only by
  explicit typed Catalog transitions. Discarded jobs are excluded from the
  uniqueness boundary so still-runnable Catalog work can be re-created.
- `queue.Scheduler` derives bounded River work from pending Catalog receipts,
  desired/applied pipeline state, projection revisions, and active ROE runs.
  It projects Catalog QoS into River priority metadata, receives coalesced
  post-commit wake hints, and retains a periodic recovery pass. Replacing
  QueueDB is covered by the Catalog receipt recovery test.
- The old Domain Outbox, Envelope/Command/DomainAdapter path, generic commit
  identity, and domain-specific outbox reconciler are deleted.
- `task server:test` and `task architecture:check` pass with the SQLite FTS
  build tag.
