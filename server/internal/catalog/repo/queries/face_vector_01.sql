-- name: GetSimilarFaces :many
SELECT
    fi.*,
    CAST(1.0 - vec1_cos_distance(fi.embedding, sqlc.arg('embedding_query')) AS REAL) AS similarity
FROM face_items fi
WHERE fi.id != sqlc.arg('id')
AND fi.embedding IS NOT NULL
AND 1.0 - vec1_cos_distance(fi.embedding, sqlc.arg('embedding_query'))
    >= CAST(sqlc.arg('min_similarity') AS REAL)
ORDER BY similarity DESC
LIMIT sqlc.arg('limit');
