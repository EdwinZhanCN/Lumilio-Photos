# Postmortem 0001: Repository scans wedged after the River timeout

## Executive summary

Repository scans inherited River's one-minute default job timeout. A Windows
production scan reached that deadline before its reconciliation transaction,
then tried to persist failure with the already-cancelled job context. The scan
receipt remained `running`, so its uniqueness constraint made every retry and
periodic scan look like a harmless concurrent-scan skip until application
restart. Scans now have no fixed River deadline, and terminal failure writes use
a separate bounded cleanup context.

## What broke

The directory walk is only part of a scan: new or changed media is inspected and
hashed before the file index, asset reconciliation, and discovery jobs are
committed together. On a repository that took longer than one minute, River
cancelled the worker context. The final transaction returned
`context deadline exceeded`, and `FailRepositoryScanRun` immediately failed for
the same reason because it reused that context.

The partial unique index allowing only one `running` scan receipt then rejected
new receipts. `ProcessScanRepository` intentionally treats that conflict as a
successful no-op for genuine concurrent scans, so subsequent jobs completed
without doing any work and without repairing the stale receipt.

## Why every net missed it

- The worker embedded `river.WorkerDefaults`, making the effective one-minute
  limit invisible in Lumilio code.
- Scanner integration tests exercised successful reconciliation and startup
  reclamation, but did not cancel the context after a running receipt had been
  created.
- The failure path was tested as ordinary database logic, not as terminal state
  that must survive cancellation of the work context.
- Small development repositories completed before the inherited deadline, so
  retries never encountered a stale `running` receipt.

## Guardrails added

- [The worker timeout regression test](../../server/internal/queue/ingest_repository_scan_worker_test.go)
  locks scan jobs to an explicit infinite River timeout instead of the implicit
  one-minute default.
- [The cancelled-scan integration test](../../server/internal/storage/scanner/scanner_integration_test.go)
  cancels immediately before reconciliation, verifies a terminal failed receipt,
  and proves the next scan completes.
- Both tests run under the existing [`server:test` task](../../taskfile.yml).
