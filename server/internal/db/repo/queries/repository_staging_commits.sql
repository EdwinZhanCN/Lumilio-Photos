-- name: CreateRepositoryStagingCommit :one
INSERT INTO repository_staging_commits (
    commit_id,
    repository_id,
    owner_id,
    source_kind,
    staging_path,
    original_filename,
    mime_type,
    full_hash,
    file_size,
    quick_fingerprint,
    quick_fingerprint_version,
    status,
    created_at,
    updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, 'prepared', ?12, ?12)
RETURNING *;

-- name: GetRepositoryStagingCommit :one
SELECT * FROM repository_staging_commits WHERE commit_id = ?1;

-- name: ClaimRepositoryStagingCommit :one
UPDATE repository_staging_commits
SET status = 'committing', updated_at = ?2
WHERE commit_id = ?1
  AND status IN ('prepared', 'committing', 'committed')
RETURNING *;

-- name: SetRepositoryStagingCommitTarget :one
UPDATE repository_staging_commits
SET target_path = ?2, updated_at = ?3
WHERE commit_id = ?1
  AND status IN ('prepared', 'committing')
RETURNING *;

-- name: MarkRepositoryStagingCommitOnDisk :one
UPDATE repository_staging_commits
SET status = 'committed', target_path = ?2, updated_at = ?3
WHERE commit_id = ?1
  AND status IN ('prepared', 'committing', 'committed')
RETURNING *;

-- name: CompleteRepositoryStagingCommit :one
UPDATE repository_staging_commits
SET status = 'completed', node_id = ?2, asset_id = ?3,
    failure_code = NULL, failure_detail = NULL,
    completed_at = ?4, updated_at = ?4
WHERE commit_id = ?1
  AND status IN ('committing', 'committed', 'completed')
RETURNING *;

-- name: QuarantineRepositoryStagingCommit :one
UPDATE repository_staging_commits
SET status = 'quarantined', staging_path = ?2,
    failure_code = ?3, failure_detail = ?4, updated_at = ?5
WHERE commit_id = ?1
  AND status <> 'completed'
RETURNING *;

-- name: ListRecoverableRepositoryStagingCommits :many
SELECT * FROM repository_staging_commits
WHERE status IN ('prepared', 'committing', 'committed')
ORDER BY updated_at, commit_id
LIMIT ?1;
