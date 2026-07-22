-- name: CreateRepository :one
INSERT INTO repositories (
    repo_id,
    name,
    path,
    config,
    role,
    status,
    default_owner_id,
    created_at,
    updated_at,
    root_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) RETURNING *;

-- name: GetRepository :one
SELECT * FROM repositories
WHERE repo_id = $1;

-- name: GetRepositoryByPath :one
SELECT * FROM repositories
WHERE path = $1;

-- name: GetPrimaryRepository :one
SELECT * FROM repositories
WHERE role = 'primary'
  AND status = 'active';

-- name: GetHostOwnerID :one
-- The primary repository pins the Host Owner after bootstrap. Before the
-- primary exists, the first account is the initial administrator and therefore
-- the Host Owner.
SELECT candidate.owner_id::integer AS host_owner_id
FROM (
    SELECT default_owner_id AS owner_id, 0 AS priority, created_at, repo_id::text AS tie_breaker
    FROM repositories
    WHERE role = 'primary'
      AND default_owner_id IS NOT NULL

    UNION ALL

    SELECT user_id AS owner_id, 1 AS priority, created_at, user_id::text AS tie_breaker
    FROM users
) candidate
ORDER BY candidate.priority ASC, candidate.created_at ASC, candidate.tie_breaker ASC
LIMIT 1;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY created_at DESC;

-- name: ListActiveRepositories :many
SELECT * FROM repositories
WHERE status = 'active'
ORDER BY created_at DESC;

-- name: CountPrimaryRepositories :one
SELECT COUNT(*) FROM repositories
WHERE role = 'primary';

-- Status is deliberately absent: it is owned by UpdateRepositoryStatus alone.
-- Letting a settings edit write status resurrects a repository that reconcile
-- has marked offline.
-- name: UpdateRepository :one
UPDATE repositories
SET
    name = $2,
    config = $3,
    default_owner_id = $4,
    updated_at = $5
WHERE repo_id = $1
RETURNING *;

-- name: UpdateRepositoryPath :one
UPDATE repositories
SET
    path = $2,
    root_id = $3,
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

-- name: SetUnownedRepositoryHostOwner :exec
UPDATE repositories
SET
    default_owner_id = $1,
    updated_at = NOW()
WHERE default_owner_id IS NULL;
