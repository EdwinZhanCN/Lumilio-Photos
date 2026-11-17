# Decision: Asset lifecycle acceptance — three rules the #223 tests forced

Status: implemented, 2026-10-16 (rc.1 session S10). Completes
[the Asset lifecycle decision](2026-09-24-asset-lifecycle.md) and
[the repository scan index decision](2026-09-24-repository-scan-index.md);
the plan that implemented both (`docs/exec-plans/active/repository-index-and-asset-lifecycle.md`,
2026-09-24 → 2026-10-16) is deleted and lives in git history. The
acceptance evidence for #222 and #223 is listed at the end.

## Problem

Writing the last deterministic acceptance tests for #223 against a complete
in-process Server found three places where the implemented lifecycle did not
match the decision:

1. **A backup restored from before a Delete put the photo in Missing, not in
   the Trash.** The restored catalog listed the file as present; the Trash
   rebuild adopted the sidecar into a trashed row for the same Asset at the
   same path, and the next scan marked the present row missing. With one
   missing and one trashed entry the derived state is `missing`.
2. **An in-place edit never got new thumbnails.** Artifact identities are
   immutable per source content, so the regenerated thumbnails of a
   re-pointed Asset live at new paths; the catalog commit refused them as a
   mismatch against the row's "immutable" path and the derivatives macro
   retried forever while the viewer kept the old content's thumbnail.
3. **A Trash rebuilt into a catalog that never knew the file showed nothing.**
   The library and the Trash view list logical media items, which activation
   creates for an Asset bound to a present file; a trashed Asset adopted from
   its sidecar had no media item.

## Decision

- **A trashed row supersedes a stale row of the same Asset at the same
  repository path.** `scan.removeEntryTx` deletes a vanished row when the
  Trash already holds the Asset at that path instead of marking it missing;
  `scan.AdoptTrash` deletes a missing row of the Asset at the path it records.
  Both orders (rebuild then scan, scan then rebuild) end with exactly one
  trashed row, so the Asset is listed in the Trash with its metadata and
  Restore returns it. The rule needs two scan-index queries
  (`HasTrashedRepositoryEntryAtPath`, `DeleteMissingRepositoryEntriesAtTrashedPath`)
  and stays inside the writer budget: neither is a count.
- **Artifact identities are immutable; the thumbnail row per (asset, size)
  follows the Asset's current content.** `commit.applyAssetDerivatives` keeps
  the fence check (the result must belong to the Asset's current content and
  pipeline version) and replaces a row that still points at an earlier
  content's artifact through the existing upsert. The old artifact is an
  unreferenced file the artifact cleaner removes after its grace period.
- **An Asset adopted from a trash sidecar gets its media item with the
  Asset.** `scan.AdoptTrash` creates the media item and attaches the Asset as
  its original component, the same shape activation produces, so the Trash
  view lists it and Restore (which runs activation) processes it like any
  other file.

## Alternatives considered

- *Let the lifecycle trigger prefer `trashed` over `missing` when both rows
  exist.* Rejected: a missing copy elsewhere and a trashed copy are both real
  for a multi-copy Asset, and "missing wins" is the state that keeps the user
  looking for a file that is not in any trash. The duplicate rows at one path
  are the anomaly; removing them keeps the trigger's rule simple.
- *Have the Trash rebuild reconcile against the scan index itself (delete the
  present row when it adopts a sidecar).* Rejected: the file on disk, not the
  sidecar, decides whether a present row is stale; only a scan may retire a
  present row, per the "a scan proves the file vanished" contract.
- *Key thumbnail artifact paths by Asset instead of by content, so a re-point
  overwrites in place.* Rejected: it reintroduces the crash and
  nondeterministic-encoder races the 2026-08-30 catalog-truth decision
  removed, and a re-pointed Asset would briefly serve a half-written file.
- *Delete the stale thumbnail rows when the Asset is re-pointed, letting the
  derivatives stage recreate them.* Rejected: between the re-point and the
  regeneration the viewer would have no thumbnail at all; replacing the row
  at commit time keeps the old thumbnail visible until the new one is ready.
- *Create the media item lazily when an adopted Asset is restored.* Rejected:
  the Trash view is the only way to reach Restore, so a trashed Asset without
  a media item is unreachable.

## Acceptance evidence

All tests run in `task server:test`; `task architecture:check` carries the
static writer-budget rule. Red runs were recorded for every new test (the
rule under test removed or the fix reverted).

#222 (closed 2026-10-14, S9, #251):

| Box | Test |
| --- | --- |
| No-change rescan: 0 hashes, 0 entry writes | `storage/scan` `TestNoChangeRescanHashesAndWritesNothing` (hash computations counted by `hashutil.Computations`, entry writes by audit triggers the fixture installs) |
| Moves keep Asset IDs and albums | `storage/scan` `TestMovesKeepAssetIDsAndAlbumsAndRemoveTheOldEntry` |
| Upload then scan does not rehash | `storage/scan` `TestUploadedContentIsNotRehashedOrDeferredByTheNextScan`; `app` `TestScanIndexUploadThenScanDoesNotRehash` |
| Disabled or unavailable watcher converges | `app` `TestScanIndexConvergesWithoutWatcherHints` |
| Structural writer budget | `storage/scan` `TestWriterTransactionsStayWithinTheRowBudget`; `architecturecheck` `checkScanIndexWriterBudget` |
| Model test, offline, marker removed, EACCES, duplicate copy, missing file returns, racily clean, settle follow-up, linearity | `storage/scan` (Phase 2) |

#223 (closed by the S10 PR):

| Box | Test |
| --- | --- |
| Two-copy Delete and Restore keep rating, like, album, user tag, and a manual face assignment | `storage/trash` `TestDeleteMovesEveryCopyAndRestoreBringsThemBackWithMetadata` |
| External deletion leaves browse, search, albums, and the Storage count; the file returning reactivates the same Asset; Remove missing items purges | `app` `TestLifecycleRegressionMissingFileLeavesLibraryBrowse`, `TestLifecycleExternalDeletionIsHiddenEverywhereAndReversible` |
| Duplicate resolution moves non-kept files into the Trash and Restore returns them | `app` `TestLifecycleDuplicateResolutionTrashesTheNonKeptFileAndRestoreReturnsIt` |
| In-place edit with a real EXIF and XMP write keeps the Asset, the user's rating, like, tag, and description; a pixel edit regenerates the thumbnail | `app` `TestLifecycleRegressionInPlaceOverwriteKeepsAlbumMembership`, `TestLifecycleInPlaceEditKeepsUserMetadataAndRegeneratesArtifacts`; `commit` `TestDerivativesOfRepointedContentReplaceTheThumbnailRows` |
| Fork and bind-without-merge carry the full user metadata snapshot | `storage/scan` `TestInPlaceRewriteOfOneCopyForksTheAssetWithItsMetadata`, `TestInPlaceRewriteMatchingAnotherAssetBindsWithoutMerge` |
| Backup taken before a Delete, restored through the in-process runtime restart, ends with the file in the Trash view | `app` `TestLifecycleBackupTakenBeforeADeleteRestoresIntoTheTrash`; `storage/trash` `TestARestoredBackupTakenBeforeADeleteListsTheFileInTheTrash` (both orders) |
| Trash rebuilt into a catalog that never knew the file is listed and restorable | `app` `TestLifecycleTrashIsRebuiltIntoACatalogThatNeverKnewTheFile`; `storage/trash` `TestTrashIsRebuiltFromSidecarsAndCanBeRestored` |
| Re-upload of a trashed photo is visible; repository removal purges missing-only Assets; scan cost is linear | `app` Phase 0 regression tests |

Reference-hardware numbers (N100, radxa) and a Docker round trip of the
compatibility baseline stay optional per
[the rc.1 evidence decision](2026-10-13-rc1-evidence-without-hardware.md).
