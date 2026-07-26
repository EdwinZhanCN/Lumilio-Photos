-- name: AgentFacetTimeHistogram :many
-- granularity is 'hour', 'day', 'month' or 'year'.
SELECT
    strftime(
        CASE
            WHEN sqlc.arg('granularity') = 'hour' THEN '%Y-%m-%d %H:00'
            WHEN sqlc.arg('granularity') = 'day' THEN '%Y-%m-%d'
            WHEN sqlc.arg('granularity') = 'year' THEN '%Y'
            ELSE '%Y-%m'
        END,
        COALESCE(a.taken_time, a.upload_time) / 1000000,
        'unixepoch'
    ) AS bucket,
    COUNT(*) AS count
FROM assets a
WHERE a.asset_id IN (sqlc.slice('asset_ids'))
  AND a.is_deleted = false
GROUP BY 1
ORDER BY 1;

