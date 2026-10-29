# Decision: rc.1 readiness is proven without hardware; native checkpoints move to rc.2

Status: implemented — owner constraint stated 2026-10-13 (no physical
machine available for the foreseeable future; every remaining rc.1 session
runs in Claude Code cloud). Amends
[the lighter-gates decision](2026-10-07-lighter-gates.md), which still
assumed one Mac pass before the tag and a human-driven Docker host.

## Problem

Three rc.1 items still depended on a person at a machine: the tag-time
smoke on "any Docker host and Edwin's Mac"
(`docs/exec-plans/active/rc-smoke-checklist.md`), the single pre-tag
native picker pass that closes the storage refactor
(the storage repository lifecycle plan's P8, completed 2026-10-26; see
[its decision records](2026-10-26-storage-command-boundary.md)), and the
post-tag published Desktop install. The owner cannot run any of them, and
the schedule has already slipped past its original 2026-09-29 target. A
cloud session has Go, Node, Task, and GitHub Actions, but no Docker daemon
and no display.

## Decision

Every gate that decides whether `v26.1.0-rc.1` is tagged is reproducible on
GitHub Actions or in a plain checkout. Specifically:

1. **Tag-time smoke is CI.** The checklist's core rows map to existing or
   new Playwright specs in the `@smoke` slice and to Server integration
   tests, run by the promotion PR's CI on the promotion head. Rows with no
   automatable evidence are listed as such, not pretended.
2. **Published-artifact smoke is a workflow.** A manually dispatched
   `release-smoke` workflow pulls the published Server image by digest,
   installs it fresh with the release bundle's Compose files, completes
   setup through the API and browser, uploads and browses media, and runs a
   backup → restore round trip. It is the post-tag Docker checkpoint and
   also produces the rc.1 compatibility fixture.
3. **Native Desktop journeys are proven by harness, not by hand.** The
   storage refactor's picker and Apply paths are covered by the embedded
   Desktop picker-controller and runtime-config harness plus the macOS and
   Windows CI jobs. The one Mac pass and the published Desktop install
   become the first checkpoints of rc.2, next to the updater proof (#240).
4. **Desktop still ships in rc.1.** The release workflow publishes one
   release only when every Server and Desktop artifact succeeds; the
   release notes say that the Desktop builds were verified by automated
   tests on CI runners, not on a physical machine, since beta.1. In return
   a dedicated code-correctness review of the Desktop launch and storage
   paths runs before promotion (session S12b in the release-hardening
   plan), with failing-first tests for whatever it finds.
5. **Nothing else changes.** rc.1 stays quality-gated and issue-gated; the
   blocker definition, the milestone membership, the no-skip rule for
   correctness tests, the all-P0–P8 scope of the storage refactor, and the
   owner's explicit tag confirmation stand. N100 numbers stay optional
   reference data (#243).

## Alternatives considered

- **Wait for hardware.** Rejected: the wait is open-ended, and every week
  of delay widens the gap between the published beta.1 and `dev`.
- **Ship rc.1 as Server-only and publish Desktop later.** Rejected: the
  CalVer release contract publishes one immutable release across both
  families; splitting it reintroduces the partial-release state that
  contract removed, and Desktop already has CI coverage on both platforms.
- **Keep the manual checklist and tick it from memory of beta.1.** Rejected:
  beta.1 predates the scan index, the Asset lifecycle, and the storage
  refactor, so its verdicts say nothing about rc.1.
- **Relax the storage refactor to P2 only (fix #242) and defer P3–P8.**
  Rejected by the owner on 2026-10-07 and not reopened here: the remaining
  phases map to the blocker definition (silent reconnect no-op, reinstall
  dead end, private-data preservation).
