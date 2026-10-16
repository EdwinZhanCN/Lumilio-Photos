# Decision: Processing is a grid of uniform stage cards with one panel

Status: implemented, 2026-09-22. Supersedes
[the two-pattern Processing decision](2026-09-22-processing-two-patterns.md)
and the response shape of
[the Processing Monitor decision](2026-09-14-processing-monitor-api.md); the
Catalog-truth rule of
[the control-plane decision](2026-08-31-catalog-derived-execution-control-plane.md)
is unchanged. Architecture documentation lives in
[Monitor doc.ts](../../web/src/features/monitor/doc.ts); the read model lives
in `server/internal/processing`.

## Problem

Processing showed three sections an owner could not tell apart: a file tray,
a list whose rows counted different things (files, projection rows,
receipts), and a queue section counting River deliveries. Stages had no
common identity or attributes, so the page read as unrelated counters, and
the owner asked why River jobs were not simply mapped into the UI.

## Decision

Processing is a closed catalog of ten stages in two groups. Media: Import,
Scan, Metadata, Thumbnails, Video, Analysis. Catalog: Events, Places, Text
search, Backup. Every stage has the same shape — remaining, queued, running,
retrying, failed, and `done` for per-file stages — in one declared unit
(files, Repositories, updates, runs), with a status derived in one order:
attention, working, retrying, waiting, idle.

Counts come from Catalog desired/applied facts. River contributes only the
execution facts `running` and, for non-file stages, `retrying`, read through
`internal/queue/jobs.ReadDeliveryGroups` so product code never reads River
tables. Reindex is attributed as a source of Analysis work through
`asset_pipeline_receipt_stages`, never a stage. Failed items expose reason
codes only. Retry re-requests failures through the existing Catalog request
path under the named `processing.stage_retry` transaction; Import, Scan, and
Backup are not retryable there.

The UI is a stage-card grid beside one panel, under the shared `MonitorFrame`
header row every Monitor tab uses. The panel is an overview (the Rive tray
and six totals) until a card is selected, then that stage's counts, facts,
failed items with retry, and waiting items. River delivery totals and queue
error samples live in a Diagnostics dialog.

## Alternatives considered

**Map River jobs directly into the UI** — rejected. QueueDB is disposable and
may be empty while work remains; one file produces several deliveries and
retries produce more; the scheduler inserts only a bounded batch; a discarded
delivery is not a product failure; completed jobs are pruned. River's shape
(kind, state, attempts, error, timestamps) is kept as the card shape instead.

**Keep the Rive tray plus a mixed work list** — rejected. Rows mixed units
and the three sections still read as three stories.

**A separate Reindex card** — rejected. Reindex pages files into Analysis, so
a separate count would double-count them.

**Split Analysis into Lumen features** — rejected. Enrichment is one per-file
stage with no per-task pending facts; splitting it is a pipeline change.

**An "Off" state for Analysis when Lumen tasks are disabled** — rejected.
Enrichment always computes perceptual hashes for duplicate detection.

**Compute `done` on every poll** — rejected. It needs a full pass over the
pipeline table (hundreds of milliseconds at 100k assets), so it is refreshed
at most once a minute; every other count reads the pending partial index.
