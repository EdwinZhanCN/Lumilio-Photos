# Repository scan index and Asset lifecycle

Status: active, created 2026-09-24. Not started. Child of
[release-hardening.md](release-hardening.md) (Phase 6). Implements the RC
blockers [#222](https://github.com/EdwinZhanCN/Lumilio-Photos/issues/222)
(replace the Repository Observation Engine with a scan index) and
[#223](https://github.com/EdwinZhanCN/Lumilio-Photos/issues/223) (unify
trash, missing files, and purge). All owner decisions on both issues were
resolved on 2026-09-24. The rc.1 date is TBD until this plan lands (owner
decision 2026-09-24).

Goal: `dev` scans repositories with a Syncthing-style index whose cost is
linear in the tree and never holds the catalog writer beyond the batch budget.
Every Asset follows one lifecycle: active, missing, or trashed, with a single
purge path. Delete moves files into a per-repository trash, recoverable for
30 days, and in-place edits keep their metadata. The acceptance criteria of
both issues are ticked.

## Non-goals

- Compatibility, data migration, or a fallback path to ROE. Pre-release
  catalogs are rejected under #221; the baseline is edited in place, for the
  last time before rc.1.
- An Archive or Hide feature, deleting one specific copy of a multi-copy
  Asset, and rename detection through a `file_id` hint (see #222 Notes).
- Cloud-side deletion. Cloud sources stay import-only.

## Fixed contracts

- **The issues are the design.** #222 specifies the data model, scan algorithm,
  watcher, and execution; #223 specifies the lifecycle, delete, restore,
  purge, in-place carry-over, and surfaces. Change the issue body first, and
  only with the owner's approval, before code diverges from it.
- **An Asset exists if and only if it has at least one entry.** `PurgeEntries` is
  the only code path that deletes from `assets`. Enforce this with
  `tools/architecturecheck` or a test.
- **A scan, a watcher, or a cloud sync never unlinks a file, never moves a file
  into the trash, and never purges an Asset.** Only a user action, or the
  expiry of one, does.
- **No filesystem I/O, hashing, or sleeping inside a catalog transaction.**
  Writer batches are at most 256 rows and at most 25 ms. There is no
  repository-wide `COUNT` in a writer transaction.
- **File moves stay on one volume and never overwrite.** A failed move changes
  nothing in the catalog. Delete, restore, and expiry are journaled through
  `lifecycle_operations` and audited in `lifecycle_audit_events`.
- **New persisted formats start at version 1**, per
  [rc-compat-baseline.md](rc-compat-baseline.md):
  - The trash info sidecar (`<repo>/.lumilio/trash/info/<trash_id>.json`)
    carries `"format": 1`. It lives in user folders, so later builds must keep
    reading it.
  - `repository_trash.retention_days` is a required TOML field with no code
    default. Generated configs write `30`. It lands before the compat plan
    resets `schema_version` to 1.
- **Ordering with the other RC plans.**
  - This plan's schema lands before the rc.1 baseline freezes.
  - rc-compat-baseline Phases 3–4 and rc-smoke-checklist Part A run only
    after this plan's last PR merges.
  - The draft promotion PR #210 is not marked ready before then.
- **Folded-in debt.** A scan that defers files inside the
  `repository_scan.settle_seconds` window schedules one delayed follow-up
  scan for that subtree. This replaces the tracker entry "A manual scan right
  after a file lands can skip it", which is deleted when Phase 2 lands.

## Execution phases

Phases 1–6 are each one PR into `dev` unless noted; Phase 0 tests ride in the
PR that fixes them. Run the narrowest checks per
`lumilio-select-checks`. Every PR updates this plan's `Status:` and ticks its
own boxes.

### Phase 0 — Lock the failures (written per fixing phase)
- [ ] Black-box regression tests at the service or API level, which survive
  the rewrite and fail on current `dev`:
  - scan cost is linear, comparing 10k and 40k generated entries;
  - re-uploading a trashed photo shows it in the library (#223 defect 3);
  - repository removal purges missing-only Assets (defect 2);
  - a missing file is not listed in browse (defect 1);
  - an in-place overwrite keeps album membership (defect 5).
- [ ] These tests are never skipped. Each one is written first on the branch
  of the phase that fixes it, run once against unmodified `dev` code, and its
  failing output is recorded in that PR's description. It then lands in the
  same PR as the fix.

### Phase 1 — Decisions
- [ ] `.agents/decisions/2026-09-2x-repository-scan-index.md` supersedes
  `2026-08-22-repository-observation-engine.md`. Record Syncthing, Git, and
  Watchman as the references, and the rejected alternatives: keeping ROE,
  path-proven absence via journals, and the OS trash for delete.
- [ ] `.agents/decisions/2026-09-2x-asset-lifecycle.md` replaces the implicit
  "no hard delete" rule with the core rule, the invariants, the repository
  trash, and the in-place carry-over rule.
- [ ] Get the owner's approval of both records before Phase 2 merges.

### Phase 2 — Schema and scan core (#222)
- [ ] Baseline: `repository_entries` (with the partial unique index over
  `pending_hash` and `present`) and `repository_scans`. Drop the seven ROE
  tables and `assets.is_deleted` / `deleted_at`. Run `task server:sqlc`.
- [ ] `server/internal/storage/scan`:
  - walk with the stat cache and racily-clean handling;
  - hash with a stat compare-and-swap commit, including the three in-place
    carry-over branches from #223;
  - sweep with the marker gate and positive `lstat`;
  - moves and restore;
  - the settle follow-up described under Fixed contracts.
- [ ] Seeded model test with at least 64 seeds; a linearity check in `task
  server:test` that finishes in under 2 minutes; writer-hold assertions.

### Phase 3 — Wiring and ROE removal (#222)
- [ ] Scheduler, one unique River job per repository, and bounded turns
  through the commit coordinator.
- [ ] Upload and cloud known content bind entries without a rehash.
- [ ] `locations.Resolver` reimplemented over entries; folder browse moved to
  `parent_path`.
- [ ] Watcher: `github.com/syncthing/notify` plus a Syncthing-style
  aggregator, falling back to a full scan on error or overflow; periodic full
  scan with jitter. `task desktop:test` for the cgo FSEvents build.
- [ ] Delete `storage/roe` except `pathsemantics`. Simplify
  `repo_relocate_observation.go`. Run `task architecture:check`.

### Phase 4 — Lifecycle core (#223)
- [ ] Derived Asset states and `PurgeEntries` with its enforcement check.
- [ ] Delete to the repository trash: a preflight that fails the whole batch
  when a repository is offline or a stat tuple differs; journaled renames;
  info sidecars in format 1. Restore never overwrites. A new present entry
  reactivates a trashed Asset.
- [ ] Expiry job, "Delete permanently", "Remove missing items", and
  repository removal, all through `PurgeEntries`.
- [ ] Duplicate resolution routed through Delete.
- [ ] Artifact cleaner keyed on Asset existence.
- [ ] A crash between rename and commit is reconciled on startup.
- [ ] The Trash view can be rebuilt from the sidecars.

### Phase 5 — API and Web (#222, #223)
- [ ] DTOs and endpoints for the scan status and counters, trash, the Missing
  view, and the typed `asset_missing`, `asset_offline`, and `asset_trashed`
  problems. Follow `lumilio-api-contract-change` (`task dto`).
- [ ] Web:
  - Trash view with restore, delete permanently, and empty;
  - Missing view reached from the Storage page count, with "Remove missing
    items";
  - a delete confirmation that shows Assets, files, bytes, repositories, and
    retention;
  - viewer states for the typed problems;
  - `VerificationHistoryModal` and `useRepositoryVerifications`.
- [ ] Terminology registry entries (`lumilio-frontend-i18n`) and feature
  `doc.ts` updates (`lumilio-feature-doc`).

### Phase 6 — Docs and proof
- [ ] Rewrite the ROE sections of `docs/BACKEND.md` and `docs/architecture.md`;
  update the guardrail links in postmortem 0001; add the retention field to
  `site/docs` (en and zh-cn) where configuration is documented.
- [ ] E2E: update `storage-admin.spec.ts` for the new scan statuses; add a
  trash spec (delete, restore with album intact, delete permanently, file gone
  from disk).
- [ ] N100 qualification (`lumilio-remote-qualification`): the 100k profile,
  a no-change rescan, writer-hold p99, and a real library import of 10k or
  more while measuring API p95. Record the numbers here and in #222.
- [ ] Complete this plan per `lumilio-exec-plan` and close #222 and #223.

## Validation boundaries

- Every acceptance box in #222 and #223 is ticked, with the evidence linked
  from the closing PRs.
- CI is green on `dev` with no skipped, disabled, or quarantined test. Every
  Phase 0 test is present, and its PR records that it failed on the old code.
- N100 numbers are recorded, and they meet #222's targets or carry an
  owner-approved revision.
- No code path other than `PurgeEntries` deletes from `assets`, enforced by a
  check.
