-- name: UpsertRepositoryRoot :one
INSERT INTO repository_roots (
    root_id,
    name,
    path,
    kind,
    status,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (root_id) DO UPDATE SET
    name = EXCLUDED.name,
    path = EXCLUDED.path,
    kind = EXCLUDED.kind,
    status = EXCLUDED.status,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetRepositoryRoot :one
SELECT * FROM repository_roots
WHERE root_id = $1;

-- name: GetRepositoryRootByPath :one
SELECT * FROM repository_roots
WHERE path = $1;

-- name: GetDefaultRepositoryRoot :one
SELECT * FROM repository_roots
WHERE kind = 'default';

-- name: ListRepositoryRoots :many
SELECT * FROM repository_roots
ORDER BY kind ASC, created_at ASC;

-- name: UpdateRepositoryRootFromDisk :one
UPDATE repository_roots
SET
    name = $2,
    status = $3,
    updated_at = $4
WHERE root_id = $1
RETURNING *;

-- name: DeleteExternalRepositoryRoot :execrows
DELETE FROM repository_roots
WHERE repository_roots.root_id = $1
  AND repository_roots.kind = 'external'
  AND NOT EXISTS (
      SELECT 1 FROM repositories WHERE repositories.root_id = repository_roots.root_id
  );

-- name: SetRepositoryRoot :one
UPDATE repositories
SET root_id = $2, updated_at = $3
WHERE repo_id = $1
RETURNING *;
