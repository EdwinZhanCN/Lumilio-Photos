-- name: AgentFacetTimeHistogram :many
-- granularity is 'hour', 'day', 'month' or 'year'.
WITH filter_params AS (
  SELECT CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json
)
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
WHERE a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
  AND a.is_deleted = false
GROUP BY 1
ORDER BY 1;
