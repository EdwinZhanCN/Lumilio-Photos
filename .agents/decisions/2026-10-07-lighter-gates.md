# Decision: Reproducible correctness gates and optional performance evidence

Status: implemented — owner-approved gate audit policy applied to tests and release plans.

## Problem

Precise scan timing thresholds failed on shared runners without correctness
regressions. Physical N100 sign-off and exhaustive manual/process materials
also blocked issue closure and release readiness without adding reproducible
correctness evidence.

## Decision

Hard gates cover correctness and data safety reproducible stably on GitHub
Actions or any ordinary developer machine. CI required, architecture,
generated freshness, Server unit/race checks, Web checks, E2E slices with
failOnFlakyTests, Desktop macOS/Windows jobs including server changes,
release integrity, and the rc.1 compatibility fixture remain intact. Branch
protection is unchanged. Deterministic #222/#223 acceptance remains required:
zero hashes/writes on unchanged rescan, offline scan zero changes, marker
safety, crash reconciliation, only PurgeEntries deleting Assets, at most 256
rows per write batch, and no file I/O/hash/sleep in write transactions.

Normal scan tests reject gross regressions (growth >=3x or writer p99
>=100ms) and require completed scans. Precise 1.5x/25ms targets use
LUMILIO_PERF=1 in the separate non-required perf.yml workflow, run weekly,
manually, and on PRs targeting main. LUMILIO_PERF_OUT captures JSON metrics;
test logs and JSON are uploaded even on failure.

N100/radxa measurements (100k scans, unchanged-rescan time, writer p99,
API p95, throughput and constrained-memory behavior) are optional reference
hardware and marketing data, not completion or issue-closure gates. Docker
smoke and recovery checkpoints can run on any Docker host. Correctness tests
cannot be skipped or quarantined to reach green; declared optional perf and
hardware lanes are not quarantine.

rc.1 is quality-gated, not date-gated: the storage refactor completes fully,
storage-repository bugs are eradicated, and core paths hold up through real
use by Edwin and a few friends. It can be postponed. Only data loss, broken
core flows, or broken install/upgrade issues enter its blocker milestone.
#242 stays; #217 leaves rc.1; #240 and released-build updater proof move to
rc.2, requiring a prior released updater build. Milestone membership was
updated on GitHub to match (new milestone `v26.1.0-rc.2`). N100 reference
numbers are collected in #243.

Tag-time smoke focuses on setup, upload/browse, Storage create including a
removable location, Trash/restore, and language switch on any Docker host
and Edwin's Mac; Windows and remaining rows are spot checks when available.
zh coverage is reported at tag time without blocking. Published Desktop
installation is a post-tag checkpoint. User confirmation before tag/release
and main promotion through a PR remain required.

Memory updates, red-run materials, and deferred-item tracker details remain
useful guidance; missing materials do not block merges. Product-behavior
changes need Edwin; implementation deviations are recorded in PRs and
decision records.

## Alternatives considered

- Keep precise timing and dedicated hardware as hard gates: rejected because
  shared-runner noise and unavailable hardware block correct changes.
- Remove performance checks entirely: rejected because broad always-on
  limits still catch gross regressions and optional artifacts reveal trends.
- Relax correctness checks or release integrity: rejected because their
  reproducible evidence protects original media and core paths.
- Ship rc.1 to a calendar deadline or defer the storage refactor: rejected
  because eradicated storage bugs and sustained real use define readiness.
