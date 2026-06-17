-- name: GetRepositoryDefaults :one
SELECT * FROM repository_defaults
WHERE id = 1;

-- name: UpsertRepositoryDefaults :one
INSERT INTO repository_defaults (
    id,
    strategy,
    duplicate_handling
) VALUES (
    1, $1, $2
)
ON CONFLICT (id) DO UPDATE SET
    strategy = EXCLUDED.strategy,
    duplicate_handling = EXCLUDED.duplicate_handling,
    updated_at = NOW()
RETURNING *;
