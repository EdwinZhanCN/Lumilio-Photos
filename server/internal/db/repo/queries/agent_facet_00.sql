-- name: AgentFacetOverview :one
SELECT
    COUNT(*) AS total,
    MIN(COALESCE(a.taken_time, a.upload_time)) AS date_from,
    MAX(COALESCE(a.taken_time, a.upload_time)) AS date_to,
    COUNT(*) FILTER (WHERE a.liked = true) AS liked_count,
    COALESCE(MIN(a.capture_offset_minutes), 0) AS capture_offset_minutes
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.is_deleted = false;
