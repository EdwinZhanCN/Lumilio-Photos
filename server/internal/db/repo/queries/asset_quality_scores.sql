-- Asset quality scores: per-asset aesthetic score from MLP head on SigLIP.

-- name: UpsertAssetQualityScore :one
INSERT INTO asset_quality_scores (asset_id, score, model_version)
VALUES ($1, $2, $3)
ON CONFLICT (asset_id)
DO UPDATE SET
    score = EXCLUDED.score,
    model_version = EXCLUDED.model_version,
    updated_at = NOW()
RETURNING *;

-- name: GetAssetQualityScore :one
SELECT asset_id, score, model_version, created_at, updated_at
FROM asset_quality_scores
WHERE asset_id = $1;
