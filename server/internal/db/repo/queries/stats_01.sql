-- name: GetCameraLensStats :many
SELECT
    json_extract(a.specific_metadata, char(36) || '.camera_model') AS camera_model,
    json_extract(a.specific_metadata, char(36) || '.lens_model') AS lens_model,
    COUNT(*) AS count
FROM assets a
WHERE
    a.is_deleted = false
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND json_extract(a.specific_metadata, char(36) || '.camera_model') IS NOT NULL
    AND json_extract(a.specific_metadata, char(36) || '.camera_model') != ''
    AND json_extract(a.specific_metadata, char(36) || '.lens_model') IS NOT NULL
    AND json_extract(a.specific_metadata, char(36) || '.lens_model') != ''
GROUP BY camera_model, lens_model
ORDER BY count DESC
LIMIT sqlc.arg('limit');
