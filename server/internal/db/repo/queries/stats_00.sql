-- name: GetFocalLengthDistribution :many
SELECT
    json_extract(a.specific_metadata, char(36) || '.focal_length') AS focal_length,
    COUNT(*) AS count
FROM assets a
WHERE
    a.is_deleted = false
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND json_type(a.specific_metadata, char(36) || '.focal_length') IN ('integer', 'real')
    AND json_extract(a.specific_metadata, char(36) || '.focal_length') > 0
GROUP BY focal_length
ORDER BY count DESC
LIMIT 50;
