# Decision: Admit pipeline stages from one execution budget and closed demand catalog

Status: implemented and qualified on 2026-08-31.

## Problem

River worker width previously acted as an accidental resource scheduler. Media
processors combined source reads, probing, codec work, and artifact publication
inside one admission, so codec slots were held across disk I/O and actual work
could exceed or disagree with its declared resource vector. Tool concurrency
also had several configuration sources, and a large macro width could starve
interactive work or oversubscribe a small host.

Splitting work into load, compute, and publish introduced another lock-order
hazard during qualification: prepared media work retained a `RepositoryFS`
read lease while waiting for the next governor admission. A repository writer
could then block later load stages that already held CPU/Disk tokens while the
earlier readers waited behind them in the governor, forming a circular wait.

## Decision

One immutable `execution.Budget` is the only production source for governor
capacity, River macro width, bounded wait depth, ffmpeg threads/preset, and the
resolved hardware tool session. One closed `(step, media)` demand catalog is
the only production source of CPU, DiskIO, memory, ImageCodec, VideoCodec, and
Inference vectors. River carries the eight closed macro commands and their
admission class; the in-process runtime executes bounded load, compute, scale,
and publish steps under the demand catalog.

Software ffmpeg demand uses the same positive integer as `ffmpeg_threads` and
the emitted `-threads` argument. Video frame extraction and audio waveform or
transcode reserve VideoCodec; video thumbnail scaling separately reserves
ImageCodec. Hardware sessions retain a CPU token while codec work is active.
River macro width is required from Budget and has no package default.

Repository capabilities are stage-local. Prepared thumbnail and transcode work
may carry immutable asset/content identity, probed metadata, in-memory output,
or a temporary encoder path, but never an open `RepositoryFS` lease across an
admission boundary. Each stage re-resolves the current asset location, verifies
the source-content fence, performs its bounded operation, and closes the lease
before it waits for another resource vector.

The four-core N100 E2E profile uses CPU 4, DiskIO 4, ImageCodec 2, VideoCodec
1, Inference 1, 768 MiB admitted memory, macro width 16, and three software
ffmpeg threads. Four CPU tokens are intentional: a three-thread software
transcode can pack with a one-CPU photo step while still matching its honest
CPU demand.

## Qualification evidence

On the Radxa X4 Intel N100 (four cores, 7.523 GiB RAM, NVMe/Btrfs), the pinned
demo profile accepted 262 of 266 assets; the expected two AVIF and two JXL
files were rejected by the upload allowlist. From the first accepted receipt
to media macro drain took approximately 153 seconds, or 1.71 accepted
assets/second versus the 0.965 baseline. Docker CPU samples peaked at 374.62%
and remained around 300–375% through the mixed photo/video interval. Sampled
RSS peaked at 1.265 GiB; the container remained healthy with `OOMKilled=false`.
After the repository's explicit full verification, catalog backlog, runnable
backlog, terminal backlog, desired/applied lag, outbox depth, and River
remaining/running depth were all zero.

The destructive recovery run submitted six video reprocess receipts. With
`macro_queue.running=9`, the exact `river.sqlite3`, WAL, and SHM files were
unlinked and the container was immediately killed. On restart all three files
were newly created, the catalog reconciler republished outstanding macros, and
the new queue completed 25 macros. All six catalog-owned video assets remained
readable through the product API. The final snapshot again had zero catalog
backlog, terminal errors, desired/applied lag, and River remaining/running
jobs; the recovered container was healthy and not OOM-killed.

## Alternatives considered

**Tune River queue widths per media type** — rejected because queue topology
does not express multi-resource cost and would restore multiple competing
capacity controls.

**Keep composite processor admissions** — rejected because codec slots would
remain held across probing, source reads, and artifact writes, making demand
telemetry dishonest and reducing useful packing.

**Carry an open repository capability between stages** — rejected because an
external lifecycle lease must never be held while waiting for governor
admission; qualification demonstrated the resulting lock-order cycle.

**Use CPU 3 on the N100 with three ffmpeg threads** — rejected for this profile
because software transcode would consume every CPU token and make the required
photo-plus-transcode packing impossible.
