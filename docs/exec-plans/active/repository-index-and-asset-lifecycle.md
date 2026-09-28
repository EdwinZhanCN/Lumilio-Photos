# Repository scan index and Asset lifecycle

Status: active, created 2026-09-24. Phase 0 tests are written and fail on
`dev` @ `a3fb469d` (2026-09-27); they wait on branch
`test/lifecycle-regressions` and ride into the phases that fix them. Phase 1
is done: the decision records merged in #230 on 2026-09-27, which the owner
took as approval. Phase 2 (scan core, additive and not yet wired) is in
review on `feat/scan-index-core`. On 2026-09-27 the owner approved folding
the sweep into a per-directory diff, and #222 was amended to match. Child of
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
- **New persisted formats start at version 1**, per the
  [compatibility baseline decision](../../../.agents/decisions/2026-09-24-rc-compatibility-baseline.md)
  (landed in #225; every format is already at version 1):
  - The catalog baseline, `SchemaVersion` 1, is edited in place; there is no
    step file and no version bump before rc.1 ships.
  - The trash info sidecar (`<repo>/.lumilio/trash/info/<trash_id>.json`)
    carries `"format": 1`. It lives in user folders, so later builds must keep
    reading it; its reader rejects an unknown newer format clearly (test).
  - `repository_trash.retention_days` is a required TOML field with no code
    default, added to server config `schema_version` 1 in place. Generated
    configs write `30`; regenerate the examples.
- **Ordering with the other RC plans.**
  - This plan's schema lands before the rc.1 baseline freezes at the tag.
  - rc-smoke-checklist Part A runs only after this plan's last PR merges.
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
- [x] Black-box regression tests at the service or API level, which survive
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

Where they are (2026-09-27): `server/app/blackbox_harness_test.go` boots a
complete in-process Server from the generated `dev-vite` manifest, the way
Desktop embeds it, and drives it only through HTTP and the repository
directory. Each test is its own `server/app/*_regression_test.go` file and
its own commit on `test/lifecycle-regressions`, after one harness commit, so
each fixing PR cherry-picks the harness and the tests it turns green. Because they go
through the wired runtime, they pass only once the new code is wired in:

| Test | Lands with | Failure on `dev` @ `a3fb469d` |
| --- | --- | --- |
| `ScanCostIsLinear` | Phase 3 | the 10k-file scan was still crawling at 60 s with 942 files observed |
| `InPlaceOverwriteKeepsAlbumMembership` | Phase 3 | both the pixel edit and the metadata-only write left two Assets in browse and the album on the old one |
| `MissingFileLeavesLibraryBrowse` | Phase 4 | the deleted file is still listed after a completed scan |
| `ReuploadOfTrashedPhotoIsVisible` | Phase 4 | the re-uploaded photo never appears in browse |
| `RepositoryRemovalPurgesMissingOnlyAssets` | Phase 4 | on CI (Linux, macOS, Windows) the missing-only Asset survives removal (`GET` 200); on the developer Mac its precondition fails instead (see below) |

Notes for the fixing phases:
- On one developer Mac, with the repository on an external APFS volume, a
  deleted file's Asset still counted as active seven minutes after a
  completed forced verification with `files_observed: 0`. The CI runners
  recognised the deletion. Without the missing state, removal collects the
  Asset as an active occurrence, which is why the defect-2 test waits for
  that state first.
- On Windows CI the harness's server shutdown took longer than 90 s; fix the
  harness before the tests land.
- The Linux job has no `exiftool` or `ffmpeg`, but the tests still reached
  their assertions there, so ingest produces browsable Assets without them.
- Upload admission rejects a volume with less than 5% free space, which makes
  the harness fail with a 409 on a nearly full disk. Run locally with
  `TMPDIR` on a volume with room.


### Phase 1 — Decisions
- [x] `.agents/decisions/2026-09-24-repository-scan-index.md` supersedes
  `2026-08-22-repository-observation-engine.md`. Record Syncthing, Git, and
  Watchman as the references, and the rejected alternatives: keeping ROE and
  path-proven absence via journals.
- [x] `.agents/decisions/2026-09-24-asset-lifecycle.md` replaces the implicit
  "no hard delete" rule with the core rule, the invariants, the repository
  trash, and the in-place carry-over rule, and rejects the OS trash for
  delete.
- [x] Get the owner's approval of both records before Phase 2 merges
  (#230 merged 2026-09-27).

### Phase 2 — Schema and scan core (#222)
Re-sequenced 2026-09-27: this phase is additive. Dropping the ROE tables here
would break the ROE code that Phase 3 removes, and `is_deleted` is read by 104
production files and 40 query files that Phase 4 rewrites. Each table is
dropped by the phase that removes its last reader, so every PR stays green
and `dev` keeps a working Trash throughout.
- [x] Baseline: `repository_entries` (with the partial unique index over
  `pending_hash` and `present`) and `repository_scans`, beside the ROE
  tables. `task server:sqlc` regenerated. `parent_key` holds the parent's
  `path_key`, so a case-only directory rename keeps its children attached.
  Each entry carries a `revision` that every write compares and swaps.
- [x] `server/internal/storage/scan`:
  - walk with the stat cache and racily-clean handling, as a per-directory
    diff (#222 amended 2026-09-27);
  - hash with a revision compare-and-swap commit, including the three
    in-place carry-over branches from #223;
  - absence only on a positive `lstat`, with the marker re-checked before
    every removing commit; vanished directories swept in pages of 256;
  - moves and restore;
  - the settle follow-up described under Fixed contracts.
- [x] Seeded model test with 64 seeds; a 10k-vs-40k linearity check in
  `task server:test` (about 18 s: 106 µs vs 118 µs per file on an M-series
  Mac, 1.12×); writer-hold p99 of at most 25 ms asserted for the walk and
  hash passes. On its first run the model test found orphaned rows under a
  directory renamed mid-scan; the walk now descends only into directories
  whose own row is live.

### Phase 3 — Wiring and ROE removal (#222)
- [ ] Scheduler, one unique River job per repository, and bounded turns
  through the commit coordinator.
- [ ] Upload and cloud known content bind entries without a rehash.
- [ ] `locations.Resolver` reimplemented over entries; folder browse moved to
  `parent_key`.
- [ ] Watcher: `github.com/syncthing/notify` plus a Syncthing-style
  aggregator, falling back to a full scan on error or overflow; periodic full
  scan with jitter. `task desktop:test` for the cgo FSEvents build.
- [ ] Delete `storage/roe` except `pathsemantics` (move it to
  `storage/pathsemantics`). Simplify `repo_relocate_observation.go`. Run
  `task architecture:check`.
- [ ] Drop the seven ROE tables from the baseline and redefine
  `active_asset_occurrences` (and `active_asset_occurrence_paths`) over
  present entries, keeping their columns so their 65 readers do not change.
  Move `InsertContentObject`, `InsertOwnerContentAsset`, and
  `GetOwnerContentAsset` out of `repository_observation_engine.sql` before
  deleting it. Retarget `repository_staging_commits.node_id` to `entry_id`.
- [ ] Reimplement `service.ApplyAssetActivationTx` over entries and pass it
  as `scan.Config.Activate`. Delete `RepositoryFS.ReadUserMediaDirectory`
  and `WalkUserMedia`, which `ListUserMediaDirectory` replaces.
- [ ] Upload and cloud binding write `stat_checked_ns` at least one
  `scan.RacyGranularity` after the file's mtime. Lumilio wrote the file
  itself, so it is not racily clean, and the next scan must not rehash it.
- [ ] Decide where the volume's path semantics live. Phase 2 uses the host
  default, as ROE does; a catalog moved between a case-insensitive and a
  case-sensitive host would need its keys recomputed.

### Phase 4 — Lifecycle core (#223)
- [ ] Derived Asset states and `PurgeEntries` with its enforcement check.
  Drop `assets.is_deleted` / `deleted_at` here, with their last readers.
  Add the trash path column to `repository_entries`.
- [ ] Carry-over follow-ups the scan core cannot do alone: re-extracted
  metadata never overwrites the user-edited description; manual face
  assignments are re-applied to the re-detected face with IoU ≥ 0.5, or
  surfaced as unconfirmed.
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
- [ ] Re-prove the compatibility baseline's RC-build round trip on the new
  schema (radxa, image built from `dev` after this plan's last PR): fresh
  install stamps version 1 everywhere; backup → restore keeps counts, user
  edits, and original-file checksums, and the Trash view: a trashed item
  survives the round trip and can be rebuilt from its info sidecars when the
  catalog is restored from a backup taken before the delete. Record the
  image and results here.
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
