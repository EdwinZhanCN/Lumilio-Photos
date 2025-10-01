-- name: CreateRepository :one
INSERT INTO repositories (
    repo_id,
    name,
    path,
    config,
    status,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetRepository :one
SELECT * FROM repositories
WHERE repo_id = $1;

-- name: GetRepositoryByPath :one
SELECT * FROM repositories
WHERE path = $1;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY created_at DESC;

-- name: ListActiveRepositories :many
SELECT * FROM repositories
WHERE status = 'active'
ORDER BY created_at DESC;

-- name: UpdateRepository :one
UPDATE repositories
SET
    name = $2,
    config = $3,
    status = $4,
    updated_at = $5
WHERE repo_id = $1
RETURNING *;

-- name: UpdateRepositoryStatus :one
UPDATE repositories
SET
    status = $2,
    updated_at = $3
WHERE repo_id = $1
RETURNING *;

-- name: UpdateRepositoryLastSync :one
UPDATE repositories
SET
    last_sync = $2,
    updated_at = $3
WHERE repo_id = $1
RETURNING *;

-- name: DeleteRepository :exec
DELETE FROM repositories
WHERE repo_id = $1;

-- name: DeleteRepositories :exec
DELETE FROM repositories
WHERE repo_id = ANY($1::uuid[]);

-- name: RepositoryExists :one
SELECT EXISTS(
    SELECT 1 FROM repositories
    WHERE path = $1
);

-- name: CountRepositories :one
SELECT COUNT(*) FROM repositories;

-- name: CountRepositoriesByStatus :one
SELECT COUNT(*) FROM repositories
WHERE status = $1;
