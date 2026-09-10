# Processing convergence and metadata subprocess correctness

Status: active, 2026-09-09. Runtime review reproduced successful JXL metadata rejected as EPIPE, short River snoozes waiting for fallback fetch, periodic full-verification overlap, and repeated analyze deliveries without Catalog failure settlement. Existing unrelated Music changes are preserved.

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
- [ ] Regression coverage for early successful stdin close, true subprocess/source errors, low-load continuation latency, overlapping periodic verification, and durable failure policy.

### Phase 1 — Metadata subprocess
- [ ] Fix concurrent I/O collection, exit/output adjudication, and unconditional process reaping; verify real JXL sample read-only.

### Phase 2 — Scan and continuation scheduling
- [ ] Ensure short scan/projection continuations are woken after becoming runnable.
- [ ] Coalesce timer requests and schedule verification relative to completion while preserving explicit newer force/gap demands.

### Phase 3 — Catalog failure lifecycle
- [ ] Implement fenced, durable failure classification/retry/terminal outcomes without relying on River attempt exhaustion.

### Phase 4 — Monitor and validation
- [ ] Expose current product work/errors separately from River delivery history.
- [ ] Run focused regressions and required Server/Web/generated/architecture checks according to final diff.
- [ ] Recheck current local instance where safe; record any restart or migration required.
- [ ] Extract durable decisions, update owning docs, and delete this plan at completion.

## Validation boundaries

- Successful early-closing subprocess produces metadata and is reaped; invalid output, nonzero exit, cancellation and source read errors remain failures.
- Low-load continuation is not governed by the 30-second fallback poll; notification recovery still works.
- A full scan longer than the periodic interval can settle to idle. A newer explicit force request or cursor gap is not lost.
- Same-version permanently failed work does not create endless deliveries, including after QueueDB replacement; explicit retry/version change can recover.
- Monitor counts product subjects/stages meaningfully and labels operational delivery metrics explicitly.
- Existing unrelated work is preserved; original media is untouched.
