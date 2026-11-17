# Decision: Required CI results wait on observable owned state

Status: implemented, 2026-09-03. Linux race/shuffle/repeat coverage is
`task server:test:concurrency:ci`. Playwright attempt identity lives in
`web/e2e/fixtures/test.ts`.

## Problem

Required gates could go green from goroutine scheduling, wall-clock guesses,
retry residue, a global E2E backlog reaching zero, or a port rebound after
close. A retry could then turn a contaminated attempt into a passing result,
and concurrent Server/Desktop code had no race path compatible with CGo.

## Decision

Asynchronous tests wait for a state the operation under test owns: an entered
hook, queue depth, operation receipt, repository fact, or a manually released
test gate. Sleeps whose only purpose is to let another goroutine reach a
presumed state are forbidden. The commit coordinator pressure test cancels a
blocked submitter only after `BlockedSubmitters` proves the submission entered
the full-queue wait.

Playwright attempts own distinct mutable state. Test identity includes the
test, `repeatEachIndex`, and `retry`, not worker `parallelIndex` alone. The
shared workspace fixture derives a unique user and repository from that
identity. E2E assertions use repository- or operation-scoped facts. A global
queue reaching zero is not a completion contract for one test, and global
inference counters are not product evidence.

A test that passes only after retry remains a CI failure until the
infrastructure cause is removed. Race detection is introduced only through
`server:test:concurrency:ci` (four CGo-compatible packages, `-race -shuffle
-count=3`). macOS and Windows continue validating the embedded Server product
surface; they are not flake detectors.

## Alternatives considered

**Extend timeouts or add arbitrary sleeps as the primary fix** — rejected.
That hides scheduling, it does not prove the owned state.

**Drop macOS or Windows product coverage to reduce flakes** — rejected. Platform
coverage is a product gate, not a flake detector.

**Run the full E2E library on unrelated diffs, or add a scheduled full-library
matrix** — rejected. CI selects slices through path filters.

**Enable `-race` on every Server/Desktop package** — rejected. The repository's
CGo dependencies make a whole-module race job an incompatible boundary; the
narrow concurrency target is the race path.

**Treat Playwright worker `parallelIndex` as attempt identity** — rejected. Retries
and repeats share a worker and would reuse contaminated users, repositories,
and fixture names.

**Wait for global queue-idle or Hub inference counters as video/semantic
completion** — rejected. Unrelated leftover jobs and shared inference totals
are not the operation under test.
