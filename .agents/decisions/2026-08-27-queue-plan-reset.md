# Decision: Reject task-granular queue convergence and pre-refactor qualification

Status: rejected — the queue architecture represented by the two superseded active plans was rejected on 2026-08-27.

## Problem

The plans `catalog-queue-throughput-convergence.md` and
`radxa-x4-large-library-qualification.md` treated the current queue split as the
target architecture. That direction moved River into a separate database but
kept the more important sources of write contention and recovery ambiguity:

- small pipeline steps remained individual River jobs and River queue worker
  counts remained the execution resource budget;
- River workers continued to mutate `catalog.db` and enqueue follow-up work;
- `catalog_work_intents`, `asset_processing_tasks`, and `repository_outbox`
  duplicated pieces of durable delivery and execution state;
- product-facing upload state depended on River job identifiers and River job
  state;
- compatibility migrations, cutover journals, and repair paths preserved an
  architecture that this pre-production repository no longer needs;
- the Radxa qualification plan proposed tuning and certifying that runtime
  before its control, execution, commit, and recovery boundaries were correct.

Those plans could produce a faster version of the wrong architecture. A long
hardware qualification run would then validate parameters that the refactor is
supposed to remove.

## Decision

Reject both plans and remove them from the active plan directory. Their
completed checklists are not accepted as evidence that the queue architecture
is complete.

Retain the useful infrastructure fact that `catalog.db` and `river.db` have
independent lifecycles. Reject the task-granular River execution model built on
top of that split.

The replacement direction was implemented through the completed Queue Control,
Execution, and Commit Convergence plan. Its durable result is recorded in
[Keep product truth in the catalog and make QueueDB disposable](2026-08-30-catalog-truth-disposable-macro-queue.md):

- `catalog.db` owns desired/applied product state and a transactional domain
  outbox;
- River is a disposable, durable macro-job control plane;
- fine-grained work runs under one in-process resource governor;
- background results reach `catalog.db` only through a bounded commit
  coordinator;
- completion is acknowledged only after artifacts and catalog state are
  durable;
- reconciliation derives missing work from catalog state without consulting
  River internals.

The Radxa X4 remains a useful eventual acceptance environment. It is no longer
an independently active architecture plan; qualification happens only after
the replacement plan's deterministic correctness, crash-recovery, and
observability gates pass.

This decision also means the implementation must supersede, rather than
silently rewrite,
[SQLite write concurrency](2026-08-25-sqlite-write-concurrency.md), whose
single-file River conclusion no longer represents the selected architecture.

## Alternatives considered

### Finish the existing throughput plan

Rejected because it optimizes River worker counts and per-task job flow while
leaving direct worker writes, duplicated lifecycle state, and uncoordinated
resource use in place.

### Add a commit coordinator beneath the existing jobs

Rejected because it would leave two execution models, preserve the large River
job/queue catalog, and turn the coordinator into a compatibility adapter rather
than the canonical write boundary.

### Keep the Radxa plan active while the architecture changes

Rejected because its workload, expected job counts, tuning matrix, recovery
checks, and pass criteria all depend on the rejected runtime. The hardware run
will be recreated as final evidence against the new contracts.

### Return River to `catalog.db`

Rejected because independent catalog and queue lifecycles are required for
fault isolation, disposable queue recovery, and separate WAL/checkpoint
telemetry. The split database is retained; the execution model is replaced.
