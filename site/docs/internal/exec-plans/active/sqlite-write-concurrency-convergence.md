# SQLite Write Concurrency Convergence

Status: active — implementation and diagnostic scope complete as of 2026-08-25;
formal multi-run/platform certification is deferred. The 2026-08-22
project-wide inventory, reader-default boundary,
highest-cardinality transaction rewrites, queue no-op suppression, and explicit
checkpoint policy are implemented with deterministic regression coverage. The
2026-08-23 instrumentation and controller slices now add a closed named
transaction catalog, driver-level standalone-statement and cursor telemetry,
bounded HDR reports, `DBStats` reconciliation, an AST inventory guard, the
deterministic 23-queue pressure corpus, disk watchdog, checkpoint/integrity
gates, and real-browser/recovery probes. Qualification-shaped smoke and two
full-duration dress rehearsals have exposed two project-wide queue races plus
sustained-write Online Backup starvation and a writer-starvation defect in the
Repository lifecycle coordinator, followed by an unbounded Location projection
publish; the fixes have deterministic coverage and the Location rewrite now has
combined runtime smoke evidence. A pre-absence-fix idle baseline and one
mixed-load repeat are green, but the next repeat exposed one more Repository
transaction quantum and a checkpoint-observation boundary. The rebuilt
post-fix smoke, idle baseline, and one full repeat were green, but the next
repeat proved the fixed-size absence cap still ignored the controller's time
budget. The count-plus-time-budget repair has deterministic coverage and its
fresh rebuilt smoke and baseline are green; three uncontaminated full mixed-load repeats,
adversarial profiles, 30-trial startup states, complete recovery matrix, soak,
and remaining platform rows are still pending. The plan remains active until
that evidence proves the same contract at runtime and the remaining maintenance
transactions have measured budgets.

Goal: keep initialization, authentication, setup, library browsing, status,
and other foreground requests responsive while any combination of River jobs
and application maintenance writes is active. SQLite remains the embedded
catalog and preserves atomic business-state/job transitions, but no accidental
read on the single writer, unbounded transaction, external I/O inside a
transaction, checkpoint spike, or background queue flood may monopolize the
runtime.

## Current evidence

- SQLite officially permits concurrent WAL readers and one writer. A Go
  `sql.DB` capped at one connection also acts as a process-local semaphore:
  every operation routed to it waits for the sole connection even when the SQL
  is read-only.
- `internal/db.Open` owns one writer connection and four `mode=ro`,
  `query_only=on` readers. The default generated-query surface now routes
  read-only statements to readers and mutations/unknown statements to the
  writer. A generated-statement audit compares the application classifier with
  SQLite's authoritative `sqlite3_stmt_readonly` result.
- The initial inventory found 55 explicit `BeginTx` calls across 27 files, plus
  implicit writes and River catalog transactions. It identified backup, Vec1
  training, Location and duplicate graph rebuilds, Event publication, OCR,
  Face clustering, Agent bulk effects, and periodic River producers as the
  highest-cardinality or highest-frequency risks; those paths now plan through
  readers, publish set-wise or in bounded turns, and avoid external work while
  holding the writer.
- Application tables and River queues share the one catalog and writer. This
  preserves atomic `InsertTx` transitions, but amplifies any long application
  transaction or high-frequency no-op queue producer.
- The previous repository-scan wedge is now isolated by ROE's bounded turns and
  reader routing. The audit also found setup/bootstrap phase reads, runtime
  storage status, repository/Storage Location lists, and Host Action lists that
  performed reconciliation or expiry writes from foreground read paths. Those
  paths are now strictly reader-backed; deterministic tests hold or queue the
  writer while initialization/status reads, default reads, backup, and Face
  derived convergence continue. The full mixed-load campaign is still
  outstanding.

## Progress on 2026-08-22

- Added a fail-closed default query router, production reader injection for
  queue/Event/classifier/Agent/storage/index/processor planning, and an
  architecture check that protects the one-writer/four-reader topology and
  rejects known raw-writer read wiring.
- Split projection refresh from presentation: bootstrap `Phase` derives
  incomplete gates without persisting, setup runtime status and repository
  lists consume the latest reader projection, boot/background reconciliation
  owns disk-driven writes, and Host Action list expiry is materialized without
  a GET-side update. The architecture guard rejects reintroducing foreground
  reconciliation on these paths.
- Moved Vec1 training and model construction, Event snapshot planning, Face
  vector clustering, backup source reads, Location identity work, duplicate
  graph construction, OCR serialization, and Agent bulk preparation outside
  writer transactions. Location, duplicate, OCR, Event membership, Face
  publication, and Agent membership changes are set-based or bounded.
- Face rebuilds now yield the writer between derived publications while a
  service-level rebuild guard prevents them from overwriting concurrent manual
  corrections. Event owner publication remains a deliberate complete-set
  atomic exception, with Event rows, memberships, covers, retirements, and
  redirects applied through a constant number of set-wise statements plus
  revision/lease CAS.
- Repeatable River jobs no longer use `ByPeriod`. Revisioned/outbox-backed or
  immutable work is unique over all active states; mutable Event/Location/Stack
  snapshot projections reserve at most one queued follower by excluding only
  `running` from uniqueness, so a mid-run factual change cannot be lost.
  Repository-outbox, Event-maintenance, and OCR-index recovery use cancellable
  reader probes to arm atomic hints. River periodic constructors remain
  non-blocking, and ROE outbox backlog pages snooze the same active job instead
  of inserting followers.
- Disabled `wal_autocheckpoint`, added a single explicit passive checkpoint
  monitor above the WAL-size threshold, remembered the fully checkpointed WAL
  file version so retained idle allocation does not retrigger maintenance,
  added writer wait diagnostics and slow `DB.WithTx` logging. Online Backup
  uses a query-only reader.
- Remaining measured-risk work: full mixed queue/foreground load, restore and
  generation shutdown, real Desktop/browser startup, direct-`BeginTx` coverage
  beyond `DB.WithTx`, and latency evidence for deliberate maintenance scopes
  such as complete Event publication, stack topology edits, and repository
  relocation/removal. These are not known initialization blockers, but they are
  not yet eligible to be declared within the target p99 budget.

## Progress on 2026-08-23

- Added `internal/db/catalogtx` as the sole application transaction capability.
  Every production `BeginTx` now names a compile-time operation and records
  writer/reader admission, body, commit, total lifetime, terminal outcome, and
  cancellation. The only raw source-level transaction boundaries are the
  capability/driver implementation and schema migration; driver-owned River or
  infrastructure transactions receive the bounded
  `catalog.driver.raw_transaction` label.
- Replaced the process-global live-catalog driver registration with per-catalog
  observed connectors. Standalone writer statements distinguish pool admission
  from driver execution; returned rows retain the connection until close and
  record cursor lifetime plus balanced open/current/peak counters. Generated
  reads, generated `Exec`, generated DML `RETURNING`, checkpoints, Vec1 state
  maintenance, Event/Agent/classifier/storage maintenance, and unknown fallback
  statements all have fixed labels. No SQL text, query argument, or entity ID is
  retained as a metric key.
- Added fixed-range 1 microsecond–2 minute HDR histograms with three significant
  digits and JSON summaries for transaction and statement p50/p95/p99/p99.9/max,
  outcomes, cancellations, overflow, and rows. `DB.TelemetrySnapshot` aligns
  these distributions with writer/reader `database/sql.DBStats`; a deterministic
  test holds the only writer connection and proves the same deadline appears in
  the named admission histogram and aggregate `WaitCount`/`WaitDuration`.
- Added an AST architecture inventory that rejects raw production `Begin`,
  `BeginTx`, and standalone `Exec` calls even when formatted across lines. It
  accepts only literal `catalogtx.Operation` calls, transaction-local statements,
  and narrow documented infrastructure exemptions. Its fixture tests prove raw
  database/sql, unnamed begin, dynamic-operation, and raw standalone-write
  violations fail.
- The first formal smoke attempts exposed two harness-visible convergence
  defects rather than `SQLITE_BUSY`: derived ML work could retry forever after
  repository churn removed the current asset occurrence, and River could report
  a transient zero-job gap while durable repository/Event/OCR work had not yet
  been re-enqueued. Content-derived jobs now carry an expected content revision,
  stale lifecycle work completes terminally, missing current derived input
  snoozes without consuming retry budget, and pressure drain requires a
  continuous 65-second zero-pending window covering the slowest durable-work
  recovery probe.
- Formal smoke `smoke-formal-20260823c` completed 960 retry-free steady requests,
  all 23 queues, 1,398 River jobs, one injected/recovered Face failure, integrity
  checks, and five page-complete passive checkpoints with no unexpected HTTP,
  SQLite, or queue failure. Its sole red gate revealed that WAL file mtime is not
  checkpoint lag: River legitimately renews its leader lease every five seconds,
  while SQLite normally retains and reuses WAL allocation. Catch-up evidence now
  follows SQLite's authoritative checkpoint columns and requires two distinct
  post-drain checkpoint turns with `log_pages == checkpointed_pages`; an
  intervening incomplete checkpoint resets the sequence.
- Formal smoke `smoke-formal-20260823g` was the first complete all-green run:
  zero request failures, all 23 queues and 1,290 jobs converged, the injected
  Face failure recovered exactly once, integrity was clean, and two distinct
  page-complete post-drain checkpoints passed. It also made a remaining writer
  hotspot visible: 1,562 steady `catalog.statement.unknown_write` samples and
  1,112 writer-pool waits totaling 5.154 seconds. Safe failed-statement
  diagnostics identified River's `NotificationGetAfter` polling query without
  logging SQL text, parameters, IDs, or unbounded metric labels.
- River's SQLite listener had shared its executor's sole writer-backed
  `*sql.DB`, so the 50 ms notification read loop repeatedly acquired the writer
  pool even though the statement was read-only. Queue construction now uses a
  split River driver: executor, mutations, and River transactions retain the
  writer; the listener uses the query-only pool. A held-writer regression test
  proves notification reads complete through the reader pool and never enter
  writer telemetry.
- The faster reader-backed listener then exposed a durable coalescer race in
  `smoke-formal-20260823h`: a repository-outbox drain could complete after
  consuming the currently visible prefix while additional durable effects were
  committed immediately afterward, and the former edge-triggered 30-second
  recovery suppression left those effects temporarily unowned. Repository,
  Event, and OCR recovery probes are now level-triggered over **unowned**
  durable work: an active River job suppresses duplicate wakeups, while the
  first probe after that job exits re-arms the process-local atomic coalescer.
  Deterministic tests cover active-job ownership and fast-completion re-arming.
- Rebuilt formal smoke `smoke-formal-20260823i` passes the full contract with
  zero request/SQLite/queue failures, all 23 queues and 1,513 jobs drained,
  exactly one recovered Face injection, clean integrity, and two page-complete
  checkpoints (13/13 and 8/8 pages). Relative to run `g`, writer-routed unknown
  statements fell from 1,562 to 835, writer waits from 1,112 to 448, cumulative
  wait from 5.154 seconds to 0.942 seconds, and foreground writer-admission p99
  from 64.543 ms to 7.283 ms. Initialization p99 was 13.007 ms, catalog-read
  p99 22.959 ms, status p99 21.359 ms, foreground-write end-to-end p99
  15.223 ms, and every named background writer-hold distribution remained
  within 25 ms.
- The first 1,200-media/30-minute qualification dress rehearsal
  `qualify-linux-arm64-20260823-1` completed all 57,600 retry-free steady
  requests with zero failures or missed dispatches and drained 22 queue
  families cleanly. It exercised all 23 queues and 13,430 jobs; the sole red
  fact was one discarded forced `db_backup` job after three exact 60-second
  context deadlines. Final client p99 was 9.374 ms initialization, 43.019 ms
  catalog read, 55.921 ms status, and 10.356 ms foreground write; initialization
  max was 397.576 ms. This run is not one of the final three because controller
  unit tests briefly overlapped its host measurement window.
- The long run revealed Online Backup starvation rather than writer-lock
  starvation. SQLite restarts an incremental backup when another source
  connection commits; copying 256 pages and yielding for 10 ms could therefore
  lose progress indefinitely under the sustained writer stream. The backup now
  pins one bounded query-only WAL read snapshot only for the page-copy phase,
  then releases it before validation, hashing, and fsync. A deterministic
  16 MiB test with a writer committing every 2 ms times out when that snapshot
  is deliberately removed and completes in under one second with the fix while
  at least ten writes commit concurrently. The River worker also declares a
  30-minute size/I/O ceiling instead of inheriting River's unrelated 60-second
  default; progress is guaranteed by the snapshot rather than the longer cap.
- Rebuilt smoke `smoke-backup-snapshot-20260823a` proved the new Online Backup
  completed under active writes in 115 ms and that a page-complete passive
  checkpoint followed in 22 ms. Its next red gate was independent of SQLite:
  selective `/reprocess` waited the full 30-second HTTP deadline for a
  Repository mutation lease while media readers kept entering. The
  context-aware coordinator had implemented cancellation by polling
  `sync.RWMutex.TryLock`; unlike a blocking `Lock`, that does not register a
  waiting writer, so later readers could bypass it indefinitely. Repository
  and Storage Location gates now use a context-aware FIFO read/write lease:
  once a mutation is queued, new readers wait behind it, cancellation removes
  the exact waiter, and leading readers still batch. A regression test first
  failed against the old coordinator by observing the later reader overtake
  the writer, then passed ten consecutive runs after the change; cancellation
  cleanup and a focused race-enabled run are also green.
- `smoke-fair-lease-20260823b` passed all gates and proved the bounded backup,
  FIFO coordinator, all 23 queues/1,527 jobs, controlled failure, integrity,
  and checkpoint catch-up together. It also showed that fairness alone was not
  sufficient for user latency: `/reprocess` improved from a 30-second timeout
  to success but still waited about 9.96 seconds for the current long media
  reader. Reprocess request/worker sections publish catalog and River intent;
  they do not mutate repository files. `BeginRepositoryWork` now retains the
  parent Storage Location identity read lease and durable repository-activity
  compare-and-swap, but no longer takes the filesystem mutation lease. The
  catalog CAS still serializes against removal, root mutation still blocks the
  enqueue, and workers re-resolve the current Location before execution. A
  regression first failed with a 100 ms deadline while one `RepositoryFS` was
  open, then passed while removal remained fenced.
- Final combined smoke `smoke-enqueue-intent-20260823a` passes with zero request
  failures, all 23 queues and 1,574 jobs complete, no attention/discarded work,
  one injected/recovered Face failure, clean standalone integrity, and two
  page-complete post-drain checkpoints (9/9 and 12/12). The now-explicit
  producer timer measured `/reprocess` at 1.912 ms, versus about 9.96 seconds in
  the fair-lock-only run. Steady p99 was 13.247 ms initialization, 26.687 ms
  catalog read, 16.815 ms status, and 9.503 ms foreground write; foreground
  writer admission p99 was 8.455 ms, writer wait fell to 456 events/0.832 s,
  every observed background hold distribution passed, and WAL allocation
  stayed below 0.08 GiB.
- The first true idle baseline `baseline-linux-arm64-20260823-1` completed the
  full five-minute warm-up and 30-minute steady window with exactly 57,600
  successful requests and no timeout, failure, missed dispatch, queue churn,
  resource drift, or discarded work. Steady p99 was 22.655 ms initialization,
  28.415 ms catalog read, 24.815 ms status, and 24.399 ms foreground write.
  Its final gate correctly exposed two observability gaps instead of a product
  request failure: SQLite interval coverage was 0.831, and 10,800 real
  rating/like writes were classified as generated statements rather than the
  named foreground operation.
- The 5-second telemetry ticker and 30-second checkpoint ticker were phase
  aligned. Publishing the latest-only runtime file after checkpoint work could
  overwrite the adjacent full five-second interval before the one-second host
  sampler observed it, losing almost exactly one interval in six. Checkpoint
  turns now update only `lastCheckpoint`; the single telemetry cadence publishes
  it on its next turn. A deterministic scheduling test failed on the former
  second publication and passes ten consecutive runs plus race coverage after
  the change. Rating, like, combined rating/like, and description edits now use
  the named `asset.status.mutate` transaction instead of the generic generated
  writer path; a real-catalog regression failed with no named sample before the
  change and passes with four named commits afterward.
- Rebuilt smoke `smoke-telemetry-cadence-20260823b` proved both repairs in the
  production bundle: all foreground requests succeeded, telemetry passed its
  short-window boundary-adjusted coverage floor, and `asset.status.mutate`
  recorded 390 samples with 8.335 ms admission p99. Its only red gate exposed a
  separate bounded-work defect: one 32-event native change-feed transaction
  held the sole writer for 82.852 ms (81.957 ms body), above the 25 ms
  background budget.
- Native change catch-up now decouples its commit quantum from the larger
  directory enumeration page: each feed turn requests at most four events and
  yields the writer before the next turn. The 100-event coalescing regression
  first observed an eight-event request and failed, then passed ten consecutive
  runs, the complete controller package, and race coverage with the four-event
  cap. Rebuilt smoke `smoke-change-quantum-20260823c` passes every gate with
  zero request failures, 363 foreground writer samples at 5.835 ms admission
  p99, 22 background operation classes without a violation, and two distinct
  complete checkpoints. The real change-batch distribution covered seven
  transactions with 1.897 ms hold p99/max, down from 82.879 ms.
- Formal qualification attempt `qualify-linux-arm64-final-20260823-1`
  completed the 30-minute steady window with 0.997 telemetry coverage, two
  page-complete post-drain checkpoints, bounded WAL slope, and no request or
  queue correctness failure, but it is not qualification evidence. A corpus
  path disappeared between `WalkDir` enumeration and `DirEntry.Info`, causing
  the host disk sampler to abort before final summary publication. The sampler
  now ignores only `fs.ErrNotExist` at those two race boundaries, retains every
  other filesystem error, and has deterministic disappearing-entry coverage.
- Offline analysis of that otherwise useful run found the next product defect:
  `location.rebuild.publish` executed 212 times and held the sole writer for
  10.511 ms p50, 23.535 ms p95, 29.375 ms p99, and 33.599 ms max, failing the
  25 ms background budget. Every GPS metadata completion had enqueued a rebuild
  whose writer transaction recursively scanned `active_asset_occurrences`,
  aggregated the entire Repository/owner scope, deleted the prior projection,
  recreated it, and repopulated all memberships. Active-state uniqueness could
  coalesce callers but had no durable source revision, so simply suppressing a
  running follower would also risk losing a concurrent fact.
- Migration 13 now makes Location clusters an explicit eventually consistent
  projection. Asset GPS/owner/lifecycle facts, Location binding/unbinding, and
  Repository node structure advance a per-Repository/owner `source_revision`
  in the source transaction; idempotent fact writes are trigger-fenced. A
  concrete worker reads revision, desired members, stored clusters, and stored
  memberships from one query-only WAL snapshot, computes a stable-cluster-ID
  diff in Go, and applies at most 16 indexed membership statements plus one
  cluster mutation per transaction. One River turn performs at most eight
  apply transactions before snoozing the same active job; publication is a
  short revision CAS and optional resolver enqueue. A query-only scheduler
  re-arms dirty scopes only when no concrete worker owns them, closing the
  insert-versus-finalize race without follower churn. The former full-scope
  delete, recursive aggregate, and bulk-membership writer query APIs were
  removed so the old transaction shape cannot be reused.
- Deterministic tests prove 130 same-geohash members stop after exactly
  `8 * 16 = 128` membership changes in the first turn, a 131st source fact
  committed while snoozed forces re-planning, and convergence publishes only
  when `source_revision == published_revision`. Trigger tests cover idempotent
  metadata retry, owner transfer, Location unbinding, and node tombstoning;
  recovery tests prove active concrete/scheduler jobs suppress duplicate wakes,
  and a River worker test proves bounded continuation snoozes instead of
  completing. These tests pass ten repeated runs, focused race coverage, the
  full Server suite, Web suite, Desktop race suite, architecture/Compose/lock
  gates, and a stable second SQL generation hash.
- Post-rewrite smoke `smoke-location-convergence-20260823a` passes every gate
  with zero request failures and all 23 queue classes drained without an
  attention item or duplicate-effect candidate. The old
  `location.rebuild.publish` writer-hold p99 fell from 29.375 ms to 1.578 ms;
  the new bounded `location.rebuild.apply_batch` hold p99 is 0.348 ms. All 23
  observed background operation distributions pass their budgets. Foreground
  writer admission p99 is 13.335 ms, initialization p99 is 16.287 ms, and the
  three other steady request classes also pass without a timeout or missed
  dispatch. The catalog reports `quick_check=ok`, no foreign-key violation,
  two distinct page-complete post-drain PASSIVE checkpoints, a bounded WAL
  slope, and successful recovery from the injected Face failure. This smoke is
  diagnostic rather than qualification evidence because each request class has
  fewer than 10,000 steady samples.
- Fresh idle baseline `baseline-linux-arm64-20260823-3` is qualification
  eligible on the production Linux/arm64 image hosted by an AC-powered Apple
  M2 Pro through OrbStack. Initialization and catalog reads each have 18,000
  steady samples; foreground writes and status each have 10,800. All four
  classes report zero failures, timeouts, missed dispatches, or overflow, with
  p99 of 13.991 ms, 21.855 ms, 9.783 ms, and 10.911 ms respectively. Foreground
  writer admission p99 is 2.627 ms, SQLite telemetry coverage is 0.997, retained
  WAL allocation is constant at 21,873,112 bytes, both post-drain checkpoints
  are page-complete, and catalog integrity passes. The image ID, Server binary,
  runtime configuration hashes, SQLite build, corpus, arrival schedule, and
  disabled-retry policy are recorded for strict mixed-run comparison. It must
  be renewed after the subsequent absence-cap Server rebuild before the final
  comparison campaign.
- Full mixed repeat `qualify-linux-arm64-final-20260823-2` passed all gates with
  zero request failures, at least 10,800 samples in every foreground class, all
  23 queues drained without attention or duplicate-effect candidates, 0.997
  SQLite telemetry coverage, and page-complete checkpoints. Initialization,
  catalog-read, foreground-write, status, and writer-admission p99 were 11.791
  ms, 40.991 ms, 19.135 ms, 55.487 ms, and 6.843 ms. Location apply/publish
  writer-hold p99 remained bounded at 3.593/1.407 ms. This run is useful defect
  evidence but cannot count toward the final three because a later repeat
  required a Server binary change.
- Repeat `qualify-linux-arm64-final-20260823-3` kept all foreground classes
  retry-free and converged every queue, but correctly failed two gates. Of 13
  steady `repository.observe.finalize_absence` samples, the largest held the
  writer for 27.055 ms, exceeding the 25 ms background p99 budget. The
  transaction used the generic 48-row Repository page even though each absence
  child performs several revision-fenced writes. Absence pages were initially
  capped independently at 16 while retaining their persisted keyset cursor and
  lease; a behavioral test fails against the old 20-row turn and passes with
  that first cap. The later full-repeat evidence below tightens this further.
  The same repeat physically completed a second PASSIVE checkpoint at
  17:35:00 with `busy=0` and 9/9 pages copied, but its result became visible in
  `sqlite-runtime.json` on the 17:35:05 telemetry turn. The 65-second host
  deadline raced its one-second poll and reported only the earlier checkpoint.
  The observation window is now 75 seconds, covering two 30-second checkpoint
  turns, telemetry publication, and scheduler margin without weakening the
  requirement for two distinct page-complete checkpoints.
- Rebuilt post-fix smoke `smoke-absence-catchup-20260823a` passes every gate on
  the new Server image. Its 300 initialization, 300 catalog-read, 180
  foreground-write, and 180 status steady samples have zero failures, timeouts,
  missed dispatches, or histogram overflow; their p99 values are 17.087 ms,
  47.231 ms, 28.863 ms, and 29.135 ms. Foreground writer admission p99 is 5.467
  ms, SQLite telemetry coverage is 0.836, Location apply/publish writer-hold
  p99 remains bounded at 2.429/2.929 ms, and all 22 observed background
  operation distributions pass. All 23 queue classes drain with no attention
  or duplicate-effect candidate, `quick_check=ok`, foreign-key violations are
  zero, and controlled Face failure recovery passes. The 75-second catch-up
  window independently observes two page-complete PASSIVE checkpoints 30
  seconds apart and completes in 58 seconds, proving the former publication
  race is closed. This short corpus did not execute a final-absence turn, so
  the cap's runtime distribution remains a mandatory observation in the fresh
  full qualifications in addition to its red/green and race regression.
- Fresh post-fix idle baseline
  `baseline-linux-arm64-converged-20260823-2` is qualification eligible against
  image `sha256:bb4f0cb3546e...` and Server binary
  `8a594e152d013a...`. Initialization and catalog reads each record 18,000
  steady samples; foreground writes and status each record 10,800. All four
  have zero failures, timeouts, missed dispatches, or overflow, with p99 of
  19.839 ms, 23.215 ms, 8.919 ms, and 10.031 ms. Foreground writer admission
  p99 is 2.036 ms, SQLite telemetry coverage is 0.997, retained WAL allocation
  remains constant at 22,375,752 bytes across 360 samples, and both post-drain
  checkpoints are page-complete. Catalog quick-check and foreign-key integrity
  pass with no remaining queue attention or duplicate-effect candidate. This
  run freezes the dirty-tree hash, configuration hashes, SQLite source/build,
  corpus manifest, disabled-retry arrival schedule, Linux/arm64 container
  runtime, and AC-powered Apple M2 Pro host row for the final mixed campaign.
- Final mixed qualification repeat 1/3
  `qualify-linux-arm64-converged-20260823-1` passes every gate against the
  frozen image. Its 18,000 initialization, 18,000 catalog-read, 10,800
  foreground-write, and 10,800 status samples have zero failures, timeouts,
  missed dispatches, or overflow; p99 is 9.719 ms, 40.447 ms, 14.079 ms, and
  57.055 ms. Writer admission p99 is 5.995 ms and SQLite telemetry coverage is
  0.997. The fixed `repository.observe.finalize_absence` executes 14 steady
  turns with writer-hold p99/max 19.503 ms, below the 25 ms budget; all 25
  observed background operation distributions pass. All 23 queue classes
  drain without attention or duplicate-effect candidates, the one controlled
  Face failure recovers, quick-check and foreign keys are clean, WAL has no
  final positive slope, and two distinct page-complete checkpoints are
  observed. This was one of the required three consecutive uncontaminated
  repeats, but became historical evidence after the following repeat required
  another Server change.
- The next repeat `qualify-linux-arm64-converged-20260823-2` passes every
  foreground, writer-admission, queue, controlled-failure, WAL/checkpoint, and
  integrity gate but correctly fails the background budget. Its 12 steady
  `repository.observe.finalize_absence` samples have 43.679 ms writer-hold
  p99/max; 41.855 ms is transaction body and only 3.725 ms is commit, proving
  this is accumulated statement work rather than checkpoint/fsync overhead.
  The four request classes still record the required 18,000/18,000/10,800/
  10,800 retry-free samples with p99 11.863/39.455/14.215/54.431 ms, all 23
  queues converge, controlled Face recovery and integrity pass, and two
  page-complete checkpoints are observed. The former 16-row cap bounded count
  but, unlike directory apply, did not enforce the controller's existing 4 ms
  `TransactionBudget`. Absence turns now cap at four children and check that
  budget after each guaranteed-progress child; when it expires, the frontier
  advances only through the last inspected child and immediately yields the
  writer. Revision gaps remain legal and monotonic, while persisted cursor,
  lease, tombstone, location-close, observation, and cascade invariants are
  unchanged. Budget and begin/commit overruns now also emit targeted warnings.
- Rebuilt post-budget-fix smoke `smoke-absence-budget-20260823a` passes every
  gate against the new production image. Its retry-free request p99 is 12.775
  ms initialization, 27.695 ms catalog read, 14.135 ms foreground write, and
  23.231 ms status; writer admission p99 is 5.439 ms with no cancellation.
  All 23 queue classes drain with zero attention or duplicate-effect candidate,
  the injected Face failure recovers, quick-check and foreign keys are clean,
  retained WAL stays below 0.08 GiB, and two distinct page-complete post-drain
  checkpoints are observed. The short corpus does not exercise an absence
  finalization during the steady window, so this is rebuild/integration proof;
  the fresh full baseline and three-repeat campaign remain the runtime budget
  proof.
- Fresh frozen idle baseline `baseline-linux-arm64-budgeted-20260824-2` passes
  and is qualification eligible against image
  `sha256:aee138a84a7a2f35386de4b8f5fe9ad291db18ed323c242e4f94a4d9a1630f55`
  and Server binary `7877f26ddd17a2fe...`. It records the required 18,000 /
  18,000 / 10,800 / 10,800 retry-free initialization, catalog-read,
  foreground-write, and status samples with p99 12.719 / 21.135 / 18.319 /
  16.799 ms; writer admission p99 is 1.928 ms and SQLite telemetry coverage is
  0.997. WAL allocation peaks at 0.02 GiB with no positive slope, 61
  checkpoints complete during the steady window, and the two post-drain
  checkpoints are page-complete. Catalog quick-check, foreign keys, and all
  queue/integrity checks pass. This freezes the current dirty-tree/config
  hashes, deterministic corpus, Linux/arm64 runtime, and AC-powered host row
  for the three mixed-load qualification repeats.
- The first fresh three-run sequence after the budget fix
  (`qualify-linux-arm64-budgeted-20260824-3` through `-5`) has three green
  per-run summaries and absence hold p99 values of 7.119, 7.479, and 5.655 ms.
  The strict comparison correctly rejected the sequence for two reasons: the
  generated corpus manifest digest included its wall-clock `GeneratedAt` field
  even though all three runs had the same 1,200 entries and 361,544,633 bytes,
  and the status class reached a 4.796x mixed/idle p99 ratio against the 4x
  diagnostic limit. The digest is now canonicalized to exclude only that
  timestamp while retaining every content/expectation field; the regression
  test is red against the missing implementation and green after the fix.
  The three runs are historical diagnostics, not final comparison evidence;
  the new tool binary requires a fresh baseline and three repeats.

## Non-goals

- Do not add multiple ordinary catalog writers and rely on `busy_timeout` to
  arbitrate them. SQLite still serializes WAL writers, and that design replaces
  deterministic process-local ordering with lock races.
- Do not move River to a second database unless the audit proves that the
  atomic catalog/queue boundary cannot be kept bounded. A separate queue file
  would require an outbox for every business transition plus coordinated
  backup, restore, migration, and generation ownership.
- Do not weaken atomic domain invariants merely to shorten a transaction.
- Do not add request retries around arbitrary writes. Retry is legal only for
  a transaction whose complete effect is idempotent and revision-fenced.
- Do not hide starvation behind longer HTTP, SQLite, or River timeouts.

## Decisions (frozen)

### Connection roles

- The live catalog owns exactly one physical read/write connection and a
  bounded query-only WAL reader pool. The reader pool is the default for every
  non-transactional query, including startup inspection and background
  planning reads.
- The writer surface is explicit: a mutation either executes one bounded
  statement or enters a named, measured write transaction. Generated writer
  queries must not be injected as a generic read dependency.
- Read snapshots that must be consistent across several statements use a
  short read-only transaction on the reader pool. Rows are always closed before
  CPU, filesystem, media, network, sleep, River, or response serialization work.

### Write lifetime and admission

- One catalog write transaction contains database statements and in-memory
  validation only. Filesystem/media/network/native-tool/River work occurs
  before it begins or after it commits. Business mutations enqueue River work
  through `InsertTx` only when the insert is part of the same short transaction;
  longer delivery uses a revisioned outbox.
- Foreground and background writes share the one physical writer, so every
  background unit is bounded and restartable. Batches stop by row count,
  byte count, cancellation, and wall time; no worker holds a transaction while
  processing an unbounded result set.
- Writer wait, transaction duration, statement count, and pool/checkpoint state
  are observable. A context that expires while waiting or executing rolls the
  complete mutation back and produces a typed failure or retryable River error;
  it never leaves a durable operation in a non-terminal state.

### Queue behavior

- Worker concurrency is a compute/I/O limit, not permission for concurrent
  catalog writers. Workers plan through readers, perform slow work outside the
  catalog, then publish through one bounded transaction.
- Periodic jobs may not generate steady no-op SQLite churn. Wakeups are
  signal/coalescer or durable-pending-state driven, with a slower recovery tick.
- Queue uniqueness includes every state needed to prevent queued followers
  racing a running job. Self-chaining work yields between bounded pages.

### Checkpoint behavior

- Long-lived reader snapshots are defects because they can starve WAL
  checkpoints. Automatic checkpoint cost must not unpredictably land on a
  foreground commit; checkpoint policy and WAL growth require explicit runtime
  evidence on supported Desktop and Linux filesystems.

## Runtime proof campaign (frozen 2026-08-22)

### Architectural and measurement basis

- SQLite's transaction contract permits multiple concurrent readers but only
  one write transaction. WAL allows readers and the writer to overlap, while a
  long-lived reader can stop a checkpoint from advancing beyond that reader's
  snapshot. The campaign therefore measures writer admission, write lifetime,
  reader lifetime, and checkpoint recovery separately instead of treating
  `busy_timeout` as a throughput control. See the official
  [transaction](https://www.sqlite.org/lang_transaction.html),
  [isolation](https://www.sqlite.org/isolation.html), and
  [WAL](https://www.sqlite.org/wal.html) documentation.
- Online Backup is exercised while writers remain active because SQLite's
  [Online Backup API](https://www.sqlite.org/backup.html) is designed to hold
  the source only for bounded read steps. A copied live database file is never
  accepted as backup evidence.
- `database/sql.DBStats` exposes total wait count and duration, not a latency
  distribution. Its values remain a reconciliation signal, but p99 comes from
  per-operation observations and client-side histograms rather than dividing
  aggregate counters. See the Go
  [`DBStats`](https://pkg.go.dev/database/sql#DBStats) contract.
- The run manifest records `sqlite_version()`, `sqlite_source_id()`, the
  `go-sqlite3` version, and the application commit. The currently pinned
  `go-sqlite3` v1.14.48 bundles SQLite 3.53.3, which is newer than the official
  WAL-reset fix boundary. A run with an unexpected or vulnerable SQLite build
  fails preflight rather than being compared with the baseline.

### Workspace and disk contract

All campaign-owned bytes live under
`.cache/sqlite-concurrency/<run-id>/` on `/Volumes/CodeBase`. A pressure-specific
Compose override bind-mounts that run's `storage/` and `app-state/` directories
instead of putting the growing catalog, WAL, derived media, and backups in
opaque Docker named volumes. The immutable source corpus is shared read-only;
each repeat restores a known base snapshot into a new run directory.

| Owner | Hard allowance |
| --- | ---: |
| Pinned source corpus and deterministic expanded media | 3 GiB |
| Live repositories and derived media | 6 GiB |
| Catalog, WAL/SHM, backups, restore points, and restore staging | 4 GiB |
| Histograms, JSONL telemetry, resource samples, browser traces, and profiles | 3 GiB |
| Safety reserve that no test may allocate | 4 GiB |
| Total | 20 GiB |

- A watchdog samples directory usage and free space every five seconds. It
  stops producers at 15 GiB, fails the run at 16 GiB, and preserves the 4 GiB
  reserve for SQLite rollback, checkpoint, logs, and cleanup. Disk-full behavior
  is tested with injected/quota-scoped faults, never by filling the host volume.
- Cleanup may remove only the resolved run-id directory created by the harness.
  Failed runs retain summaries and the smallest useful failure artifacts until
  review; successful repeats keep summaries/histograms and discard media copies.
- On 2026-08-23, after the user completed the approved Docker/cache cleanup,
  `/Volumes/CodeBase` has 220 GiB free and the macOS Data volume backing
  OrbStack has 33 GiB free. This clears the 25 GiB preflight boundary, but the
  harness still rechecks both volumes before every Compose run. It never runs
  `docker system prune` itself, and Docker backing-store capacity remains
  separate from the 20 GiB campaign budget.

### Percentile and repeatability contract

- Steady-state p99 is reported only for an operation class with at least 10,000
  scheduled terminal observations in that run. The primary histogram includes
  a timeout at its deadline instead of calculating a survivor-only percentile;
  a successful-response histogram is secondary. Each platform runs one idle
  baseline and three independent mixed-load qualifications. Histograms are not
  pooled across repeats: all three must pass, and the report headline uses the
  worst per-run p99.
- The load generator uses a monotonic clock and constant scheduled arrival rate,
  not a closed loop whose rate falls when the server slows. It records scheduled
  time, dispatch lateness, connection time, time to first byte, and completion.
  The primary latency includes generator lateness; the ordinary observed HTTP
  latency is retained as a diagnostic. Missed dispatches and timeouts are
  terminal samples, never discarded from the denominator.
- Histograms cover 1 microsecond through 2 minutes with three significant
  digits. Each result records sample count, p50/p95/p99/p99.9/max, failures,
  timeouts, and the exact operation mix. Warm-up samples, restore outage windows,
  and injected failures are tagged and reported separately rather than silently
  filtered.
- Startup uses 30 independent trials per state and reports p50, p95, and max.
  Thirty observations cannot support an honest p99 claim. A future startup p99
  claim requires at least 1,000 trials with a confidence interval on dedicated
  hardware and remains separate from the steady-state request/transaction p99
  gate.
- Every qualification records OS/build, architecture, CPU and memory, power
  mode, filesystem, Docker/OrbStack version where applicable, Go and SQLite
  versions, binary/config hashes, source-asset manifest, worker counts, run
  temperature, and background host load. Retries are disabled for measurement.

### Instrumentation prerequisite

The campaign may not start from the current slow-log-only telemetry. Its first
implementation slice must provide the following without publishing a private
debug API or adding unbounded labels:

- Route every runtime write transaction through a named catalog capability and
  observe writer admission (`BeginTx` call to acquisition), transaction body,
  commit, total lifetime, outcome, and cancellation. Names are compile-time
  bounded operation identifiers such as `event.publish_owner_snapshot`; user,
  repository, asset, and job IDs never become metric labels.
- Regenerate the direct-transaction inventory and either migrate or explicitly
  exempt every call site. The current working tree has 62 production `BeginTx`
  references across 29 files, including one reader transaction and migration/
  infrastructure paths; `DB.WithTx` alone is therefore not adequate coverage.
- Observe single-statement writer admission and reconcile the per-operation
  histograms with `DBStats.WaitCount`/`WaitDuration`. Keep the existing 25 ms
  slow warning, but do not infer a percentile from that thresholded log.
- Sample reader transaction lifetime and open rows so a held cursor can be tied
  to checkpoint progress. Sample WAL bytes/version, checkpoint result and
  duration, writer/reader pool stats, River state counts and oldest-job age,
  process/container CPU/RSS/I/O, and free disk on a common monotonic timeline.
- Add a host-side `sqlitepressure` controller rather than changing
  `uploadbench`'s documented ML-excluded contract. The controller drives public
  setup/auth/repository/upload/settings/business APIs, emits HDR-compatible
  histograms plus JSON summaries, and uses direct read-only catalog inspection
  only after a run for integrity and exact River timing.

The transaction/statement/rows/`DBStats` portion and the controller's
common-timeline queue/resource/WAL sampling and artifact emission were
implemented and validated on 2026-08-23. Full-duration/repeat evidence remains
active.

Expected artifacts are `run.json`, `requests.jsonl`, per-class histogram logs,
`sqlite-writer.jsonl`, `queues.jsonl`, `resources.csv`, `startup.json`,
`recovery.json`, `integrity.json`, and `summary.json`/`summary.md`. Playwright
trace/video is off during normal measurement and retained only for failures and
the three slowest startup trials.

### Deterministic pressure corpus

The corpus is generated into the run cache from the pinned Lumilio Assets
revision; no media or generated database is committed. Valid image derivatives
receive deterministic EXIF timestamp/GPS/text changes so they have distinct
content hashes. Video variants are deterministic remuxes. Purpose-built exact
duplicate, near-duplicate, Live Photo, burst/stack, GPS, OCR, face, RAW, audio,
short-video, long-video, and controlled one-shot failure cohorts remain
separate in the manifest so expected domain facts are countable.

The qualification corpus targets about 1,200 media files and at most 3 GiB of
source bytes. A 150-file smoke corpus touches every stimulus class before the
long run. Fakelumen supplies deterministic semantic, BioCLIP, OCR, face, and
video-frame responses so the campaign saturates SQLite publication paths
without conflating GPU/model variance with catalog contention; an optional real
Lumen soak is informational and cannot replace the deterministic gate.

### Queue coverage and mixed workload

The harness owns a queue-coverage manifest. A configured queue is not considered
tested merely because it was registered: the run must observe at least one
claim and one expected terminal completion for every queue below. Product APIs
and repository filesystem changes create the work; the harness may inspect but
never insert directly into `river_job` to manufacture coverage.

| Queue family | Required queues | Stimulus |
| --- | --- | --- |
| Core ingest | `ingest_asset`, `metadata_asset`, `thumbnail_asset`, `transcode_asset`, `retry_asset` | Eight concurrent uploads plus valid image/audio/video and controlled one-shot media failures that succeed on retry |
| Repository observation | `observe_repository`, `hash_repository_node`, `repository_outbox` | Four repositories with bounded add/rename/modify/delete churn across observation epochs |
| Rebuild and maintenance | `reindex_assets`, `rebuild_location_clusters`, `rebuild_events`, `event_scheduler`, `db_backup` | Full/missing-only backfills, GPS/Event fact revisions, coalesced scheduler wakeups, and an on-demand online backup while backlog is non-zero |
| Media topology | `detect_stacks`, `match_live_photo`, `process_phash` | Burst/stack, Live Photo pair, exact-duplicate, and near-duplicate cohorts |
| ML publication and sidecar | `process_semantic`, `process_bioclip`, `process_ocr`, `ocr_index`, `process_face`, `process_video_frames`, `classify_zeroshot` | All capabilities enabled through a complete pressure manifest, deterministic inference, OCR outbox drain, and zero-shot classifier backfill |

The main qualification uses the production worker-count calculation on the
measured platform and a complete pressure TOML manifest; it does not mutate
runtime-immutable settings through environment overrides. During a five-minute
warm-up, producers establish backlog. The 30-minute measured window maintains
the following open-loop foreground traffic while all queue families, an online
backup, rebuilds, and checkpoints overlap:

- 10 requests/second: readiness, setup/bootstrap, and authenticated-session
  inspection (initialization-critical readers).
- 10 requests/second: first-page library browse, repository list, and bounded
  asset detail (ordinary catalog readers).
- 6 requests/second: queue/runtime status and durable-operation receipt polling.
- 6 requests/second: deterministic like/rating/tag/album mutations over disjoint
  assets, with an expected final-state manifest.

Each class therefore has at least 10,800 scheduled samples before failures.
The producer scheduler paces uploads, repository revisions, reindex waves, and
the backup across the window so coverage is concurrent rather than a sequence
of independently drained queues.
After producers stop, the harness allows at most 20 minutes for all expected
work to converge. The base snapshot is restored before each of three repeats;
results cannot benefit from a prior repeat's warmed derived state.

Additional adversarial profiles are distinct from the main histogram:

- `held-reader`: hold one known read snapshot for 90 seconds while bounded
  writes continue, prove other readers stay responsive, then release it and
  prove checkpoint catch-up. This validates the documented WAL starvation and
  recovery behavior without treating the intentional hold as a leak.
- `writer-cancel`: queue foreground and River writes behind a held bounded
  writer, expire selected contexts, and prove rollback/terminal receipts and
  subsequent admission.
- `restart-backlog`: stop and restart one runtime generation with every queue
  family non-empty, then verify uniqueness, retries, operation receipts, and
  exact final domain counts.
- `soak`: one two-hour run at half the qualification arrival rate, with periodic
  repository revisions and backups, to detect no-op job churn, handle leaks,
  monotonic WAL growth, or latency drift that a 30-minute window misses.

### Mixed-load acceptance gates

- Zero unexpected `SQLITE_BUSY`, `SQLITE_LOCKED`, HTTP 5xx, transport reset, or
  context timeout during steady state. Expected failures are allowed only in a
  tagged injection/restore outage interval and must have the documented status
  or terminal operation receipt; no request can hang indefinitely.
- Initialization-critical reads: p95 at or below 100 ms, p99 at or below
  250 ms, and max below 1 second. Ordinary bounded catalog/status reads: p99 at
  or below 500 ms. Foreground writes: p99 end-to-end at or below 1 second and
  p99 writer admission at or below 100 ms. Every class also reports its mixed/
  idle p99 ratio; a regression above 4x requires a named investigation even if
  the absolute gate passes.
- Named background write transactions and standalone statements: writer-hold
  p99 at or below the existing 25 ms budget. Admission and total
  admission-plus-hold are reported separately so contention is visible rather
  than charged to the bounded transaction body twice. Foreground writer
  admission retains its separate 100 ms p99 gate. Atomic exceptions such as
  complete Event publication, Stack topology, and repository cutover retain
  separate labels and input-size evidence; they do not disappear into an
  aggregate percentile or silently waive the budget.
- Every configured queue has the expected coverage, no unexpected discarded or
  exhausted job, no duplicate durable effect, and no oldest-job age that keeps
  increasing after producers stop. The queue and every revisioned outbox drain
  within 20 minutes, and manifest-derived domain counts match exactly.
- Under ordinary mixed load, WAL allocation stays below 512 MiB and does not
  show an unbounded positive slope. A retained WAL allocation is not itself a
  failure: after drain/releasing the held reader, two successive, distinct
  30-second monitor checkpoints (plus at most five seconds of observation
  margin) must report `busy=0` and `log_pages == checkpointed_pages`. A
  corresponding write such as River's leader-lease renewal may advance the WAL
  between turns, but the next turn must catch that committed tail completely;
  repeated publication of one old checkpoint never counts twice.
- Final standalone inspection reports `quick_check=ok`, zero foreign-key
  violations, the expected application ID/library ID and migration generations,
  no orphan restore journal, and clean reader/writer closure.

### Real-browser startup matrix

Startup measurement uses the production web bundle in a real headless Chromium
process. Playwright waits on semantic UI states rather than network-idle, and
records three clocks: process start to `/health/ready`, navigation start to the
`BootstrapGate`/`PrimaryRepositoryGate` leaving its loading state, and process
start to the target page accepting a real interaction.

Thirty retry-free trials run for each state:

- `fresh-bootstrap`: new catalog and browser profile; the first-admin form is
  interactive. The separate TOTP ceremony remains before ordinary E2E seeding.
- `cold-established`: seeded catalog, stopped process, new browser profile and
  authenticated storage state; the first gallery page is interactive.
- `warm-established`: same catalog, warmed static assets, clean server restart;
  the first gallery page is interactive.
- `backlog-restart`: seeded non-empty queues and WAL, abrupt prior generation
  stop, immediate browser navigation; the loading gate resolves while River
  recovery and checkpoint maintenance resume.

Provisional product gates are warm-established p95 at or below 3 seconds,
cold-established/fresh-bootstrap p95 at or below 8 seconds, backlog-restart p95
at or below 10 seconds, no non-restore trial above 15 seconds, and zero trial
stuck on an initialization/loading gate. Server-ready and browser-interactive
components are reported separately so a container/image delay cannot be
misdiagnosed as SQLite starvation.

### Recovery proof matrix

Recovery is layered so fast deterministic coverage owns every journal boundary
while a smaller number of real process/browser cases proves the wiring:

1. Go restore tests inject process loss after staged copy, previous rename,
   active rename, before/after active-installed marker, and after verified
   marker; rollback injects after failed rename, rollback rename, and rollback
   marker. Each case is reconciled twice to prove idempotence. Partial staged
   copy, corrupt/checksum-invalid snapshot, receipt identity, and ENOSPC at each
   atomic marker write are included.
2. Process integration runs a real server/container through successful restore,
   verification failure and rollback, `SIGKILL` after the previous catalog has
   moved, `SIGKILL` after the new catalog is installed, and a second kill during
   rollback. It also sends `SIGTERM` with active readers/writers and exercises
   the forced-cancellation shutdown deadline.
3. Real-browser coverage retains the existing backup/download/successful restore
   and corrupt pre-stage rejection, then adds one verification-triggered rollback
   and one kill/restart reconciliation. The browser must leave its loading state,
   reconnect to the new generation, and observe exactly one terminal receipt.

Every case asserts whether the old or restored catalog must win, exact
post-backup mutation presence, restore-point/failed-artifact retention, marker
cleanup, SQLite integrity, queue recovery, generation ownership closure, and a
bounded readiness/browser-recovery window. Planned transport loss between
generation shutdown and readiness is measured as restore outage, not mixed into
steady-state request p99.

### Platform boundary

The first 20 GiB local campaign can produce enforceable macOS arm64 native
Server evidence and production-shaped Linux arm64/OrbStack evidence, plus a
real Chromium startup result for each. The OrbStack number is enforceable for
that pinned local environment but remains diagnostic for native Linux. The
campaign also measures the macOS Desktop in-process runtime generation
separately from browser navigation. Emulation is never used for p99.

GitHub's current macOS 15 and Windows 2025 Desktop jobs remain correctness/build
gates. Shared hosted runners are too variable to establish release p99 budgets.
An enforceable native Linux or Windows p99 claim requires a dedicated or
self-hosted runner with pinned hardware and filesystem; until one exists, those
platform rows are explicitly `functional-only`, not inferred from local Docker
or cross-compilation. APFS, the Linux container/bind-mount filesystem, and NTFS
results are never pooled.

### Execution slices and commands

Implementation is split so instrumentation can be reviewed independently of
the stress verdict:

1. **Complete 2026-08-23:** add named writer/reader telemetry,
   histogram/report types, inventory guard, and unit tests that prove successful
   samples, cancellations, cursor closure, `DBStats` reconciliation, and bounded
   labels.
2. **Complete 2026-08-23:** add the `sqlitepressure` controller, deterministic corpus builder, complete
   pressure manifest, Compose bind-mount override, disk watchdog, and queue
   coverage/integrity verifier.
3. **Controller probes complete; full matrices pending:** add retry-free startup timing and layered process/browser recovery cases.
4. **Complete 2026-08-23:** add root cross-module tasks `sqlite:pressure:smoke`,
   `sqlite:pressure:baseline`, `sqlite:pressure:qualify`, `sqlite:pressure:compare`, `sqlite:pressure:soak`, `sqlite:pressure:startup`,
   `sqlite:pressure:recover`, and a run-id-scoped cleanup target. Because these
   are CI-relevant Taskfile changes, use `lumilio-add-task-target` and update the
   owning workflow path filters in the same change. A scheduled/manual full job
   may call the cross-module root target; ordinary workflows continue calling
   module targets directly.
5. Run smoke, the three-repeat macOS/Linux qualification, recovery, startup,
   and soak; fix any failed invariant or budget and rerun the affected complete
   matrix. A full local evidence pass is expected to occupy roughly 8–12 hours
   of machine time while reusing less than 20 GiB.

## Execution phases

### Phase 0 — Lock the project-wide failures (deterministic core complete)

- Inventory every explicit and implicit write, transaction, River queue,
  periodic producer, startup mutation, and service/handler query dependency.
- Add deterministic tests that hold the writer while representative setup,
  auth/bootstrap, library, status, Monitor, and operation reads execute. Any
  path accidentally using the writer must fail the latency guard.
- Add initial transaction probes for pool wait, slow `DB.WithTx` lifetime,
  checkpoint latency, WAL size, cancellation, and leaked/open rows. Full named
  runtime transaction coverage is the remaining Phase 2/4 prerequisite above.
- Add a static architecture inventory that fails when a new generic writer
  query dependency or prohibited transaction-side effect escapes review.

### Phase 1 — Make readers the default (complete)

- Introduce explicit read/write catalog capabilities and update application
  construction so read-only dependencies receive query-only readers.
- Split mixed services into reader planning and explicit mutation boundaries.
  Route HTTP GET/status/setup/auth/library/Monitor and background planning
  through readers without weakening snapshot or owner-scope semantics.
- Prove query-only dependencies fail closed on accidental writes.

### Phase 2 — Bound every application transaction (high-risk paths complete;
remaining maintenance measurement active)

- Review all explicit `BeginTx` scopes and implicit multi-row writes. Move all
  filesystem, media, network, crypto/tool, sleep, River, and unbounded CPU work
  outside the transaction.
- Convert large loops to keyset pages and bounded commits. Add revision/CAS,
  operation receipts, leases, or outboxes wherever a multi-turn operation must
  resume safely.
- Centralize write-transaction instrumentation and ensure cancellation/failure
  cleanup uses an independent bounded context where terminal state is required.
- Regenerate and close the direct-`BeginTx` inventory, give every runtime scope
  a bounded operation name, and prove the instrumentation observes admission,
  body, commit, outcome, and cancellation without high-cardinality labels.

### Phase 3 — Converge every River queue (identity and idle producers complete;
formal mixed-load certification deferred)

- Classify every queue by planning reads, slow work, publish writes, worker
  count, uniqueness, timeout, retry idempotency, and periodic wake behavior.
- Remove no-op producers, bound self-chaining fan-out, and prevent running-job
  followers. Convert catalog-to-work transitions to short `InsertTx` or a
  revisioned outbox.
- Implement the queue-coverage manifest and retain the completed smoke and
  functional convergence evidence. The three-repeat qualification,
  held-reader, writer-cancel, restart-backlog, and soak profiles remain an
  optional follow-up certification, not a blocker for this implementation
  scope. A queue must still be claimed and converge; registration alone is not
  coverage.

### Phase 4 — Control checkpoints and publish diagnostics (policy and core
telemetry complete; platform measurements deferred)

- Measure WAL/checkpoint behavior under mixed load. Move expensive checkpoint
  work off foreground commits if evidence requires it while preserving the
  durability policy.
- Publish bounded operator diagnostics for writer pool wait, transaction
  duration/budget violations, WAL/checkpoint health, and per-queue pending/
  running pressure without exposing private data.
- Execute the real-browser startup and layered restore matrices on native macOS
  and production-shaped Linux only as a separate release/performance
  certification. This incident records the harness and the explicit platform
  limitation instead of requiring another long run.
- Update Backend/architecture references and support guidance.

### Phase 5 — Prove convergence and complete the plan (deferred certification)

- The Server and architecture slices selected by the implementation diff are
  complete. Desktop-native, Web/API integration, real-browser startup,
  backup/restore, queue recovery, and long soak remain deferred certification
  work rather than open implementation work.
- Preserve the small machine-readable summaries with the change or CI evidence,
  keep media/catalog artifacts out of git, and document any platform row that
  remains functional-only rather than implying unmeasured p99 parity.
- Extract durable decisions, move surviving debt, and delete this plan only
  after every validation boundary is executable and green.

## Validation evidence on 2026-08-22

- `task architecture:check` passes with the SQLite topology/wiring guard,
  including non-blocking River periodic constructors and asynchronous durable
  work probes.
- `task server:test` passes in the environment that permits loopback listeners.
  The restricted sandbox run reached the same packages but its listener tests
  failed with `bind: operation not permitted`; no code assertion failed there.
- `task desktop:test` passes, including the race-enabled Desktop runtime,
  shutdown, and storage packages.
- Focused writer-contention tests prove a default read completes while one
  write transaction is held and another writer is queued; bootstrap phase,
  storage runtime status, and pending Host Action reads each complete within a
  100 ms deadline while the writer is held. Online Backup reads outside the
  writer, Face convergence yields the writer, and queue/outbox continuation
  reuses one active River job.
- The generated-statement corpus agrees with SQLite's statement-readonly result
  for every sqlc query. `task dto`, `task config:examples`, `task web:docs`, and
  `task server:sqlc` all exit successfully and produce identical aggregate
  hashes on a consecutive rerun. The ordinary `task verify:generated` final
  `git diff --exit-code` is not directly interpretable in this already-dirty
  worktree, so content stability is recorded instead.
- `git diff --check` passes; all Server Go files are `gofmt`-clean. The raw
  production writer-read inventory contains only the two named checkpoint
  PRAGMAs in `internal/db`.

## Validation evidence on 2026-08-23

- `task architecture:check` passes with the SQLite topology, foreground-reader,
  named-transaction, and standalone-writer inventory guards.
- `task server:test` passes outside the restricted network sandbox, including
  `internal/db/catalogtx`, the real observed go-sqlite3 connector,
  Online Backup native-connection unwrapping, generated-statement readonly
  audit, Vec1 maintenance, application construction, and all queue/service/
  storage packages. The first restricted run failed only tests that bind local
  loopback listeners with `operation not permitted`; the authorized rerun is
  green.
- Catalog unit tests prove committed transactions, admission cancellation,
  reader snapshot lifetime, exactly-once manual finalization, standalone
  statement admission/cancellation, cursor open/close balance, driver-owned raw
  transaction labeling, bounded operation roles/kinds, readable JSON, and HDR
  percentile export. `git diff --check` passes.
- After the Location projection rewrite, `task server:test`, `task web:test`,
  `task desktop:test`, and `task ci:architecture` pass. The Desktop module now
  records the Server's HDRHistogram indirect dependency, which its clean
  race-enabled build exposed. Focused Location/recovery tests pass with `-race`
  on macOS using the repository CGO allowlist, and two consecutive `sqlc`
  generations have the same aggregate SHA-256.
- `task sqlite:pressure:smoke
  RUN_ID=smoke-location-convergence-20260823a` passes all request, writer,
  background-budget, queue-convergence, WAL/checkpoint, injected-failure, and
  integrity gates. Its machine-readable artifacts remain under the
  run-id-scoped `.cache/sqlite-concurrency` directory and are not committed.
- `task sqlite:pressure:baseline
  RUN_ID=baseline-linux-arm64-20260823-3 EXTRA_ARGS=--build=false` passes and is
  qualification eligible with at least 10,800 steady samples in every request
  class, zero request failures, 0.997 SQLite telemetry coverage, bounded WAL,
  two page-complete post-drain checkpoints, and clean catalog integrity.
- The Repository absence cap test is proven red against the former generic
  page (`absence turn applied 20 rows, cap 16`), then passes ten consecutive
  runs and focused `-race` coverage together with the checkpoint catch-up
  tests. Complete `internal/storage/roe/controller` and `tools/sqlitepressure`
  package suites pass. `task server:test` passes in the loopback-enabled
  environment after the expected restricted-sandbox bind-only failures, and
  `git diff --check` passes.
- `task sqlite:pressure:smoke
  RUN_ID=smoke-absence-catchup-20260823a` rebuilds the production image and
  passes all gates. Its 23 queue classes converge cleanly, all four request
  histograms are retry-free, catalog integrity is clean, and the extended
  catch-up window observes two distinct page-complete checkpoints in 58
  seconds. Artifacts remain under the run-id-scoped
  `.cache/sqlite-concurrency` directory and are not committed.
- The directly compiled current `sqlitepressure` controller runs the equivalent
  `baseline --build=false` workflow as
  `baseline-linux-arm64-converged-20260823-2`; it passes, is qualification
  eligible, records the required 57,600 retry-free steady requests with 0.997
  telemetry coverage, holds WAL allocation constant, observes two complete
  post-drain checkpoints, and passes catalog integrity. The direct binary
  avoids recreating a host Go build cache after the root Task target's first
  preflight-only attempt was correctly rejected at 23.61 GiB free.
- `qualify-linux-arm64-converged-20260823-1` is qualification eligible and
  passes all foreground, writer-admission, 25-operation background budget,
  controlled-failure, 23-queue convergence, WAL/checkpoint, and integrity
  gates. Its runtime absence distribution passes at 19.503 ms p99/max.
- `qualify-linux-arm64-converged-20260823-2` fails only the absence background
  budget at 43.679 ms hold p99/max and therefore invalidates the consecutive
  campaign despite clean request, queue, recovery, checkpoint, and integrity
  evidence. A new budget-yield regression is proven red against the former
  implementation (`largest budget-limited absence turn applied 5 rows, want
  1`) and green after the fix. The four-row cap and nanosecond-budget tests pass
  ten repetitions, focused `-race`, the complete controller package, and full
  `task server:test`.
- `smoke-absence-budget-20260823a` rebuilds the production image containing
  the count-plus-time-budget repair and passes all request, writer-admission,
  23-queue convergence, controlled-failure, WAL/checkpoint, and integrity
  gates. Its 110-second drain observes two page-complete checkpoints; the
  smoke's steady window has no absence-finalization sample, so no final budget
  claim is inferred from it.
- `baseline-linux-arm64-budgeted-20260824-2` is the fresh
  qualification-eligible idle baseline for the current image. It passes all
  eligible gates over the 30-minute steady window, including the required
  request counts, zero request failures, 0.997 telemetry coverage, bounded
  WAL/checkpoint behavior, and clean catalog integrity. The controlled-failure
  gate is intentionally ineligible because baseline disables background
  producers; mixed qualification runs must exercise that gate.
- The first current-image qualification sequence
  (`qualify-linux-arm64-budgeted-20260824-3`, `-4`, `-5`) passes each runtime
  gate independently, including 25 background operations, controlled Face
  recovery, clean 23-queue convergence, and complete checkpoint/integrity
  gates. `sqlitepressure compare` rejects it before release evidence because
  the per-run corpus identity is timestamp-unstable and status p99 reaches
  4.796x idle in the worst run. `server/tools/sqlitepressure` now computes a
  canonical corpus digest and its focused regression, package suite, and
  escalated `task server:test` all pass; this invalidates those three runs for
  the final comparison and requires a new full campaign.
- The first canonical mixed run
  `qualify-linux-arm64-canonical-20260824-2` exposed three real writer-hold
  budget violations while every request, queue, recovery, checkpoint, WAL, and
  integrity gate passed: native change batches p99 55.871 ms, directory apply
  p99 26.751 ms, and single-asset known-content publication p99 34.335 ms.
  The former four-event change quantum is now one event, which preserves the
  persisted cursor/fence contract while reducing the diagnostic smoke's change
  hold p99 to 3.783 ms. Known-content publication remains one bounded atomic
  source identity/location/outbox commit; it is now explicitly classified as
  `atomic_exception` and retains its independent hold/admission/total evidence
  rather than being hidden in the background percentile. Rebuilt diagnostic
  `smoke-change-quantum-20260824b` passes all gates (3,000/3,000/1,800/1,800
  retry-free requests, 24 background classes with zero violations, 23 queues,
  controlled recovery, WAL/checkpoint, and integrity). A fresh full baseline and
  three canonical repeats are still required after this Server-image change.
- Fresh baseline `baseline-linux-arm64-canonical-20260824-4` passes on the
  current image: initialization/catalog-read/foreground-write/status p99 are
  14.791/21.727/16.183/15.407 ms, writer-admission p99 is 1.410 ms, telemetry
  coverage is 0.997, WAL peaks at 0.02 GiB with no positive slope, and all
  checkpoint/integrity gates pass. The first post-baseline mixed repeat
  `qualify-linux-arm64-canonical-20260824-3` completes all request, recovery,
  queue, WAL, checkpoint, and integrity gates but fails the background hold
  budget in eight classes: `asset.metadata.publish` 26.303 ms,
  `embedding.save` 39.039 ms, `embedding.video_frames.save` 34.623 ms,
  `location.rebuild.apply_batch` 29.807 ms, `repository.materialize.hash`
  33.567 ms, `repository.observe.apply_change_batch` 28.639 ms,
  `repository.observe.apply_directory_batch` 37.791 ms, and
  `repository.observe.request` 27.519 ms. Initialization max also crossed one second
  (p99 48.511 ms), while foreground writer admission remained within 100 ms.
  This is actionable evidence that the short change quantum did not bound every
  maintenance writer under the full 30-minute workload; the run is not final
  evidence.
- A retry `qualify-linux-arm64-canonical-20260824-4` was intentionally aborted
  after host telemetry showed an invalid platform condition: `OrbStack Helper
  vmgr` reached about 603% CPU and host load reached 40.73 on a 10-core Apple
  M2 Pro while the pressure stack was running. Its partial artifacts are kept
  as environmental evidence and cannot be used to accept or reject the SQLite
  implementation. Subsequent qualification runs require a settled host/OrbStack
  baseline (or a pinned dedicated runner) before starting the measurement window.

## Validation boundaries

- Holding an application or River write transaction cannot block setup status,
  auth/bootstrap inspection, representative library reads, queue/status reads,
  or durable-operation polling; reference p95 is below 100 ms.
- Foreground writes under saturated background load have a bounded admission
  latency and cannot remain pending indefinitely. Cancellation rolls back the
  mutation or records its terminal operation state through a bounded cleanup
  context.
- Every production read outside an explicit write transaction uses the
  query-only pool. An attempted write through a read dependency fails closed in
  tests and architecture checks.
- No production write transaction performs filesystem, media, network,
  native-tool, sleep, River execution, or unbounded iteration. Background
  transaction p99 remains at or below 25 ms and every batch has an explicit
  row, byte, cancellation, or wall-time bound appropriate to its public input
  limit and atomicity contract.
- All River queues remain idempotent under duplicate delivery, crash after
  durable transitions, timeout, and restart; periodic producers create no
  sustained no-op write load.
- A mixed-load test runs queue claiming/completion, representative application
  mutations, readers, and checkpoints concurrently without `SQLITE_BUSY`,
  initialization starvation, WAL growth without recovery, or lost domain work.
- Steady-state p99 evidence uses at least 10,000 samples per operation class and
  passes on three independent runs without pooling. Startup evidence uses 30
  retry-free real-browser trials and is reported as p50/p95/max, not p99.
- The local evidence campaign remains below its 20 GiB run-data ceiling and
  retains 4 GiB for rollback/cleanup. Docker backing-store capacity is verified
  separately before Compose starts.
- Backup/restore and Desktop generation shutdown still close reader and writer
  ownership cleanly across the deterministic, process-kill, and browser
  recovery layers; generated artifacts and owning docs remain fresh.

## Scope decision on 2026-08-25

This task is closed for implementation and project-wide diagnosis; no further
local or remote pressure run is required for this incident. The completed
evidence is the architecture/inventory work, named transaction telemetry,
deterministic queue/controller regressions, the full Server gate, the all-green
bounded smoke, and the current-image idle baseline. The first current-image
qualification also proves request, queue, recovery, WAL, checkpoint, and
integrity convergence, but records eight background writer-hold p99 violations
and one initialization maximum above target. Its retry was invalidated by
host/OrbStack CPU interference. Neither result supports a universal p99 claim,
so the plan deliberately does not claim one.

If a later release requires platform certification, it is a separate bounded
follow-up on a pinned Linux host: one smoke, one clean full qualification, and
a second repeat only when the first result is stable or ambiguous. That
follow-up must not reopen implementation scope without a new reproducible
product failure.
