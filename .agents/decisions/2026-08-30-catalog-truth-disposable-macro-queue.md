# Decision: Keep product truth in the catalog and make QueueDB disposable

Status: implemented and qualified on 2026-08-30.

## Problem

The previous asynchronous architecture described the same work three times:
catalog intent/task rows, fine-grained River jobs, and product tables. Workers
could own broad catalog writers and enqueue children, product APIs exposed
River identity, queue concurrency was the accidental resource scheduler, and
some recovery decisions inspected River rows. This made QueueDB loss a product
correctness event and allowed the sum of individually bounded workers to
oversubscribe a small host.

Repository observation, OCR indexing, and SQLite concurrency decisions also
encoded parts of that topology: `repository_outbox`, a snoozing River
controller lifecycle, a recurring OCR drain job, and a shared catalog/queue
database assumption. Preserving those mechanics would leave two architectures
reachable after the cutover.

## Decision

`catalog.db` is the sole product truth for desired and applied generations,
terminal errors, operation receipts, repository observation state, projection
revisions, and the transactional `domain_outbox`. Product APIs explain work
only from those records. Foreground commands atomically mutate domain state and
publish typed envelopes; they never insert River jobs or expose River IDs.

QueueDB is an independent, disposable control-plane database. River registers
exactly these eight macro kinds on one `catalog_macro` queue:

- `ingest_asset`
- `scan_repository_batch`
- `analyze_asset`
- `generate_asset_derivatives`
- `transcode_media`
- `enrich_asset`
- `rebuild_projection_batch`
- `backup_catalog`

The domain-outbox adapter bulk-publishes closed macro commands. A periodic
catalog-only reconciler republishes every outstanding generation without
asking QueueDB whether work exists. Replacing QueueDB therefore loses only
control history. For repository observation, recovery republishes the active
run's `requested_epoch`; a newer coalesced `desired_epoch` is scheduled after
that active run applies. A macro may execute while `desired_epoch` is newer,
but it must still match the active run and its requested epoch.

One process-wide execution engine admits fine-grained steps against explicit
CPU, disk, memory, image-codec, video, and inference capacities. One bounded
commit coordinator owns typed asynchronous catalog mutations and acknowledges
each producer only after commit or a proven stale/already-applied no-op.
Workers and preparers receive narrow read/filesystem capabilities, not catalog
writers, transaction callbacks, or child-job insertion.

Derived artifacts use immutable identities containing source fence, stage,
pipeline version, and name. Publication precedes catalog activation. The first
complete, nonempty regular file successfully published at that identity is
canonical; a retry that recomputes different but valid bytes adopts and hashes
the existing artifact instead of overwriting it. This accommodates
nondeterministic encoders while preserving immutable identity and crash-safe
catalog activation.

The E2E image has a dedicated Dockerfile target containing its test manifest.
The production final target does not. This keeps local and remote-daemon
Compose runs on one path without a host bind mount that a remote daemon cannot
read.

## Superseded clauses

This decision does not rewrite earlier decisions. It supersedes only their
queue/control mechanics:

- The 2026-08-17 OCR decision's recurring River drain and OCR-specific queue
  are replaced by catalog projection desired/applied state,
  `rebuild_projection_batch`, and catalog reconciliation. Its revisioned
  source-of-truth and idempotent projection requirements remain.
- The 2026-08-22 repository-observation decision's `repository_outbox`, one
  snoozing River-controller lifecycle, queue uniqueness, and compatibility
  cutover/preflight are replaced by `domain_outbox`, bounded scan macros,
  catalog active-run recovery, and the destructive target schema. Its C0/C1,
  identity, ownership, absence-proof, and periodic-authoritative-verifier
  invariants remain.
- The 2026-08-25 SQLite decision's rejection of a separate River database is
  reversed because QueueDB no longer contains business transitions or product
  truth. Its one-writer catalog, query-only reader pool, bounded transaction,
  cancellation, instrumentation, and checkpoint rules remain.

## Qualification evidence

On the Radxa X4's Intel N100, a 144-unit mixed CPU/disk/image/video/inference
workload completed in 3.632 seconds (39.6 units/second). Measured peaks matched
the configured capacities: CPU 4, disk 3, image 2, video 1, inference 1, and
168 MiB reserved memory. The bounded wait queue peaked at 139; wait p50/p95/p99
were 1.514/3.375/3.565 seconds.

The live QueueDB-replacement scenario converged four repository runs after
recovering their active epochs. All four finished with
`desired_epoch = applied_epoch = 6`, no active run or terminal error, zero
catalog backlog/outbox rows, and 107 completed versus zero non-completed River
jobs. A separate reprocess receipt completed after the server was killed and
`river.sqlite3`, its WAL, and SHM were deleted; the catalog-owned asset and
receipt remained readable and correct.

At the drained snapshot, catalog and QueueDB writer-wait deltas were zero,
their WALs were 2,995,272 and 1,153,632 bytes, and coordinator depth was zero
with peak one, zero failures, and 206 acknowledgements. Coordinator enqueue
p50/p95/p99 were 1/7/18 microseconds; oldest-wait p50/p95/p99 were
0.238/10.263/10.575 milliseconds; transaction p50/p95/p99 were
0.400/2.449/3.629 milliseconds; batch p99 was two. Catalog backlog, runnable
work, terminal errors, desired/applied lag, domain outbox, and macro remaining
depth were all zero.

The exact dirty tree passed the serial tagged Linux/CGO server suite, focused
race suites, local Server/Web/Desktop/Site gates, architecture checks,
generated-contract idempotency, crash-window and recovery tests, and the
fresh-volume video-semantic E2E slice.

### Demo-seed resource observation — 2026-08-30

The current dirty tree was built into a fresh-volume E2E stack on the Radxa
X4 (Intel N100, four cores, 7.5 GiB RAM, NVMe/Btrfs). The E2E settings enabled
semantic and video-semantic processing with at most eight video frames, while
BioCLIP, OCR, and face processing were disabled. Lumen was the deterministic
fixture, so these measurements cover server orchestration and local media
processing, not real model-inference cost.

The pinned demo profile contained 266 files totaling 385,104,288 bytes. The
server accepted 262 files totaling 378,363,192 bytes (360.84 MiB) in 14.94
seconds: 17.54 accepted assets/second and 24.15 MiB/second. Two AVIF and two JXL
files totaling 6,741,096 bytes were rejected by the upload allowlist, so the
demo-seed command exited with status 1. Because the canonical E2E setup already
owned the primary repository, the measured run targeted that existing primary;
the demo seeder's attempted regular-repository creation had returned HTTP 400
with `invalid repository storage folder: invalid repository name: name is
required`.

From the first accepted receipt to the final workload macro completion, the
run took 271.579 seconds; 257.624 seconds elapsed between the last accepted
receipt and that final completion. The workload added 1,339 completed macro
jobs, or 4.93 jobs/second over the active interval, and completed the accepted
asset set at 0.965 assets/second. All 792 asset-pipeline rows reached their
desired versions (`sum(desired_version) = sum(applied_version) = 792`) with no
terminal errors. Macro remaining depth reached zero. The remaining catalog
runnable row was repository observation state with
`full_verification_required = 1` and source `periodic`; the E2E verifier
interval was 3,600 seconds.

Docker CPU uses one core as 100%. Sampled photo/enrichment processing used
approximately 125–173%, while video processing used 323–375%; the highest
sample was 375.38%. During video processing, five-second host samples were
86–100% busy with 0% IO wait. The four-core-normalized active-interval average
was approximately 62%, estimated from cumulative cgroup CPU time after
adjusting for the observed idle intervals. The deterministic Lumen fixture
generally used 2–7% of one core.

The server container used approximately 86 MiB before upload and reached a
sampled peak of 1.025 GiB. After macro drain, Docker memory samples were 681.5
MiB immediately, 677.9 MiB after approximately three minutes, and 684.1 MiB
after approximately five minutes. At the five-minute sample, PID 1 reported
573,888 KiB `VmRSS`, 814,568 KiB `VmHWM`, and 12,768 KiB `VmSwap`. The
container recorded zero restarts, `OOMKilled=false`, and remained healthy.

Workload cgroup block-I/O deltas were 164.40 MiB read and 857.48 MiB written;
host NVMe deltas were 259.45 MiB read and 1,139.15 MiB written. Five-second
host IO-wait samples were 0–3% during upload and 0% during the CPU-saturated
video intervals. The drained volumes contained 97,136,440 bytes of app state
and 465,118,379 bytes of storage data.

At the drained snapshot, the commit coordinator had depth zero, peak depth
one, zero blocked submitters, zero failures, 3,425 applied intents, 9
microseconds enqueue p95, and 34.943 milliseconds transaction p95. The
execution governor admitted 2,128 steps; 75 waited, with wait p95/p99/max of
6.865/36.831/40.534 seconds. Final catalog and QueueDB WAL sizes were
76,644,392 and 11,882,112 bytes. The macro queue had 1,340 lifetime completed
jobs (one pre-workload and 1,339 from this run), zero remaining jobs, 63.239
seconds average latency, and 352.508 milliseconds average runtime.

## Alternatives considered

**Keep fine-grained River jobs and tune worker counts** — rejected because
queue topology is not a resource budget and would retain River as a competing
lifecycle authority.

**Keep catalog and QueueDB in one SQLite file** — rejected because a disposable
control plane must be replaceable without migrating, restoring, or risking
product state.

**Preserve old jobs behind adapters, feature flags, or dual dispatch** —
rejected because two reachable schedulers can diverge and make deletion/recovery
proofs ambiguous.

**Require byte-identical encoder output on artifact retry** — rejected because
valid encoders may be nondeterministic. Immutable first-publication identity,
file validation, hashing, fenced activation, and orphan collection provide the
required safety without overwriting.
