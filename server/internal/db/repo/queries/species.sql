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
