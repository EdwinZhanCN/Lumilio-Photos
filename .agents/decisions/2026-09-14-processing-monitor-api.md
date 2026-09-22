# Decision: One Processing Monitor read, separate work and delivery semantics

Status: implemented — 2026-09-14.

## Problem

Processing fetched `/admin/river/stats` and `/admin/river/queue-summary`
independently. Both reported delivery totals for the same single `catalog_macro`
queue, while the former also contained authoritative Catalog work counts. Two
polls and refresh paths obscured the distinction between product work and
execution records and could leave sections displaying different refresh results.

## Decision

`GET /api/v1/admin/monitor/processing` is the single administrator-only read.
It returns `processing` (Catalog pending/failed work), `deliveries` (global
River state counts and per-queue diagnostics), and `generated_at`. The old two
routes are removed; Server and Web ship the generated contract together.
`error_limit` remains bounded to 0–20, default 5, per queue.

StatMonitor owns one five-second TanStack Query subscription and manual refresh.
QueueSummaryList receives queue data as props. Loading, failed refresh, and
last-success presentation belong to the parent snapshot; cached facts survive a
failed refresh. An API error is never converted into a successful zero snapshot.

The response is global, read-only, and spans Catalog and QueueDB. It is not an
atomic cross-database snapshot; `generated_at` means response generation time.
River totals describe retained delivery records, not completed files or progress
of a particular run. Retryable records appear in both remaining and attention
counts; those categories must not be added as disjoint totals. UI hierarchy is
chosen for the user's task, not copied from transport boundaries.

This change does not introduce independent ML/Reindex queues, per-kind or
per-rebuild progress, or alter the indexing API's existing semantics. Those
require their own backend work rather than inference from aggregate counters.

## Alternatives considered

- Keep both endpoints: preserves overlapping reads, refresh ownership, and
  counters without a current consumer need for separate polling.
- Flatten everything into one counter object: hides the crucial distinction
  between durable product facts and disposable execution history.
- Fold indexing, storage, and capabilities into a universal Monitor response:
  couples unrelated scopes, costs, and refresh cadences to this local change.
- Preserve legacy aliases: retains two competing contracts for first-party
  consumers that are migrated together in this change.
