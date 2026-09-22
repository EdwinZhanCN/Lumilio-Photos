# Processing stage grid

Status: active, created 2026-09-22. Contracts below frozen 2026-09-22 from the
owner-approved wireframe (board E of the Processing Redesign canvas). No phase
has landed yet.

Goal: the Monitor Processing tab is one grid of stage cards with a right-hand
panel. Every card has the same shape — remaining, running, retrying, failed,
and done where meaningful — in one declared unit. The panel shows a library
overview when nothing is selected and becomes the selected stage's detail
(counts, facts, failed and queued items, retry) when a card is clicked. The
header row is the shared `MonitorFrame` row every Monitor tab already uses.

This plan is not a pipeline change: stages, scheduling, River, and the Catalog
desired/applied model stay as they are. It replaces only the Processing read
model, its API, and its UI.

## Non-goals

- Splitting `enrich` into per-feature stages (semantic, OCR, face, BioCLIP).
  Enrichment is one per-file stage and has no per-task pending facts.
- A Reindex card. A reindex request pages files into the `enrich` stage; it is
  shown as a source of AI-analysis work, never as its own count.
- Redesigning the ML coverage or Capabilities tabs. They keep the shared
  `MonitorFrame` header row; only Processing changes layout.
- Changing retry or terminal semantics. Retry reuses the existing Catalog
  request path.

## Fixed contracts

### Stage catalog (closed, ordered)

| id | group | unit | Catalog source | River kind (running) | done |
|---|---|---|---|---|---|
| `import` | media | files | `catalog_operation_receipts` kind `ingest`, state `pending`/`failed` | `ingest_asset` | no |
| `scan` | media | repositories | `repository_observation_state` desired > applied; `terminal_error` | `scan_repository_batch` | no |
| `metadata` | media | files | `asset_pipeline_state` stage `analyze` | `analyze_asset` | yes |
| `thumbnails` | media | files | stage `derivatives` | `generate_asset_derivatives` | yes |
| `video` | media | files | stage `transcode` | `transcode_media` | yes |
| `analysis` | media | files | stage `enrich` | `enrich_asset` | yes |
| `events` | library | updates | `event_projection_pipeline_state` | `rebuild_projection_batch` / `event` | no |
| `places` | library | updates | `location_projection_state` + `location_resolution_pipeline_state` | `rebuild_projection_batch` / `location`, `location_resolution` | no |
| `text_search` | library | updates | `ocr_projection_pipeline_state` | `rebuild_projection_batch` / `ocr` | no |
| `backup` | library | runs | receipts kind `backup` (latest per subject) | `backup_catalog` | no |

Media stage rows join `assets` and exclude deleted assets.

### Count semantics (per stage)

- `failed`: a Catalog terminal error (`terminal_error IS NOT NULL`, receipt
  state `failed`).
- `retrying`: media asset stages — a non-terminal pending row with an
  `asset_pipeline_failures` row whose `retry_after` is in the future. Other
  stages — River deliveries in `retryable` state for the stage's kind.
- `running`: River deliveries in `running` state for the stage's kind (and
  `projectionKind` argument for projections). This is the one execution fact
  on a card.
- `queued`: Catalog pending, non-terminal, minus `retrying` and `running`,
  clamped at zero. The Catalog and QueueDB reads are not one snapshot.
- `remaining` = `queued + running + retrying` is the card's big number;
  `failed` is never part of it.
- `done` (media file stages only): rows at `applied_version = desired_version`
  with no terminal error.
- `status`, first match wins: `off` (analysis when no Lumen task is enabled) →
  `attention` (failed > 0) → `working` (running > 0) → `retrying` →
  `waiting` (queued > 0) → `idle`.
- `sources` (media file stages): pending rows attributed through
  `asset_pipeline_receipt_stages` to `reprocess`, `retry`, or `reindex`
  receipts; the rest are new or changed files.
- `oldest_queued_at` and `last_activity_at` per stage.

### API

- `GET /api/v1/admin/processing` replaces `GET /api/v1/admin/monitor/processing`
  (removed in the same change):
  `{ generated_at, overview: { media_total, media_in_progress, running,
  failed_media, library_pending, last_activity_at }, stages: [ { id, group,
  unit, status, remaining, queued, running, retrying, failed, done?,
  sources?, oldest_queued_at?, last_activity_at?, retryable } ] }`.
  `retryable` states whether bulk retry exists for the stage.
- `GET /api/v1/admin/processing/stages/{id}/items?state=failed|queued&limit=&cursor=`:
  bounded (limit ≤ 50) items `{ subject_id, asset_id?, label, reason_code?,
  attempts?, updated_at }`. Failed items carry a localizable `reason_code`,
  never a raw error string in the public payload.
- `POST /api/v1/admin/processing/stages/{id}/retry`: media file stages create
  one `retry` receipt and call `RequestAssetStagesTx` for at most 500 failed
  rows per call, returning `{ accepted, remaining }`. Library projection stages
  re-request their projection. `import`, `scan`, and `backup` are not
  retryable here (`retryable: false`).
- Per-file retry reuses `POST /api/v1/assets/{id}/reprocess` with that stage.
- Queue delivery diagnostics (per-queue summaries, error samples, delivery
  totals) move to `GET /api/v1/admin/processing/diagnostics`, shown only in
  the Diagnostics dialog.
- All routes are administrator-only. OpenAPI regenerated with `task dto`.

### UI

- Header: the shared `MonitorFrame` row — "Last success {time}", the
  Diagnostics action, and Refresh — exactly as ML coverage and Capabilities.
- Body: two columns. Left, "Media" (3-column grid) and "Library" (4-column
  grid) sections of identical cards: name, status tag, `remaining` with unit,
  one line (`N failed` or `N running`). Right, one panel:
  - Overview (no selection): the Rive tray driven by media in progress, and
    six stats — media in library, media in progress, running now, failed,
    library updates, last activity.
  - Detail (card selected): name, status, four counts, facts (done, sources,
    oldest queued, last activity), failed items with per-item and stage
    "Retry all", then queued items. Back or re-clicking the card returns to
    Overview.
- Selection lives in the `stage` URL parameter beside `tab`.
- No descriptive paragraphs, no cross-tab links, no hidden legends.
- Narrow widths: the panel stacks below the grid; the grid collapses to two
  then one column.
- The Rive tray appears once, in Overview.

## Execution phases

### Phase 0 — Freeze contracts
- [x] Stage catalog, count semantics, API shapes, and UI recorded above.

### Phase 1 — Summary read model (Server)
- [ ] Stage catalog type and one read that computes every stage from Catalog
  plus River running/retryable counts, with fixture tests over a migrated
  Catalog and QueueDB for each count rule, deleted-asset exclusion, clamping,
  and `sources` attribution.
- [ ] `GET /api/v1/admin/processing`; remove `/admin/monitor/processing`.
- [ ] Measure the summary on a 100k-asset fixture; cache for a few seconds if
  it exceeds 50 ms.

### Phase 2 — Items, retry, diagnostics (Server)
- [ ] Items endpoint with bounded pagination and reason codes.
- [ ] Bulk retry through `RequestAssetStagesTx` and projection re-requests;
  tests prove terminal errors clear, work re-enters the scheduler, the batch
  bound holds, and a repeat call is harmless.
- [ ] Diagnostics endpoint carrying today's queue summaries.
- [ ] `task dto`; `task server:test`.

### Phase 3 — Stage grid UI (Web)
- [ ] Stage card, grid, Overview panel, Stage detail panel, Diagnostics
  dialog; `stage` URL parameter.
- [ ] Remove `WorkLaneList`, the files hero, and inline delivery sections.
- [ ] Component tests and a flow spec: selection switches the panel, Back
  restores Overview, retry calls the right endpoint, failed stages never read
  as idle, refresh failure keeps cached facts with the stale warning.
- [ ] Visual spec at 390/800/1280 in light and dark.
- [ ] i18n extract and zh fill; `task web:test`.

### Phase 4 — Close
- [ ] Decision record superseding `2026-09-22-processing-two-patterns.md` and
  the response shape of `2026-09-14-processing-monitor-api.md`.
- [ ] Monitor `doc.ts` and regenerated `doc.md`; `task verify:generated`;
  `task architecture:check`.
- [ ] Delete this plan.

## Validation boundaries

- Every card reports the same fields in its declared unit; no card mixes files
  and updates.
- A stage with failures is never shown as idle or clear, including when
  nothing remains.
- Deleting QueueDB changes only `running`/`retrying` execution facts; Catalog
  counts, failed items, and retry still work.
- Reindex work appears once, inside AI analysis, attributed as a source.
- Bulk retry never replays a mutation outside the requested stage and is
  bounded per call.
- Processing, ML coverage, and Capabilities share the same header row.
- The summary stays within its latency budget on a 100k-asset fixture.
