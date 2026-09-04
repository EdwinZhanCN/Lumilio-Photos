# Test Matrix Determinism

Status: active, implementation substantially complete 2026-09-03. Deterministic
Go/native synchronization, attempt-isolated Playwright state, scoped video
completion, visible browser-test flakes, and a Linux Server concurrency gate
are implemented. E2E stack startup was approved and completed. Smoke exposed
two additional assumptions: Events legitimately has two headings, and scanned
files share the bootstrap owner and deduplicate across repositories. Semantic
heading scoping and content-distinct scan copies now pass smoke (5/5, no
retries). Video passes (1/1, no retries) on the same retained stack. Agent
runtime cross-invocation isolation passes (8/8, no retries). Global inference
counter assertions have now been removed at the user's request; the modified
video slice passes (1/1, no retries, 1.0 minute). The fixture metrics endpoint remains
available for diagnostics, but is not consulted by this spec. The Windows
native gate later exposed one remaining wall-clock assumption in the commit
coordinator pressure test: a 1 ms timeout could expire before the submission
entered the full-queue wait. The test now observes `BlockedSubmitters` before
canceling each submission.

Goal: make every required CI result depend on repository behavior rather than
goroutine scheduling, wall-clock guesses, retry residue, global E2E backlog, or
ports released before use. A retry must never turn a contaminated attempt into
a green result, and concurrent code must have an explicit race-detection path.

## Non-goals

- Do not weaken product assertions, add arbitrary sleeps, or extend timeouts as
  the primary fix.
- Do not remove macOS or Windows product coverage merely to reduce the chance
  of observing a flake.
- Do not make the E2E library run for unrelated diffs or add a scheduled
  full-library matrix.
- Do not redesign production concurrency contracts unless a deterministic
  regression proves that the product contract itself is wrong.

## Fixed contracts

- Asynchronous tests wait for an observable state owned by the operation under
  test: an entered hook, queue depth, operation receipt, repository state, or a
  manually released test gate.
- Playwright attempts own distinct mutable state. Test identity includes the
  test, repeat, and retry rather than worker `parallelIndex` alone.
- E2E assertions use repository- or operation-scoped facts; a global queue
  reaching zero is not a completion contract for one test.
- A test that passes only after retry remains visible as a CI failure until the
  infrastructure cause is removed.
- Race detection is introduced only through a target/workflow boundary that
  remains compatible with the repository's CGo dependencies and native matrix.

## Execution phases

### Phase 0 — Lock the failures

- [x] Verify the cloud implementation/test blobs are unchanged between
  `d2b2b141` and `4746ff0f`.
- [x] Tie Actions run `33680119172` to cancellation before the fake provider
  entered `List`.
- [x] Inventory retry reuse, global queue waits, sleep-based synchronization,
  fixed browser delays, and released-port helpers across the required matrix.
- [ ] Make each repaired test fail deterministically against the old readiness
  or isolation assumption where practical.

### Phase 1 — Deterministic Go and native tests

- [x] Wait for the cloud fake provider to enter `List` before cancellation.
- [x] Replace RepositoryAccess and commit coordinator sleeps with observable
  waiting/queue state.
- [x] Drive commit coordinator cancellation only after the blocked-submitter
  metric proves that the operation entered the full-queue path.
- [x] Remove close-then-rebind loopback-port allocation from Desktop and Server
  transport tests.
- [x] Run the narrow Server/Desktop packages repeatedly and under the race
  detector where the local toolchain supports it.

### Phase 2 — Attempt-isolated Playwright E2E

- [x] Give the shared workspace fixture test/repeat/retry-scoped identity.
- [x] Make fixed filenames and mutable resources unique per attempt, without
  hiding duplicates through `.first()`.
- [x] Remove video semantic global queue-idle waits; retain repository coverage.
- [x] Remove global inference-counter assertions, waits, and inferred frame
  counts from the video timing report.
- [ ] Add operation-scoped completion and persisted per-video frame-count
  evidence. Backfill/reprocess currently assert request acceptance, not fresh
  execution; frame-cap enforcement is no longer claimed by this E2E slice.
  Disabled ML still asserts unindexed repository coverage and playable video,
  but does not prove zero inference requests. Repository coverage can be satisfied
  before a rebuild; indexing `queued_jobs` currently reports global backlog
  even when the endpoint is queried with a repository filter.
- [x] Ensure cleanup does not depend on unrelated global backlog.
- [x] Make retry-assisted passes fail CI while retaining retry artifacts for
  diagnosis; remove retries once the affected slices are stable.

### Phase 3 — Deterministic Vitest browser infrastructure

- [x] Replace the 75 ms CapabilitiesMonitor delay with a manually released MSW
  gate.
- [x] Remove or expose retry masking for integration and browser projects.
- [x] Preserve serial browser-file execution only where the Vite/Vitest runtime
  still requires it, with the reason documented.

### Phase 4 — Matrix race and repeat coverage

- [x] Define the narrowest race/shuffle/repeat target compatible with Server and
  Desktop CGo constraints.
- [x] Wire any new CI-relevant Task target and matching workflow path filters in
  the same change.
- [x] Confirm macOS and Windows continue validating the embedded Server product
  surface without relying on their duplicate executions as a flake detector.
  Both jobs still call `ci:desktop:native`, which runs Server + Desktop tests
  and the native build. New Windows execution remains CI-only evidence.

## Verification evidence (2026-09-03)

- Server concurrency target: four packages, race + shuffle + count=3 passed;
  focused readiness regressions also passed repeated runs.
- Desktop focused migration regressions passed twice locally.
- Commit coordinator pressure regression failed in Windows Actions run
  `33833806184` when one 1 ms context expired before queue admission; the
  barrier-based replacement passed focused repeat and Server gates.
- Vitest integration: 37 files / 94 tests passed without retries. Browser:
  20 passed / 6 skipped tests, with the existing GPU skips preserved.
- Smoke: 5 passed on a retained stack containing earlier failed attempts.
  Old scan fixture failed three consecutive attempts because its bytes reused
  the bootstrap-owned asset's old original filename; the content-distinct
  JPEG-comment fixture passes without weakening filename or count assertions.
- Video semantic: 1 passed in 1.5 minutes with smoke + e2e asset profiles and
  fakelumen replay; no recording or real Hub was used.
  After removing global counters, the slice passed again in 1.0 minute on a
  fresh isolated stack with the same profiles and replay mode. Type checking
  (700 files), focused lint/formatting and diff whitespace checks also passed.
- Agent runtime: 8 passed in 28.4 seconds using the keyless fake Ollama.
- Type checking: 700 files passed. Changed E2E helper/spec lint and formatting
  passed. Earlier architecture and workflow checks passed.

## Validation boundaries

- The cloud cancellation regression cannot cancel before the provider-entry
  barrier and passes repeated scheduling runs.
- RepositoryAccess and commit backpressure tests contain no sleep whose purpose
  is to let another goroutine reach a presumed state.
- Every Playwright retry/repeat that mutates server state receives a distinct
  user, repository, and fixture identity, or performs explicit cleanup before
  reuse.
- Video semantic completion and cleanup succeed regardless of unrelated queue
  jobs left by an earlier E2E slice.
- Vitest does not report a green required job solely because a browser test
  succeeded on retry.
- Relevant narrow Server, Desktop, Web integration/browser, and E2E slices pass;
  the final report records any platform/toolchain limitation on race evidence.
