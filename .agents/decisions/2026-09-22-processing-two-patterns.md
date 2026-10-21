# Decision: Processing is one animated file tray plus one exact work list

Status: superseded on 2026-09-22 by
[the stage grid decision](2026-09-22-processing-stage-grid.md). Originally
implemented 2026-09-22. Superseded the Processing bullet of
[the Monitor visual-media decision](2026-09-12-monitor-visual-media.md); the
Rive runtime contract stays
[the Rive decision](2026-09-10-rive-processing-tray-runtime.md), and the
product/delivery split stays
[the Processing Monitor decision](2026-09-14-processing-monitor-api.md).
Architecture documentation lives in
[Monitor doc.ts](../../web/src/features/monitor/doc.ts).

## Problem

The Processing tab rendered four equal trays, and in code every tray loaded
the Rive asset even though the earlier decision reserved Rive for files. A
Repository scan, a projection, or an operation has no meaningful "stack of
paper", so three of the four trays invented a unit. Meanwhile the ML tab kept a
"Global activity" section that repeated one global enrichment backlog and the
reindex-request count: job facts inside a coverage view, read from a legacy
field that the indexing API duplicates across five lanes.

## Decision

Processing converges on exactly two patterns:

1. **Rive tray** — only files awaiting processing animate, with the fixed
   eighteen-layer, 32-files-per-layer reference. Retry-waiting stages and the
   attention count sit beside the tray as file facts.
2. **Work list** — Repository scans, optional ML analysis, reindex requests,
   projections, and operations are list rows that state what the work is, the
   exact pending count, a status (clear / in progress / needs attention / no
   data), attention, and a link to the route that owns the work.

Queue delivery diagnostics remain a separate, explicitly historical section.
`/api/v1/admin/monitor/processing` now carries `pending_analysis_assets`,
`failed_analysis_assets` (the per-file `enrich` stage, excluding deleted
assets), and `pending_reindex_requests`, so the ML backlog is a Catalog fact in
the Processing snapshot. The ML tab reports coverage and rebuild commands only,
with no cross-tab link or hidden legend; its rebuild dialog still warns about
already-pending reindex requests because that fact gates the action.

## Alternatives considered

**Keep four trays with per-type layer units** — rejected. The unit for a
Repository or an operation is arbitrary, and four canvases cost four Rive
instances for one meaningful animation.

**Animate every lane with the same Rive asset but hide the unit** — rejected.
A drained tray would imply completion semantics the Server does not expose for
scans or projections.

**Keep the ML "Global activity" section and read `queued_jobs`** — rejected.
It mixed job facts into coverage and read a backlog field the indexing API
repeats in every lane for response-shape compatibility.

**Derive the ML backlog in the browser from the indexing stats endpoint** —
rejected. That endpoint polls every fifteen seconds with repository scope and
coverage counts; the processing snapshot is the global five-second Catalog
read that already owns backlog.
