# Decision: Processing converges from Catalog facts and proven subprocess I/O

Status: implemented, 2026-09-18. Monitor product/delivery split is
[the Processing Monitor decision](2026-09-14-processing-monitor-api.md).
Catalog-versus-River authority remains
[the catalog-truth decision](2026-08-30-catalog-truth-disposable-macro-queue.md)
and
[the control-plane decision](2026-08-31-catalog-derived-execution-control-plane.md).

## Problem

Valid JXL metadata was rejected because a reader-side stdin EPIPE was treated
as subprocess failure. Short scan continuations waited on the 30-second
fallback poll. Overlapping periodic full-verification timer requests started
new runs before the active scan could settle. Analyze deliveries repeated
without Catalog failure settlement, so replacing QueueDB could recreate
endless work for the same failed generation.

## Decision

Subprocess success is the conjunction of valid output, process exit, and I/O
provenance. An expected stdin EPIPE must not mask valid output. Genuine source
read failures, invalid output, nonzero exit, and cancellation remain failures.
Every started process is waited. `internal/utils/exif/extract.go` keeps source
failures separate from reader-side EPIPE and lets `os/exec` own copy goroutines
and process lifetime.

`asset_pipeline_failures` is Catalog product state, fenced by
`source_content_id`, `pipeline_version`, and `desired_version`. River delivery
attempts never determine retry or terminal outcomes. Same-version permanently
failed work does not create endless deliveries after QueueDB replacement.
Explicit retry or a version change can recover.

Periodic full verification remains mandatory. Timer requests coalesce onto an
active run instead of starting overlapping scans. `full_verification_requested_epoch`
is distinct from the sticky `full_verification_required` bit;
`full_verification_performed` records completion so the next interval starts
after the scan finishes. A newer explicit force request or cursor gap is not
lost.

Low-load scan and projection continuations are woken after they become
runnable; they are not governed by the 30-second fallback poll. Notification
recovery still works.

A live local-instance recheck was not a completion gate. The regressions named
in this decision lock the contracts.

## Alternatives considered

**Treat any stdin EPIPE as metadata failure** — rejected. Tools that close
stdin after a successful read produce valid output and must be reaped as
success.

**Let River attempt exhaustion mark Catalog terminal failure** — rejected.
QueueDB is disposable. Product retry/terminal state belongs in the fenced
failure row.

**Start a new full verification whenever the periodic timer fires during an
active scan** — rejected. Overlapping requests never settle to idle on a
repository larger than the interval.

**Keep one sticky full-verification bit without a requested epoch or performed
marker** — rejected. Completion and a newer force/gap request could not be
told apart, so either work repeated forever or an explicit demand was lost.

**Wait for the 30-second fallback poll to continue short scans** — rejected.
Low-load work must resume from the commit/deadline wake, not from poll
latency.
