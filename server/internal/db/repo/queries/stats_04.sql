-- name: GetDailyActivityHeatmap :many
SELECT
    strftime('%Y-%m-%d', COALESCE(a.taken_time, a.upload_time) / 1000000, 'unixepoch') AS date,
    COUNT(*) AS count
FROM assets a
WHERE
    a.is_deleted = false
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (a.taken_time IS NOT NULL OR a.upload_time IS NOT NULL)
    AND COALESCE(a.taken_time, a.upload_time) >= sqlc.arg('start_time')
    AND COALESCE(a.taken_time, a.upload_time) <= sqlc.arg('end_time')
GROUP BY date
ORDER BY date;
