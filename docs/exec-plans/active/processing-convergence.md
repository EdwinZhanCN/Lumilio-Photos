# Processing convergence and metadata subprocess correctness

Status: active, updated 2026-09-18. Only a safe local-instance recheck and this plan's deletion remain. The runtime review reproduced successful JXL metadata rejected as EPIPE, short River snoozes waiting for fallback fetch, periodic full-verification overlap, and repeated analyze deliveries without Catalog failure settlement.

Phases 0–3 and Phase 4's product-work/delivery split are implemented and their regressions are committed in `1d5c4899`. That commit also carries unrelated Music work behind a Music message, so Server validation evidence stays scoped to the processing paths (`internal/processors`, `internal/queue`, `internal/storage/roe/controller`, `internal/utils/exif`, `internal/pipeline`, `internal/commit`, `app`) rather than to the commit as a whole. The Monitor surface and its endpoint are committed separately in `18a7a1d4`, which carries no unrelated Server work.

Goal: processing converges to idle after finite work, valid JXL metadata succeeds, permanent failures become actionable Catalog state, and Monitor distinguishes product work from delivery history.

## Non-goals

- Replace River, SQLite writer ownership, or ROE.
- Weaken absence proofs, source fences, or preserve failures by blocking disposable QueueDB recovery.
- Change unrelated Music implementation or original media.

## Fixed contracts

- Catalog owns desired/applied versions and domain retry/terminal state; River delivery attempts never determine product terminal state.
- ROE retains C0, bounded crawl, fixed C1, dirty verification, and authoritative finalization. Periodic full verification remains mandatory, without overlapping timer requests producing endless runs.
- Background compute stays outside catalog transactions; results use the existing commit coordinator.
- Subprocess success combines valid output, process exit, and I/O provenance. An expected stdin EPIPE must not mask valid output, genuine source read failures, or process failure. Every started process is waited.

## Execution phases

### Phase 0 — Lock the failures
- [x] Regression coverage for early successful stdin close, true subprocess/source errors, low-load continuation latency, overlapping periodic verification, and durable failure policy.
  - stdin close: `TestStreamEarlySuccessfulInputClose`; process/output vs source errors: `TestStreamRejectsProcessAndOutputErrors`, `TestStreamPreservesSourceErrors`.
  - continuation latency: `TestLowLoadShortSnoozeWakesAfterCommitAndDeadline`.
  - verification overlap: `TestPeriodicTickDuringFullScanSettlesWithoutAnotherRun`, `TestPeriodicVerificationIntervalStartsAtCompletion`, `TestPeriodicVerificationDoesNotStarveBehindIncrementalWork`.
  - durable failure policy: `TestCatalogFailureBudgetSurvivesFreshQueueAndExplicitRetryRecovers`, `TestAssetFailureAcknowledgementIsFencedAndIdempotent`, `TestAssetFailureClassificationAndCancellation`, `TestAssetGuardDoesNotAcknowledgeFailedCatalogCommit`.

### Phase 1 — Metadata subprocess
- [x] Fix concurrent I/O collection, exit/output adjudication, and unconditional process reaping; verify real JXL sample read-only.
  - `internal/utils/exif/extract.go` now delegates copy goroutines and process lifetime to `os/exec` and keeps source failures separate from reader-side EPIPE.

### Phase 2 — Scan and continuation scheduling
- [x] Ensure short scan/projection continuations are woken after becoming runnable.
- [x] Coalesce timer requests and schedule verification relative to completion while preserving explicit newer force/gap demands.
  - Migration 000017 separates `full_verification_requested_epoch` from the sticky requirement and records `full_verification_performed`; `roe/controller/commands.go` reports coalesced active runs.

### Phase 3 — Catalog failure lifecycle
- [x] Implement fenced, durable failure classification/retry/terminal outcomes without relying on River attempt exhaustion.
  - `asset_pipeline_failures` is fenced by `source_content_id`, `pipeline_version`, and `desired_version`; `internal/pipeline/failure.go`, `internal/commit/asset_failure.go`, and `app/pipeline_failure.go` own classification, backoff, and typed acknowledgement.

### Phase 4 — Monitor and validation
- [x] Expose current product work/errors separately from River delivery history.
  - `/api/v1/admin/monitor/processing` returns one envelope, `ProcessingMonitorResponse`: `processing` carries Catalog product work and `deliveries` carries River state totals and queue diagnostics. Monitor renders product work first and labels River counters as delivery records inside a disclosure. Committed in `18a7a1d4`.
- [x] Run focused regressions and required Server/Web/generated/architecture checks according to final diff.
  - `task server:test`: passes across the module. A sandboxed run must set `GOCACHE=$PWD/.local/gocache`; Go's default cache lives outside the workspace, and the denial surfaces as `server/app` and `server/cmd` reporting `[setup failed]` on the cache path rather than as a test failure. Cached packages still print `ok`, so the task exit code — not the log tail — is the signal.
  - `task web:test`: 118 files passed, 2 skipped; 506 tests passed, 6 environment-dependent skips.
  - `task architecture:check`: passes, including user-facing terminology.
  - `vp check`: 788 files correctly formatted; no lint, warning, or type errors.
  - `task verify:generated`: passes once `18a7a1d4` is committed. The gate diffs regenerated output against HEAD, so a dirty tree cannot satisfy it.
  - `i18next-cli status`: zh 100% (2194/2194).
- [ ] Recheck current local instance where safe; record any restart or migration required.
  - Not performed. No local instance was exercised, so nothing here is claimed.
- [x] Extract durable decisions and update owning docs.
  - `.agents/decisions/2026-09-10-rive-processing-tray-runtime.md`, `2026-09-12-monitor-visual-media.md`, `2026-09-14-processing-monitor-api.md`. Feature ownership is updated in `web/src/features/monitor/doc.ts` with its regenerated `doc.md`.
- [ ] Delete this plan once the local-instance recheck is recorded.

## Validation boundaries

- Successful early-closing subprocess produces metadata and is reaped; invalid output, nonzero exit, cancellation and source read errors remain failures.
- Low-load continuation is not governed by the 30-second fallback poll; notification recovery still works.
- A full scan longer than the periodic interval can settle to idle. A newer explicit force request or cursor gap is not lost.
- Same-version permanently failed work does not create endless deliveries, including after QueueDB replacement; explicit retry/version change can recover.
- Monitor counts product subjects/stages meaningfully and labels operational delivery metrics explicitly.
- Existing unrelated work is preserved; original media is untouched.
