# Decision: One Asset lifecycle — present, missing, trashed — with a single purge path

Status: accepted, 2026-09-24 (owner, issue #223). Implementation is in
progress under
[the scan index and Asset lifecycle plan](../../docs/exec-plans/active/repository-index-and-asset-lifecycle.md);
this line becomes `implemented` when that plan completes. It replaces the
implicit "no hard delete" rule that the
[Repository Observation Engine decision](2026-08-22-repository-observation-engine.md)
stated for inferred signals. Its scan half is
[the repository scan index decision](2026-09-24-repository-scan-index.md).

## Problem

Deleting a photo, a file disappearing, and removing a repository were three
independent mechanisms: the app Trash (`assets.is_deleted`), ROE absence
(closed Locations), and repository removal, the only hard
`DELETE FROM assets`. The audit of 2026-09-24 (`dev` @ `34a218b`) found:

1. **Ghost photos.** A missing or replaced Asset kept `is_deleted = 0`, so
   browse still listed it while repository views and Storage counts dropped
   it, and opening it failed with a generic 409 or 500.
2. **Orphans never collected.** Repository removal only collected Assets that
   still had an active occurrence there, so already-missing Assets survived
   forever.
3. **The Trash swallowed new files.** `InsertOwnerContentAsset` upserted on
   `(owner_id, content_id)` without checking `is_deleted`, so re-uploading a
   trashed photo bound it to the trashed Asset and it never appeared.
4. **No way to delete a photo.** The Trash never freed space, and duplicate
   resolution only hid duplicates.
5. **In-place edits lost metadata.** Any tool that rewrites a file, even a
   metadata-only XMP or EXIF write, produced new content and a new empty
   Asset; albums, rating, like, tags, and people stayed on the ghost.
6. **One inconsistent hard delete.** Only repository removal deleted Assets
   and their metadata.

The plan's Phase 0 locks defects 1, 2, 3, and 5 with black-box tests that
drive an in-process Server through its HTTP API.

## Decision

**Core rule.** An Asset exists if and only if it has at least one entry. An
entry is a physical occurrence in `repository_entries`: `present`, `missing`,
or `trashed`. `PurgeEntries` is the only hard delete: it removes entries and,
in the same transaction, deletes every Asset left with no entry, together
with its metadata, derived rows, and artifacts. No other code path deletes
from `assets`, and a check in `tools/architecturecheck` or a test enforces it.

**Derived states** replace `assets.is_deleted` and `deleted_at`:
- *active*: at least one `present` entry;
- *missing*: no `present` entry and at least one `missing` entry; metadata is
  kept and the Asset becomes active again when a file returns;
- *trashed*: only `trashed` entries; hidden from the library and listed in
  the Trash view with its metadata;
- *offline*: every entry is in an offline repository; a display state only.

Browse, search, albums, people, events, and library counts show active
Assets only. Missing Assets are listed in a Missing view reached from the
Storage page's missing count. The viewer and original download return typed
`asset_missing`, `asset_offline`, and `asset_trashed` problems.

**Delete moves files into a per-repository trash.** Every repository
directory, including `primary` and `inbox/`, is user-owned, so one rule
applies everywhere. Delete (single, bulk, and duplicate resolution alike)
resolves every present entry of the selected Assets and shows the Assets,
files, bytes, repositories, and retention before confirming. If any affected
repository is offline or any file's stat tuple differs from the catalog, the
whole operation fails and nothing moves. Otherwise each file is renamed on
the same volume to `<repo>/.lumilio/trash/files/<trash_id>/<name>`, an info
sidecar `<repo>/.lumilio/trash/info/<trash_id>.json` is written with
`"format": 1` (original path, Asset, content hash, size, time, actor), and the
entry becomes `trashed`. Moves are journaled through `lifecycle_operations`
(prepared → filesystem applied → catalog committed) so a crash is reconciled
on startup, and recorded in `lifecycle_audit_events`. The sidecar lives in
user folders, so later builds keep reading format 1 and reject an unknown
newer format with a clear error. The Trash view can be rebuilt from the
sidecars alone.

**Restore** moves the file back to its original path and never overwrites; an
occupied path gets a non-colliding sibling name that is reported. A new
present entry for a trashed Asset, from an upload or a scan, makes it active
again; its trashed copies expire normally.

**Irreversible steps**, each audited, are the only ones:
- retention expiry after `repository_trash.retention_days`, a required TOML
  field with no code default that generated configs set to 30;
- "Delete permanently" in the Trash view, after explicit confirmation;
- "Remove missing items", per repository or selection, which purges
  `missing` entries and touches no file;
- repository removal, which leaves files in place and purges all of the
  repository's entries, so Assets with entries elsewhere survive and
  missing-only Assets are collected.

**Inferred signals never destroy.** A scan, a watcher, or a cloud sync never
unlinks a file, moves one into the trash, or purges an Asset. Moves never
cross volumes or overwrite, and a failed move changes nothing in the catalog.

**In-place carry-over.** When a scan finds new content C2 at a path bound to
Asset A (content C1), the hash commit:
1. re-points A to C2 when no Asset exists for (owner, C2) and A has no other
   present entry — the common case. The Asset ID, albums, rating, like,
   `user` tags, and user-edited metadata stay; every derived stage re-runs;
   re-extracted values never overwrite user edits; manual face assignments
   are re-applied to the re-detected face with IoU ≥ 0.5 or surfaced as
   unconfirmed;
2. creates Asset B for C2 with a copy of A's user metadata when A still has
   another present entry, so the untouched copy keeps A;
3. binds the entry to an existing Asset X for (owner, C2) with no automatic
   merge, keeping A as a missing entry when it has no other present entry.

In every branch a scan leaves no Asset with zero entries.

## Implementation notes

Amended 2026-09-28 while landing Phase 4a.

- **The derived state is a trigger-maintained column.** `assets.lifecycle_state`
  is recomputed by triggers on `repository_entries` whenever an entry is
  bound, unbound, changes state, or is removed, and no application code
  writes it. Browse and search filter one indexed column instead of an
  `EXISTS` over entries, and the vector index, the OCR index, and media-item
  primaries follow it through their own triggers. A `pending_hash` entry
  bound to an Asset counts as present, so an Asset does not flicker while an
  in-place edit is rehashed.
- **`PurgeEntriesTx` lives in `server/internal/lifecycle`.** It also removes
  share links and agent pins that named a purged Asset and dissolves media
  items and stacks left without members. `tools/architecturecheck` fails on
  any other `DELETE FROM assets` in server source or queries.
- **The trash lives in `server/internal/storage/trash`.** A move links the
  destination and then unlinks the source, so it never replaces a file; on a
  volume without hard links it checks and renames. Entry writes go through the
  scanner (`CommitTrash`, `CommitRestore`), which tolerates a scan that saw the
  file vanish or reappear first. Recovery trusts the disk: a file found at its
  destination was moved.
- **Refusals reuse the `repository/conflict` Problem** with `conflict_type`
  `repository_offline`, `file_changed`, `asset_missing`, `not_trashed`,
  `trash_file_missing`, or `move_failed`, until Phase 5 adds dedicated asset
  Problems.
- **Irreversible steps are unlink-then-purge and need no journal**
  (amended 2026-09-29, Phase 4b). Expiry and "Delete permanently" unlink the
  trashed file, its trash directory, and its sidecar, then purge; a crash in
  between leaves a trashed entry without its file, which the next pass
  purges. An hourly maintenance pass retries deferred recovery, rebuilds the
  Trash from sidecars (`scan.AdoptTrash`), and expires files past
  `repository_trash.retention_days`. A sidecar in a newer format is reported
  and never guessed at.
- **Duplicate resolution deletes first.** The non-kept duplicates go to the
  trash as one Delete; a refused Delete leaves the group pending and nothing
  merged.

## Alternatives considered

**Keep the soft-delete flag (`is_deleted`) beside physical state.** This was
the status quo. Two independent sources of "gone" disagree by construction,
which produced ghosts, orphans, and the Trash swallowing new files. Deriving
the state from entries leaves one authority.

**Delete to the operating system's trash.** Rejected by the owner: Windows
network drives have no recycle bin, SMB volumes usually delete immediately,
and in Docker or server installs the OS trash is invisible from the web UI.
The inconsistency is itself a hazard. A per-repository trash (digiKam's
`.dtrash` model) behaves the same on every deployment and survives catalog
loss through its sidecars.

**"Remove from library but keep the file."** Rejected: the catalog mirrors
the watched folders, so the next scan re-imports the file. Hiding without
deleting is a separate Archive or Hide feature, out of scope.

**Purge automatically when a file goes missing.** Rejected: an unmounted
drive, a renamed folder, or a slow sync would destroy albums and people.
Lightroom Classic and Plex both keep missing items until the user removes
them, and Plex ignores inaccessible locations to protect against incorrect
removal.

**A new Asset for every in-place edit.** The status quo, and defect 5. Many
tools rewrite files for metadata alone, so "same path, new content" is
ordinary and must not look like a different photo.

**Merge automatically when an edit reproduces an existing Asset's content.**
Rejected: merging two Assets' albums, people, and ratings is a user decision
with no safe default; the entry binds to the existing Asset and the old one
stays visible as missing until the user removes it.

**Delete one specific copy of a multi-copy Asset.** Out of scope: Delete acts
on the Asset and all of its present copies.

**Give the retention period a code default.** Rejected by the manifest rule in
`CLAUDE.md`: runtime-immutable settings are explicit in the TOML manifest.
