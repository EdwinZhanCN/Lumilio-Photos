-- Repository scan index (#222): entries mirror the tree, scans record runs.
-- Every entry write is a compare-and-swap on revision, and nothing here
-- counts a whole repository inside a writer transaction.

-- name: ListRepositoryEntryChildren :many
-- The live and missing rows of one directory, for the walk's diff. Trashed
-- rows are not part of the tree.
SELECT * FROM repository_entries
WHERE repository_id = ?1
  AND parent_key = ?2
  AND state <> 'trashed'
ORDER BY path_key, entry_id;

-- name: GetRepositoryEntry :one
SELECT * FROM repository_entries
WHERE entry_id = ?1;

-- name: InsertRepositoryEntry :one
INSERT INTO repository_entries (
    entry_id, repository_id, path, path_key, parent_key, kind,
    size, mtime_ns, ctime_ns, file_id, stat_checked_ns, state,
    content_id, asset_id, revision, missing_since, updated_at
) VALUES (
    sqlc.arg(entry_id), sqlc.arg(repository_id), sqlc.arg(path), sqlc.arg(path_key),
    sqlc.arg(parent_key), sqlc.arg(kind), sqlc.arg(size), sqlc.arg(mtime_ns),
    sqlc.narg(ctime_ns), sqlc.narg(file_id), sqlc.arg(stat_checked_ns), sqlc.arg(state),
    sqlc.arg(content_id), sqlc.arg(asset_id), 1, sqlc.arg(missing_since), sqlc.arg(updated_at)
)
RETURNING *;

-- name: UpdateRepositoryEntryObservedCAS :execrows
-- The walk saw the entry with a new stat tuple, or saw a missing entry
-- again. The binding is kept so the hash commit can apply the in-place
-- carry-over rule.
UPDATE repository_entries
SET path = sqlc.arg(path),
    size = sqlc.arg(size),
    mtime_ns = sqlc.arg(mtime_ns),
    ctime_ns = sqlc.narg(ctime_ns),
    file_id = sqlc.narg(file_id),
    stat_checked_ns = sqlc.arg(stat_checked_ns),
    state = sqlc.arg(state),
    missing_since = NULL,
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE entry_id = sqlc.arg(entry_id)
  AND revision = sqlc.arg(expected_revision);

-- name: MarkRepositoryEntryMissingCAS :execrows
UPDATE repository_entries
SET state = 'missing',
    missing_since = sqlc.arg(missing_since),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE entry_id = sqlc.arg(entry_id)
  AND revision = sqlc.arg(expected_revision)
  AND state IN ('pending_hash', 'present', 'unsupported');

-- name: DeleteRepositoryEntryCAS :execrows
-- Removes a vanished entry whose Asset is present elsewhere (a move), or an
-- entry that never held an Asset. It never removes an Asset's last entry.
DELETE FROM repository_entries
WHERE entry_id = sqlc.arg(entry_id)
  AND revision = sqlc.arg(expected_revision);

-- name: ListLiveRepositoryEntriesUnder :many
-- One path-ordered page of the live rows at or below a vanished directory.
SELECT * FROM repository_entries
WHERE repository_id = sqlc.arg(repository_id)
  AND (path = sqlc.arg(directory) OR (path > sqlc.arg(directory) || '/' AND path < sqlc.arg(directory) || '0'))
  AND path > sqlc.arg(after_path)
  AND state IN ('pending_hash', 'present', 'unsupported')
ORDER BY path
LIMIT sqlc.arg(page_limit);

-- name: ListPendingHashRepositoryEntries :many
SELECT * FROM repository_entries
WHERE repository_id = ?1
  AND state = 'pending_hash'
ORDER BY entry_id
LIMIT ?2;

-- name: BindRepositoryEntryCAS :execrows
-- The hash commit. The stat tuple is the one the content was hashed under,
-- which InspectMedia proved stable; revision proves the row did not move on.
UPDATE repository_entries
SET state = 'present',
    content_id = sqlc.arg(content_id),
    asset_id = sqlc.arg(asset_id),
    size = sqlc.arg(size),
    mtime_ns = sqlc.arg(mtime_ns),
    ctime_ns = sqlc.narg(ctime_ns),
    file_id = sqlc.narg(file_id),
    quick_fingerprint = sqlc.narg(quick_fingerprint),
    quick_fingerprint_version = sqlc.narg(quick_fingerprint_version),
    stat_checked_ns = sqlc.arg(stat_checked_ns),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE entry_id = sqlc.arg(entry_id)
  AND revision = sqlc.arg(expected_revision)
  AND state = 'pending_hash';

-- name: SetRepositoryEntryStateCAS :execrows
-- Used when hashing finds the entry unsupported, or its stat tuple has moved
-- on: the row keeps its binding and takes the new tuple.
UPDATE repository_entries
SET state = sqlc.arg(state),
    size = sqlc.arg(size),
    mtime_ns = sqlc.arg(mtime_ns),
    ctime_ns = sqlc.narg(ctime_ns),
    file_id = sqlc.narg(file_id),
    stat_checked_ns = sqlc.arg(stat_checked_ns),
    revision = revision + 1,
    updated_at = sqlc.arg(updated_at)
WHERE entry_id = sqlc.arg(entry_id)
  AND revision = sqlc.arg(expected_revision);

-- name: HasOtherPresentRepositoryEntry :one
SELECT EXISTS (
    SELECT 1 FROM repository_entries
    WHERE asset_id = sqlc.arg(asset_id)
      AND entry_id <> sqlc.arg(entry_id)
      AND state = 'present'
) AS present;

-- name: DeleteMissingRepositoryEntriesForAsset :execrows
-- A present entry for the Asset now exists, so its missing entries were
-- moves or deletions of copies, removed like Syncthing's findRename.
DELETE FROM repository_entries
WHERE asset_id = sqlc.arg(asset_id)
  AND state = 'missing';

-- name: RepointAssetContent :execrows
-- In-place carry-over branch 1: the Asset keeps its ID and user metadata and
-- takes the new content. UNIQUE(owner_id, content_id) is checked here.
UPDATE assets
SET content_id = sqlc.arg(content_id),
    updated_at = sqlc.arg(updated_at)
WHERE asset_id = sqlc.arg(asset_id)
  AND content_id = sqlc.arg(expected_content_id);

-- name: CopyAssetUserMetadata :exec
-- In-place carry-over branch 2: the new Asset inherits rating, like, and the
-- user-edited description of the Asset whose copy was edited.
UPDATE assets
SET rating = (SELECT source.rating FROM assets source WHERE source.asset_id = sqlc.arg(source_asset_id)),
    liked = (SELECT source.liked FROM assets source WHERE source.asset_id = sqlc.arg(source_asset_id)),
    specific_metadata = COALESCE((
        SELECT json_set(COALESCE(assets.specific_metadata, '{}'), '$.description',
                        json_extract(source.specific_metadata, '$.description'))
        FROM assets source
        WHERE source.asset_id = sqlc.arg(source_asset_id)
          AND json_extract(source.specific_metadata, '$.description') IS NOT NULL
    ), specific_metadata)
WHERE assets.asset_id = sqlc.arg(target_asset_id);

-- name: CopyAssetAlbumMemberships :exec
INSERT OR IGNORE INTO album_assets (album_id, asset_id, position, added_time)
SELECT source.album_id, sqlc.arg(target_asset_id), source.position, source.added_time
FROM album_assets source
WHERE source.asset_id = sqlc.arg(source_asset_id);

-- name: CopyAssetUserTags :exec
INSERT OR IGNORE INTO asset_tags (asset_id, tag_id, confidence, source)
SELECT sqlc.arg(target_asset_id), original.tag_id, original.confidence, original.source
FROM asset_tags original
WHERE original.asset_id = sqlc.arg(source_asset_id)
  AND original.source = 'user';

-- name: GetQueuedRepositoryScan :one
SELECT * FROM repository_scans
WHERE repository_id = ?1 AND status = 'queued';

-- name: GetRunningRepositoryScan :one
SELECT * FROM repository_scans
WHERE repository_id = ?1 AND status IN ('walking', 'sweeping');

-- name: GetRepositoryScan :one
SELECT * FROM repository_scans
WHERE scan_id = ?1;

-- name: InsertRepositoryScan :one
INSERT INTO repository_scans (
    scan_id, repository_id, trigger, scope_path, status, not_before,
    requested_by, created_at, updated_at
) VALUES (
    sqlc.arg(scan_id), sqlc.arg(repository_id), sqlc.arg(trigger), sqlc.arg(scope_path),
    'queued', sqlc.arg(not_before), sqlc.narg(requested_by), sqlc.arg(created_at), sqlc.arg(created_at)
)
RETURNING *;

-- name: CoalesceQueuedRepositoryScan :one
-- A new request joins the queued scan: its scope widens to cover both, and
-- the earlier of the two start times wins (NULL means now).
UPDATE repository_scans
SET scope_path = sqlc.arg(scope_path),
    not_before = sqlc.arg(not_before),
    updated_at = sqlc.arg(updated_at)
WHERE scan_id = sqlc.arg(scan_id) AND status = 'queued'
RETURNING *;

-- name: StartRepositoryScan :execrows
UPDATE repository_scans
SET status = 'walking',
    started_at = sqlc.arg(started_at),
    updated_at = sqlc.arg(started_at)
WHERE scan_id = sqlc.arg(scan_id) AND status = 'queued';

-- name: AdvanceRepositoryScan :execrows
-- Commits one directory's progress. resume_after_path moves only forward
-- inside the same transaction as the directory's entry writes.
UPDATE repository_scans
SET status = sqlc.arg(status),
    resume_after_path = sqlc.narg(resume_after_path),
    seen = seen + sqlc.arg(seen),
    new_entries = new_entries + sqlc.arg(new_entries),
    changed = changed + sqlc.arg(changed),
    missing = missing + sqlc.arg(missing),
    restored = restored + sqlc.arg(restored),
    moved = moved + sqlc.arg(moved),
    deferred = deferred + sqlc.arg(deferred),
    errors = errors + sqlc.arg(errors),
    updated_at = sqlc.arg(updated_at)
WHERE scan_id = sqlc.arg(scan_id) AND status IN ('walking', 'sweeping');

-- name: FinishRepositoryScan :execrows
UPDATE repository_scans
SET status = sqlc.arg(status),
    error_code = sqlc.narg(error_code),
    finished_at = sqlc.arg(finished_at),
    updated_at = sqlc.arg(finished_at)
WHERE scan_id = sqlc.arg(scan_id) AND status IN ('queued', 'walking', 'sweeping');

-- name: GetLatestStartedRepositoryScanID :one
-- Hashing is per entry, not per scan; its progress is credited to the
-- repository's most recent started scan so the scan status shows it.
SELECT scan_id FROM repository_scans
WHERE repository_id = ?1 AND status <> 'queued'
ORDER BY created_at DESC, scan_id DESC
LIMIT 1;

-- name: AddRepositoryScanHashProgress :exec
UPDATE repository_scans
SET hashed = hashed + 1,
    bytes_hashed = bytes_hashed + sqlc.arg(hashed_bytes),
    moved = moved + sqlc.arg(moved_entries),
    restored = restored + sqlc.arg(restored_entries),
    updated_at = sqlc.arg(updated_at)
WHERE scan_id = sqlc.arg(scan_id);

-- name: ListRepositoryScans :many
SELECT * FROM repository_scans
WHERE repository_id = ?1
ORDER BY created_at DESC, scan_id DESC
LIMIT ?2 OFFSET ?3;

-- name: GetLiveRepositoryEntryByKey :one
SELECT * FROM repository_entries
WHERE repository_id = ?1
  AND path_key = ?2
  AND state IN ('pending_hash', 'present');

-- name: ListPresentRepositoryEntriesForAsset :many
-- The occurrences media I/O may open, in a stable preference order.
SELECT * FROM repository_entries
WHERE asset_id = ?1
  AND state = 'present'
  AND kind = 'file'
ORDER BY repository_id, path;

-- name: GetLatestRepositoryScan :one
SELECT * FROM repository_scans
WHERE repository_id = ?1
ORDER BY created_at DESC, scan_id DESC
LIMIT 1;

-- name: RequestRepositoryScanCancellation :one
UPDATE repository_scans
SET cancellation_requested = 1,
    updated_at = sqlc.arg(updated_at)
WHERE scan_id = sqlc.arg(scan_id)
  AND repository_id = sqlc.arg(repository_id)
RETURNING *;

