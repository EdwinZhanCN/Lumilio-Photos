-- name: CreateRepository :one
INSERT INTO repositories (
    repo_id,
    name,
    path,
    config,
    role,
    reachability,
    activity,
    default_owner_id,
    created_at,
    updated_at,
    root_id
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11
) RETURNING *;

-- name: GetRepository :one
SELECT * FROM repositories
WHERE repo_id = ?1;

-- name: GetRepositoryByPath :one
SELECT * FROM repositories
WHERE path = ?1;

-- name: GetPrimaryRepository :one
SELECT * FROM repositories
WHERE role = 'primary'
  AND reachability = 'active';

-- name: GetPrimaryRepositoryRecord :one
SELECT * FROM repositories
WHERE role = 'primary';

-- name: GetHostOwnerID :one
-- The primary repository pins the Host Owner after bootstrap. Before the
-- primary exists, the first account is the initial administrator and therefore
-- the Host Owner.
SELECT candidate.owner_id AS host_owner_id
FROM (
    SELECT default_owner_id AS owner_id, 0 AS priority, created_at, repo_id AS tie_breaker
    FROM repositories
    WHERE role = 'primary'
      AND default_owner_id IS NOT NULL

    UNION ALL

    SELECT user_id AS owner_id, 1 AS priority, created_at, user_id AS tie_breaker
    FROM users
) candidate
ORDER BY candidate.priority ASC, candidate.created_at ASC, candidate.tie_breaker ASC
LIMIT 1;

-- name: ListRepositories :many
SELECT * FROM repositories
ORDER BY created_at DESC;

-- name: ListActiveRepositories :many
SELECT * FROM repositories
WHERE reachability = 'active'
ORDER BY created_at DESC;

-- name: CountPrimaryRepositories :one
SELECT COUNT(*) FROM repositories
WHERE role = 'primary';

-- Reachability and activity are deliberately absent: their state machines own
-- those columns. Letting a settings edit write reachability resurrects a repository that reconcile
-- has marked offline.
-- name: UpdateRepository :one
UPDATE repositories
SET
    name = ?2,
    config = ?3,
    default_owner_id = ?4,
    updated_at = ?5
WHERE repo_id = ?1
RETURNING *;

-- name: UpdateRepositoryPath :one
UPDATE repositories
SET
    path = ?2,
    root_id = ?3,
    reachability = ?4,
    updated_at = ?5
WHERE repo_id = ?1
RETURNING *;

-- name: UpdateRepositoryReachability :one
UPDATE repositories
SET
    reachability = ?2,
    updated_at = ?3
WHERE repo_id = ?1
RETURNING *;

-- name: UpdateRepositoryActivity :one
UPDATE repositories
SET
    activity = ?2,
    pause_reason = CASE WHEN ?2 = 'paused' THEN 'manual' ELSE '' END,
    updated_at = ?3
WHERE repo_id = ?1
RETURNING *;

-- name: PauseRepositoryForLowSpace :one
UPDATE repositories
SET activity = 'paused',
    pause_reason = 'low_space',
    updated_at = ?2
WHERE repo_id = ?1
  AND reachability = 'active'
  AND (activity <> 'paused' OR pause_reason = 'low_space')
RETURNING *;

-- name: ResumeRepositoryAfterLowSpace :one
UPDATE repositories
SET activity = 'idle',
    pause_reason = '',
    updated_at = ?2
WHERE repo_id = ?1
  AND reachability = 'active'
  AND activity = 'paused'
  AND pause_reason = 'low_space'
RETURNING *;

-- name: BeginRepositoryActivity :one
UPDATE repositories
SET
    activity = ?2,
    updated_at = ?3
WHERE repo_id = ?1
  AND reachability = 'active'
  AND activity = 'idle'
RETURNING *;

-- name: BeginRepositoryMaintenance :one
UPDATE repositories
SET reachability = 'maintenance',
    activity = 'paused',
    pause_reason = 'maintenance',
    updated_at = ?2
WHERE repo_id = ?1
  AND reachability <> 'maintenance'
  AND activity = 'idle'
RETURNING *;

-- name: EndRepositoryMaintenance :one
UPDATE repositories
SET reachability = ?2,
    activity = ?3,
    pause_reason = CASE WHEN ?3 = 'paused' THEN 'manual' ELSE '' END,
    updated_at = ?4
WHERE repo_id = ?1
  AND reachability = 'maintenance'
  AND activity = 'paused'
RETURNING *;

-- name: FinishRepositoryActivity :execrows
UPDATE repositories
SET
    activity = 'idle',
    pause_reason = '',
    updated_at = ?3
WHERE repo_id = ?1
  AND activity = ?2;

-- name: ResetRepositoriesByActivity :execrows
UPDATE repositories
SET
    activity = 'idle',
    pause_reason = '',
    updated_at = ?2
WHERE activity = ?1;

-- name: UpdateRepositoryLastSync :one
UPDATE repositories
SET
    last_sync = ?2,
    updated_at = ?3
WHERE repo_id = ?1
RETURNING *;

-- name: DeleteRepository :exec
DELETE FROM repositories
WHERE repo_id = ?1;

-- name: DeleteRepositories :exec
DELETE FROM repositories
WHERE repo_id IN (sqlc.slice('repo_ids'));

-- name: RepositoryExists :one
SELECT EXISTS(
    SELECT 1 FROM repositories
    WHERE path = ?1
);

-- name: CountRepositories :one
SELECT COUNT(*) FROM repositories;

-- name: CountRepositoriesByReachability :one
SELECT COUNT(*) FROM repositories
WHERE reachability = ?1;

-- name: SetUnownedRepositoryHostOwner :exec
UPDATE repositories
SET
    default_owner_id = ?1,
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE default_owner_id IS NULL;
