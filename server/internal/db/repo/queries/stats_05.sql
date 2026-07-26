-- name: GetAvailableYears :many
SELECT DISTINCT
    CAST(strftime('%Y', COALESCE(a.taken_time, a.upload_time) / 1000000, 'unixepoch') AS INTEGER) AS year
FROM assets a
WHERE
    a.is_deleted = false
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (a.taken_time IS NOT NULL OR a.upload_time IS NOT NULL)
ORDER BY year DESC;
