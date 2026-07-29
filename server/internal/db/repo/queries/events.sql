-- name: ListEventsPage :many
SELECT *
FROM events
WHERE owner_id = ?
  AND status = 'active'
  AND is_hidden = 0
  AND (
    sqlc.narg(cursor_start_at) IS NULL
    OR start_at < sqlc.narg(cursor_start_at)
    OR (start_at = sqlc.narg(cursor_start_at) AND event_id < sqlc.narg(cursor_event_id))
  )
ORDER BY start_at DESC, event_id DESC
LIMIT ?;

-- name: GetEventForOwner :one
SELECT * FROM events WHERE event_id = ? AND owner_id = ?;

-- name: GetEventRedirectForOwner :one
SELECT r.new_event_id
FROM event_redirects r
JOIN events target
  ON target.event_id = r.new_event_id AND target.owner_id = r.owner_id
WHERE r.old_event_id = ? AND r.owner_id = ? AND target.status = 'active';

-- name: ListEventMembership :many
SELECT *
FROM event_media_items
WHERE event_id = ? AND owner_id = ?
ORDER BY position, media_item_id;

-- name: ListEventMembershipPage :many
SELECT *
FROM event_media_items
WHERE event_id = ? AND owner_id = ?
  AND (
    position > sqlc.arg(cursor_position)
    OR (position = sqlc.arg(cursor_position) AND media_item_id > sqlc.arg(cursor_media_item_id))
  )
ORDER BY position, media_item_id
LIMIT ?;

-- name: ListEventConstraints :many
SELECT * FROM event_constraints WHERE owner_id = ?
ORDER BY kind, event_id, left_media_item_id, right_media_item_id;

-- name: InsertEvent :exec
INSERT INTO events (
  event_id, owner_id, status, start_at, end_at, timezone, generated_title,
  title_override, generated_cover_media_item_id, cover_override_media_item_id,
  is_hidden, algorithm_version, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, NULL, ?, NULL, ?, ?, ?, ?, ?);

-- name: InsertEventMembership :exec
INSERT INTO event_media_items (
  event_id, owner_id, media_item_id, position, source, confidence, evidence,
  derivation_run_id, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteEventMemberships :exec
DELETE FROM event_media_items WHERE event_id = ? AND owner_id = ?;

-- name: UpdateEventBoundsAndGeneratedCover :exec
UPDATE events
SET start_at = ?, end_at = ?, timezone = ?, generated_title = NULL,
    generated_cover_media_item_id = ?, updated_at = ?
WHERE event_id = ? AND owner_id = ? AND status = 'active';

-- name: UpdateEventOverrides :exec
UPDATE events
SET title_override = ?, cover_override_media_item_id = ?, is_hidden = ?, updated_at = ?
WHERE event_id = ? AND owner_id = ? AND status = 'active';

-- name: ClearEventCovers :exec
UPDATE events
SET generated_cover_media_item_id = NULL, cover_override_media_item_id = NULL,
    updated_at = ?
WHERE event_id = ? AND owner_id = ?;

-- name: MarkEventRedirected :exec
UPDATE events SET status = 'redirected', updated_at = ?
WHERE event_id = ? AND owner_id = ? AND status = 'active';

-- name: RewriteRedirectTargets :exec
UPDATE event_redirects SET new_event_id = ?
WHERE owner_id = ? AND new_event_id = ?;

-- name: InsertEventRedirect :exec
INSERT INTO event_redirects (old_event_id, owner_id, new_event_id, reason, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: InsertEventConstraint :exec
INSERT INTO event_constraints (
  constraint_id, owner_id, kind, event_id, left_media_item_id,
  right_media_item_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeleteEventConstraint :exec
DELETE FROM event_constraints WHERE constraint_id = ? AND owner_id = ?;

-- name: UpsertEventOwnerState :exec
INSERT INTO event_owner_state (
  owner_id, active_algorithm_version, initialized_at, last_full_rebuild_at,
  automatic_rebuild_paused, revision, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(owner_id) DO NOTHING;

-- name: GetEventOwnerState :one
SELECT * FROM event_owner_state WHERE owner_id = ?;

-- name: SetEventRebuildPaused :execrows
UPDATE event_owner_state
SET automatic_rebuild_paused = ?, revision = revision + 1, updated_at = ?
WHERE owner_id = ?;

-- name: CompareAndAdvanceEventRevision :execrows
UPDATE event_owner_state
SET revision = revision + 1, updated_at = ?
WHERE owner_id = ? AND revision = ? AND automatic_rebuild_paused = 0;

-- name: InsertEventDirtyRange :exec
INSERT INTO event_dirty_ranges (
  dirty_range_id, owner_id, range_start, range_end, reason, created_at
) VALUES (?, ?, ?, ?, ?, ?);

-- name: ClaimEventDirtyRanges :execrows
UPDATE event_dirty_ranges
SET claimed_at = ?, claim_token = ?
WHERE owner_id = ? AND claim_token IS NULL;

-- name: ListClaimedEventDirtyRanges :many
SELECT * FROM event_dirty_ranges
WHERE owner_id = ? AND claim_token = ?
ORDER BY range_start, range_end, dirty_range_id;

-- name: RenewEventDirtyClaim :execrows
UPDATE event_dirty_ranges
SET claimed_at = ?
WHERE owner_id = ? AND claim_token = ?;

-- name: DeleteClaimedEventDirtyRanges :execrows
DELETE FROM event_dirty_ranges WHERE owner_id = ? AND claim_token = ?;

-- name: ClearStaleEventDirtyClaims :execrows
UPDATE event_dirty_ranges
SET claimed_at = NULL, claim_token = NULL
WHERE claimed_at IS NOT NULL AND claimed_at < ?;

-- name: ListOwnersWithEventDirtyRanges :many
SELECT DISTINCT owner_id FROM event_dirty_ranges
WHERE claim_token IS NULL OR claimed_at < ?
ORDER BY owner_id;

-- name: EventCandidateTimeBounds :one
SELECT
  min(COALESCE(a.taken_time, a.upload_time, mi.created_at)) AS range_start,
  max(COALESCE(a.taken_time, a.upload_time, mi.created_at)) AS range_end
FROM media_items mi
LEFT JOIN assets a ON a.asset_id = mi.primary_asset_id
WHERE mi.owner_id = ?;

-- name: ListEventCandidates :many
SELECT
  mi.media_item_id,
  COALESCE(capture.taken_time, upload.upload_time, mi.created_at) AS captured_at,
  CASE
    WHEN capture.taken_time IS NOT NULL THEN 'taken_time'
    WHEN upload.upload_time IS NOT NULL THEN 'upload_time'
    ELSE 'created_at'
  END AS time_source,
  CASE WHEN capture.taken_time IS NOT NULL THEN capture.capture_offset_minutes END AS capture_offset_minutes,
  COALESCE(primary_asset.gps_latitude, component.gps_latitude) AS gps_latitude,
  COALESCE(primary_asset.gps_longitude, component.gps_longitude) AS gps_longitude,
  COALESCE(asm.stack_id, '') AS stack_key
FROM media_items mi
LEFT JOIN assets primary_asset ON primary_asset.asset_id = mi.primary_asset_id
LEFT JOIN asset_stack_members asm ON asm.media_item_id = mi.media_item_id
LEFT JOIN assets capture ON capture.asset_id = (
  SELECT mia.asset_id
  FROM media_item_assets mia
  JOIN assets ca ON ca.asset_id = mia.asset_id
  WHERE mia.media_item_id = mi.media_item_id AND ca.taken_time IS NOT NULL
  ORDER BY ca.taken_time, ca.asset_id
  LIMIT 1
)
LEFT JOIN assets upload ON upload.asset_id = (
  SELECT mia.asset_id
  FROM media_item_assets mia
  JOIN assets ua ON ua.asset_id = mia.asset_id
  WHERE mia.media_item_id = mi.media_item_id
  ORDER BY ua.upload_time, ua.asset_id
  LIMIT 1
)
LEFT JOIN assets component ON component.asset_id = (
  SELECT mia.asset_id
  FROM media_item_assets mia
  JOIN assets ga ON ga.asset_id = mia.asset_id
  WHERE mia.media_item_id = mi.media_item_id
    AND ga.gps_latitude IS NOT NULL AND ga.gps_longitude IS NOT NULL
  ORDER BY CASE mia.relation
    WHEN 'live_photo_still' THEN 0 WHEN 'jpeg_original' THEN 1
    WHEN 'raw_original' THEN 2 WHEN 'original' THEN 3
    WHEN 'edited_version' THEN 4 WHEN 'alternative' THEN 5
    WHEN 'component' THEN 6 WHEN 'live_photo_video' THEN 7 ELSE 8 END,
    mia.position, mia.asset_id
  LIMIT 1
)
WHERE mi.owner_id = ?
ORDER BY captured_at, mi.media_item_id;

-- name: GetEventAssetIDsForAgent :many
SELECT COALESCE(
  CASE WHEN primary_asset.is_deleted = 0 THEN primary_asset.asset_id END,
  (
    SELECT a.asset_id FROM media_item_assets mia
    JOIN assets a ON a.asset_id=mia.asset_id
    WHERE mia.media_item_id=emi.media_item_id
      AND a.owner_id=emi.owner_id AND a.is_deleted=0
    ORDER BY mia.position,mia.asset_id LIMIT 1
  )
) AS resolved_asset_id
FROM event_media_items emi
JOIN events e ON e.event_id=emi.event_id AND e.owner_id=emi.owner_id
JOIN media_items mi ON mi.media_item_id=emi.media_item_id AND mi.owner_id=emi.owner_id
LEFT JOIN assets primary_asset ON primary_asset.asset_id=mi.primary_asset_id
WHERE emi.event_id=? AND emi.owner_id=? AND e.status='active'
ORDER BY emi.position,emi.media_item_id
LIMIT ?;

-- name: AgentLookupEvents :many
SELECT e.event_id,e.start_at,e.end_at,e.title_override,count(emi.media_item_id) AS media_count
FROM events e
JOIN event_media_items emi ON emi.event_id=e.event_id AND emi.owner_id=e.owner_id
WHERE e.owner_id=? AND e.status='active'
  AND (sqlc.narg(title_query) IS NULL OR e.title_override LIKE '%' || sqlc.narg(title_query) || '%')
GROUP BY e.event_id
ORDER BY e.start_at DESC,e.event_id DESC
LIMIT ?;
