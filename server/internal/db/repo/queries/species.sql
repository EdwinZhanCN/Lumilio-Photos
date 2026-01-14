-- name: CreateSpeciesPrediction :one
INSERT INTO species_predictions (asset_id, label, score)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetSpeciesPredictionsByAsset :many
SELECT * FROM species_predictions
WHERE asset_id = $1
ORDER BY score DESC;

-- name: DeleteSpeciesPredictionsByAsset :exec
DELETE FROM species_predictions WHERE asset_id = $1;

-- name: GetSpeciesPredictionsByLabel :many
SELECT * FROM species_predictions
WHERE label = $1
ORDER BY score DESC
LIMIT $2 OFFSET $3;

-- name: GetTopSpeciesForAsset :many
SELECT * FROM species_predictions
WHERE asset_id = $1 AND score >= $2
ORDER BY score DESC
LIMIT $3;

-- name: SearchAssetsBySpecies :many
SELECT DISTINCT a.* FROM assets a
JOIN species_predictions sp ON a.asset_id = sp.asset_id
WHERE sp.label ILIKE '%' || $1 || '%'
AND a.is_deleted = false
ORDER BY sp.score DESC
LIMIT $3 OFFSET $2;

-- name: GetSpeciesStats :one
SELECT
    COUNT(DISTINCT asset_id) as total_assets,
    COUNT(*) as total_predictions,
    AVG(score) as avg_score
FROM species_predictions;

-- name: GetTopSpeciesLabels :many
SELECT label, COUNT(DISTINCT asset_id) as asset_count, AVG(score) as avg_score
FROM species_predictions
GROUP BY label
ORDER BY asset_count DESC
LIMIT $1;
