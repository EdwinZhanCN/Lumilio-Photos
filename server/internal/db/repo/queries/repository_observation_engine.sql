-- name: EnsureRepositoryObservationState :one
INSERT INTO repository_observation_state (
    repository_id,
    adapter_kind,
    volume_identity,
    volume_kind,
    path_case_mode,
    path_normalization,
    cursor_health,
    full_verification_required,
    updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
ON CONFLICT (repository_id) DO UPDATE SET
    updated_at = repository_observation_state.updated_at
RETURNING *;

-- name: GetRepositoryObservationState :one
SELECT * FROM repository_observation_state
WHERE repository_id = ?1;

-- name: UpdateRepositoryObservationAdapter :one
UPDATE repository_observation_state
SET adapter_kind = ?2,
    adapter_identity = ?3,
    volume_identity = ?4,
    volume_kind = ?5,
    cursor_health = ?6,
    full_verification_required = CASE WHEN ?7 THEN 1 ELSE full_verification_required END,
    updated_at = ?8
WHERE repository_id = ?1
RETURNING *;

-- name: RequestRepositoryObservationEpoch :one
UPDATE repository_observation_state
SET desired_epoch = desired_epoch + 1,
    full_verification_required = CASE WHEN ?2 THEN 1 ELSE full_verification_required END,
    updated_at = ?3
WHERE repository_id = ?1
RETURNING *;

-- name: AllocateRepositoryObservationRevision :one
UPDATE repository_observation_state
SET next_revision = next_revision + 1,
    updated_at = ?2
WHERE repository_id = ?1
RETURNING next_revision - 1 AS revision;

-- name: AllocateRepositoryObservationRevisionRange :one
UPDATE repository_observation_state
SET next_revision = next_revision + ?2,
    updated_at = ?3
WHERE repository_id = ?1
  AND ?2 > 0
  AND ?2 <= 256
RETURNING next_revision - ?2 AS first_revision;

-- name: ClaimRepositoryObservationController :one
UPDATE repository_observation_state
SET controller_lease_id = ?2,
    controller_lease_expires_at = ?3,
    updated_at = ?4
WHERE repository_id = ?1
  AND (
      controller_lease_id IS NULL
      OR controller_lease_id = ?2
      OR controller_lease_expires_at < ?4
  )
RETURNING *;

-- name: ReleaseRepositoryObservationController :execrows
UPDATE repository_observation_state
SET controller_lease_id = NULL,
    controller_lease_expires_at = NULL,
    updated_at = ?3
WHERE repository_id = ?1
  AND controller_lease_id = ?2;

-- name: AdvanceRepositoryObservationEpoch :one
UPDATE repository_observation_state
SET applied_epoch = ?2,
    active_run_id = NULL,
    full_verification_required = CASE
        WHEN desired_epoch > ?2 THEN full_verification_required
        ELSE ?3
    END,
    updated_at = ?4
WHERE repository_id = ?1
  AND applied_epoch < ?2
  AND desired_epoch >= ?2
RETURNING *;

-- name: SetActiveRepositoryObservationRun :one
UPDATE repository_observation_state
SET active_run_id = ?2,
    updated_at = ?3
WHERE repository_id = ?1
  AND (active_run_id IS NULL OR active_run_id = ?2)
RETURNING *;

-- name: ClearActiveRepositoryObservationRunCAS :execrows
UPDATE repository_observation_state
SET active_run_id = NULL,
    updated_at = ?3
WHERE repository_id = ?1
  AND active_run_id = ?2;

-- name: CreateRepositoryScanRun :one
INSERT INTO repository_scan_runs (
    run_id,
    repository_id,
    requested_epoch,
    mode,
    requested_by,
    force_full_verification,
    status,
    created_at,
    updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, 'queued', ?7, ?7)
RETURNING *;

-- name: StartRepositoryScanRun :one
UPDATE repository_scan_runs
SET status = ?2,
    started_at = COALESCE(started_at, ?3),
    cursor_start = ?4,
    cursor_end = ?4,
    cursor_target = ?5,
    volume_identity = ?6,
    updated_at = ?3
WHERE run_id = ?1
  AND status = 'queued'
RETURNING *;

-- name: CaptureRepositoryScanRunCursorTarget :one
UPDATE repository_scan_runs
SET cursor_target = ?2,
    updated_at = ?3
WHERE run_id = ?1
  AND status = 'catching_up'
  AND length(cursor_target) = 0
RETURNING *;

-- name: UpdateRepositoryScanRunCursor :one
UPDATE repository_scan_runs
SET cursor_end = ?2,
    volume_identity = ?3,
    updated_at = ?4
WHERE run_id = ?1
  AND status = 'catching_up'
RETURNING *;

-- name: GetRepositoryScanRun :one
SELECT * FROM repository_scan_runs
WHERE repository_id = ?1 AND run_id = ?2;

-- name: GetLatestRepositoryScanRun :one
SELECT * FROM repository_scan_runs
WHERE repository_id = ?1
ORDER BY created_at DESC, run_id DESC
LIMIT 1;

-- name: ListRepositoryScanRuns :many
SELECT * FROM repository_scan_runs
WHERE repository_id = ?1
ORDER BY created_at DESC, run_id DESC
LIMIT ?2 OFFSET ?3;

-- name: GetActiveRepositoryScanRun :one
SELECT * FROM repository_scan_runs
WHERE repository_id = ?1
  AND status IN ('queued', 'crawling', 'catching_up', 'finalizing')
ORDER BY created_at, run_id
LIMIT 1;

-- name: CoalesceRepositoryScanRun :one
UPDATE repository_scan_runs
SET coalesced_count = coalesced_count + 1,
    requested_epoch = CASE WHEN status = 'queued' THEN ?2 ELSE requested_epoch END,
    force_full_verification = CASE
        WHEN status = 'queued' AND ?3 THEN 1
        ELSE force_full_verification
    END,
    updated_at = ?4
WHERE run_id = ?1
  AND status IN ('queued', 'crawling', 'catching_up', 'finalizing')
RETURNING *;

-- name: TransitionRepositoryScanRun :one
UPDATE repository_scan_runs
SET status = ?2,
    started_at = CASE WHEN started_at IS NULL AND ?2 = 'crawling' THEN ?3 ELSE started_at END,
    finished_at = CASE WHEN ?2 IN ('completed', 'partial', 'failed', 'cancelled') THEN ?3 ELSE NULL END,
    partial_coverage = ?4,
    failure_code = ?5,
    failure_problem_type = ?6,
    updated_at = ?3
WHERE run_id = ?1
  AND status = ?7
RETURNING *;

-- name: UpdateRepositoryScanRunProgress :one
UPDATE repository_scan_runs
SET directories_observed = directories_observed + ?2,
    files_observed = files_observed + ?3,
    bytes_queued = bytes_queued + ?4,
    bytes_hashed = bytes_hashed + ?5,
    authoritative_directories = authoritative_directories + ?6,
    error_directories = error_directories + ?7,
    outbox_depth = ?8,
    updated_at = ?9
WHERE run_id = ?1
RETURNING *;

-- name: RequestRepositoryScanRunCancellation :one
UPDATE repository_scan_runs
SET cancellation_requested = 1, updated_at = ?2
WHERE run_id = ?1
  AND status IN ('queued', 'crawling', 'catching_up', 'finalizing')
RETURNING *;

-- name: InsertRepositoryRootNode :one
INSERT INTO repository_nodes (
    node_id, repository_id, parent_node_id, name, name_key, kind,
    lifecycle, observation_revision, stability_token, created_at, updated_at
) VALUES (?1, ?2, NULL, '', '', 'directory', 'active', ?3, ?4, ?5, ?5)
RETURNING *;

-- name: GetRepositoryRootNode :one
SELECT * FROM repository_nodes
WHERE repository_id = ?1
  AND parent_node_id IS NULL
  AND lifecycle = 'active';

-- name: GetRepositoryNode :one
SELECT * FROM repository_nodes
WHERE repository_id = ?1 AND node_id = ?2;

-- name: GetPreferredActiveAssetOccurrence :one
SELECT *
FROM active_asset_occurrences
WHERE asset_id = ?1
ORDER BY repository_id, node_id
LIMIT 1;

-- name: ListAssetFullHashPrecheckMatches :many
WITH filter_params AS (
  SELECT CAST(sqlc.arg('full_hashes') AS TEXT) AS full_hashes_json
)
SELECT DISTINCT
    asset.asset_id,
    asset.original_filename,
    content.full_hash,
    content.file_size
FROM assets asset
JOIN content_objects content ON content.content_id = asset.content_id
JOIN active_asset_occurrences occurrence ON occurrence.asset_id = asset.asset_id
CROSS JOIN filter_params
WHERE occurrence.repository_id = sqlc.arg('repository_id')
  AND content.full_hash IN (
    SELECT value FROM json_each(filter_params.full_hashes_json)
  )
ORDER BY asset.asset_id;

-- name: ListAssetQuickFingerprintPrecheckMatches :many
WITH filter_params AS (
  SELECT CAST(sqlc.arg('quick_fingerprints') AS TEXT) AS quick_fingerprints_json
)
SELECT DISTINCT
    asset.asset_id,
    asset.original_filename,
    occurrence.quick_fingerprint,
    occurrence.file_size
FROM assets asset
JOIN active_asset_occurrences occurrence ON occurrence.asset_id = asset.asset_id
CROSS JOIN filter_params
WHERE occurrence.repository_id = sqlc.arg('repository_id')
  AND occurrence.quick_fingerprint IN (
    SELECT value FROM json_each(filter_params.quick_fingerprints_json)
  )
ORDER BY asset.asset_id;

-- name: GetActiveRepositoryChildByName :one
SELECT * FROM repository_nodes
WHERE repository_id = ?1
  AND parent_node_id = ?2
  AND name_key = ?3
  AND lifecycle = 'active';

-- name: ListActiveRepositoryNodesByNativeIdentity :many
SELECT * FROM repository_nodes
WHERE repository_id = ?1
  AND volume_identity = ?2
  AND native_identity_kind = ?3
  AND native_identity_value = ?4
  AND lifecycle = 'active'
ORDER BY node_id
LIMIT 3;

-- name: UpsertRepositoryNodeObservation :one
INSERT INTO repository_nodes (
    node_id, repository_id, parent_node_id, name, name_key, kind, lifecycle,
    native_identity_kind, native_identity_value, volume_identity,
    observation_revision, stability_token, file_size, modified_at_ns,
    changed_at_ns, last_seen_run_id, created_at, updated_at
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, 'active', ?7, ?8, ?9,
    ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?16
)
ON CONFLICT (node_id) DO UPDATE SET
    parent_node_id = excluded.parent_node_id,
    name = excluded.name,
    name_key = excluded.name_key,
    kind = excluded.kind,
    lifecycle = 'active',
    native_identity_kind = excluded.native_identity_kind,
    native_identity_value = excluded.native_identity_value,
    volume_identity = excluded.volume_identity,
    observation_revision = excluded.observation_revision,
    stability_token = excluded.stability_token,
    file_size = excluded.file_size,
    modified_at_ns = excluded.modified_at_ns,
    changed_at_ns = excluded.changed_at_ns,
    last_seen_run_id = excluded.last_seen_run_id,
    absence_first_observed_at = NULL,
    updated_at = excluded.updated_at
WHERE excluded.repository_id = repository_nodes.repository_id
  AND excluded.observation_revision > repository_nodes.observation_revision
RETURNING *;

-- name: RenameRepositoryNodeCAS :one
UPDATE repository_nodes
SET parent_node_id = ?3,
    name = ?4,
    name_key = ?5,
    observation_revision = ?6,
    updated_at = ?7
WHERE repository_id = ?1
  AND node_id = ?2
  AND lifecycle = 'active'
  AND observation_revision = ?8
  AND ?6 > ?8
RETURNING *;

-- name: TombstoneRepositoryNodeCAS :one
UPDATE repository_nodes
SET lifecycle = 'tombstoned',
    observation_revision = ?3,
    absence_first_observed_at = NULL,
    updated_at = ?4
WHERE repository_id = ?1
  AND node_id = ?2
  AND lifecycle = 'active'
  AND observation_revision < ?3
RETURNING *;

-- name: MarkRepositoryNodeAbsenceCandidateCAS :one
UPDATE repository_nodes
SET absence_first_observed_at = COALESCE(absence_first_observed_at, ?3),
    updated_at = ?4
WHERE repository_id = ?1
  AND node_id = ?2
  AND lifecycle = 'active'
RETURNING *;

-- name: UpdateRepositoryDirectoryCoverageCAS :one
UPDATE repository_nodes
SET last_authoritative_coverage_revision = ?3,
    updated_at = ?4
WHERE repository_id = ?1
  AND node_id = ?2
  AND lifecycle = 'active'
  AND kind = 'directory'
  AND last_authoritative_coverage_revision < ?3
RETURNING *;

-- name: ListRepositoryNodeChildrenPage :many
SELECT * FROM repository_nodes
WHERE repository_id = ?1
  AND parent_node_id = ?2
  AND lifecycle = 'active'
  AND node_id > ?3
ORDER BY node_id
LIMIT ?4;

-- name: ListUnseenRepositoryNodeChildrenPage :many
SELECT * FROM repository_nodes
WHERE repository_id = ?1
  AND parent_node_id = ?2
  AND lifecycle = 'active'
  AND (last_seen_run_id IS NULL OR last_seen_run_id != ?3)
  AND node_id > ?4
ORDER BY node_id
LIMIT ?5;

-- name: InsertRepositoryObservation :one
INSERT INTO repository_observations (
    observation_id, repository_id, revision, run_id, source,
    source_event_key, source_cursor, path_hint, parent_node_id, name, name_key,
    entry_kind, file_size, modified_at_ns, changed_at_ns,
    native_identity_kind, native_identity_value, stability_token_before,
    stability_token_after, quick_fingerprint, quick_fingerprint_version,
    resolved_owner_id, mapped_node_id, processing_state,
    authoritative_child_set, created_at
) VALUES (
    ?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11,
    ?12, ?13, ?14, ?15, ?16, ?17, ?18, ?19, ?20, ?21,
    ?22, ?23, 'pending', ?24, ?25
)
ON CONFLICT (repository_id, source, source_event_key)
    WHERE source_event_key IS NOT NULL
DO UPDATE SET created_at = repository_observations.created_at
RETURNING *;

-- name: GetRepositoryObservationBySourceEvent :one
SELECT * FROM repository_observations
WHERE repository_id = ?1
  AND source = ?2
  AND source_event_key = ?3;

-- name: GetRepositoryObservationForNodeRevision :one
SELECT * FROM repository_observations
WHERE repository_id = ?1
  AND mapped_node_id = ?2
  AND revision = ?3;

-- name: ListPendingRepositoryObservations :many
SELECT * FROM repository_observations
WHERE repository_id = ?1
  AND processing_state IN ('pending', 'retryable_error')
  AND revision > ?2
ORDER BY revision
LIMIT ?3;

-- name: CompleteRepositoryObservationCAS :one
UPDATE repository_observations
SET mapped_node_id = ?3,
    processing_state = ?4,
    failure_code = ?5,
    processed_at = ?6
WHERE repository_id = ?1
  AND observation_id = ?2
  AND processing_state IN ('pending', 'retryable_error')
RETURNING *;

-- name: EnqueueRepositoryScanFrontier :one
INSERT INTO repository_scan_frontier (
    run_id, directory_node_id, state, purpose, created_at, updated_at
) VALUES (?1, ?2, 'pending', ?3, ?4, ?4)
ON CONFLICT (run_id, directory_node_id) DO UPDATE SET
    updated_at = repository_scan_frontier.updated_at
RETURNING *;

-- name: RequeueRepositoryScanFrontierForVerification :one
INSERT INTO repository_scan_frontier (
    run_id, directory_node_id, state, purpose, created_at, updated_at
) VALUES (?1, ?2, 'pending', ?3, ?4, ?4)
ON CONFLICT (run_id, directory_node_id) DO UPDATE SET
    state = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN 'pending'
        ELSE repository_scan_frontier.state
    END,
    purpose = CASE
        WHEN excluded.purpose = 'crawl' THEN 'crawl'
        ELSE repository_scan_frontier.purpose
    END,
    lease_id = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN NULL
        ELSE repository_scan_frontier.lease_id
    END,
    lease_expires_at = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN NULL
        ELSE repository_scan_frontier.lease_expires_at
    END,
    continuation_offset = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN 0
        ELSE repository_scan_frontier.continuation_offset
    END,
    coverage_safe = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN 1
        ELSE repository_scan_frontier.coverage_safe
    END,
    authoritative_child_set = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN 0
        ELSE repository_scan_frontier.authoritative_child_set
    END,
    error_code = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN NULL
        ELSE repository_scan_frontier.error_code
    END,
    absence_cursor = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN ''
        ELSE repository_scan_frontier.absence_cursor
    END,
    absence_finalized = CASE
        WHEN repository_scan_frontier.state IN ('completed', 'error') THEN 0
        ELSE repository_scan_frontier.absence_finalized
    END,
    updated_at = excluded.updated_at
RETURNING *;

-- name: ClaimRepositoryScanFrontier :one
UPDATE repository_scan_frontier
SET state = 'leased',
    lease_id = ?2,
    lease_expires_at = ?3,
    attempt_count = attempt_count + 1,
    updated_at = ?4
WHERE rowid = (
      SELECT candidate.rowid
      FROM repository_scan_frontier AS candidate
      WHERE candidate.run_id = ?1
        AND (candidate.state = 'pending' OR (candidate.state = 'leased' AND candidate.lease_expires_at < ?4))
      ORDER BY candidate.directory_node_id
      LIMIT 1
  )
RETURNING *;

-- name: CompleteRepositoryScanFrontier :one
UPDATE repository_scan_frontier
SET state = CASE WHEN ?4 IS NULL THEN 'completed' ELSE 'error' END,
    lease_id = NULL,
    lease_expires_at = NULL,
    authoritative_child_set = CASE WHEN coverage_safe = 1 AND ?3 THEN 1 ELSE 0 END,
    error_code = ?4,
    updated_at = ?5
WHERE run_id = ?1
  AND directory_node_id = ?2
  AND state = 'leased'
  AND lease_id = ?6
RETURNING *;

-- name: ContinueRepositoryScanFrontier :one
UPDATE repository_scan_frontier
SET state = 'pending',
    lease_id = NULL,
    lease_expires_at = NULL,
    continuation_offset = ?3,
    coverage_safe = CASE WHEN ?4 THEN coverage_safe ELSE 0 END,
    updated_at = ?5
WHERE run_id = ?1
  AND directory_node_id = ?2
  AND state = 'leased'
  AND lease_id = ?6
  AND ?3 > continuation_offset
RETURNING *;

-- name: CountOpenRepositoryScanFrontier :one
SELECT count(*) FROM repository_scan_frontier
WHERE run_id = ?1 AND state IN ('pending', 'leased');

-- name: EnqueueRepositoryAbsenceCascadeFrontier :one
INSERT INTO repository_scan_frontier (
    run_id, directory_node_id, state, purpose, coverage_safe,
    authoritative_child_set, created_at, updated_at
) VALUES (?1, ?2, 'completed', 'absence', 1, 1, ?3, ?3)
ON CONFLICT (run_id, directory_node_id) DO UPDATE SET
    state = 'completed',
    purpose = 'absence',
    lease_id = NULL,
    lease_expires_at = NULL,
    continuation_offset = 0,
    coverage_safe = 1,
    authoritative_child_set = 1,
    absence_cursor = '',
    absence_finalized = 0,
    error_code = NULL,
    updated_at = excluded.updated_at
RETURNING *;

-- name: ClaimRepositoryAbsenceFrontier :one
UPDATE repository_scan_frontier
SET state = 'absence',
    lease_id = ?2,
    lease_expires_at = ?3,
    attempt_count = attempt_count + 1,
    updated_at = ?4
WHERE rowid = (
    SELECT candidate.rowid
    FROM repository_scan_frontier candidate
    WHERE candidate.run_id = ?1
      AND candidate.authoritative_child_set = 1
      AND candidate.absence_finalized = 0
      AND (
        candidate.state = 'completed'
        OR (candidate.state = 'absence' AND candidate.lease_expires_at < ?4)
      )
    ORDER BY candidate.directory_node_id
    LIMIT 1
)
RETURNING *;

-- name: ContinueRepositoryAbsenceFrontier :one
UPDATE repository_scan_frontier
SET state = 'completed',
    lease_id = NULL,
    lease_expires_at = NULL,
    absence_cursor = ?3,
    updated_at = ?4
WHERE run_id = ?1
  AND directory_node_id = ?2
  AND state = 'absence'
  AND lease_id = ?5
  AND ?3 > absence_cursor
RETURNING *;

-- name: CompleteRepositoryAbsenceFrontier :one
UPDATE repository_scan_frontier
SET state = 'completed',
    lease_id = NULL,
    lease_expires_at = NULL,
    absence_finalized = 1,
    updated_at = ?3
WHERE run_id = ?1
  AND directory_node_id = ?2
  AND state = 'absence'
  AND lease_id = ?4
RETURNING *;

-- name: CountOpenRepositoryAbsenceFrontier :one
SELECT count(*) FROM repository_scan_frontier
WHERE run_id = ?1
  AND authoritative_child_set = 1
  AND absence_finalized = 0;

-- name: UpsertRepositoryChangeCursor :one
INSERT INTO repository_change_cursors (
    repository_id, adapter_kind, cursor, volume_identity, journal_identity,
    status, applied_revision, updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)
ON CONFLICT (repository_id, adapter_kind) DO UPDATE SET
    cursor = excluded.cursor,
    volume_identity = excluded.volume_identity,
    journal_identity = excluded.journal_identity,
    status = excluded.status,
    applied_revision = excluded.applied_revision,
    updated_at = excluded.updated_at
WHERE excluded.applied_revision >= repository_change_cursors.applied_revision
RETURNING *;

-- name: GetRepositoryChangeCursor :one
SELECT * FROM repository_change_cursors
WHERE repository_id = ?1 AND adapter_kind = ?2;

-- name: InsertContentObject :one
INSERT INTO content_objects (
    content_id, hash_algorithm, full_hash, file_size, created_at
) VALUES (?1, ?2, ?3, ?4, ?5)
ON CONFLICT (hash_algorithm, full_hash, file_size) DO UPDATE SET
    created_at = content_objects.created_at
RETURNING *;

-- name: GetContentObject :one
SELECT * FROM content_objects
WHERE hash_algorithm = ?1 AND full_hash = ?2 AND file_size = ?3;

-- name: GetContentObjectByID :one
SELECT * FROM content_objects WHERE content_id = ?1;

-- name: InsertOwnerContentAsset :one
INSERT INTO assets (
    asset_id, owner_id, content_id, type, original_filename, mime_type,
    upload_time, taken_time, rating, status, updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11)
ON CONFLICT (owner_id, content_id) DO UPDATE SET
    updated_at = assets.updated_at
RETURNING *;

-- name: GetOwnerContentAsset :one
SELECT * FROM assets
WHERE owner_id = ?1 AND content_id = ?2;

-- name: CloseActiveAssetLocationCAS :execrows
UPDATE asset_locations
SET unbound_observation_revision = ?2,
    updated_at = ?3
WHERE node_id = ?1
  AND unbound_observation_revision IS NULL
  AND bound_observation_revision < ?2;

-- name: BindAssetLocation :one
INSERT INTO asset_locations (
    location_id, node_id, asset_id, bound_observation_revision,
    unbound_observation_revision, created_at, updated_at
) VALUES (?1, ?2, ?3, ?4, NULL, ?5, ?5)
RETURNING *;

-- name: GetActiveAssetLocationByNode :one
SELECT * FROM asset_locations
WHERE node_id = ?1 AND unbound_observation_revision IS NULL;

-- name: ListActiveAssetLocations :many
SELECT * FROM asset_locations
WHERE asset_id = ?1 AND unbound_observation_revision IS NULL
ORDER BY node_id;

-- name: InsertRepositoryOutboxEffect :one
INSERT INTO repository_outbox (
    outbox_id, repository_id, effect_key, effect_kind, entity_id,
    expected_revision, payload, status, created_at, updated_at
) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, 'pending', ?8, ?8)
ON CONFLICT (effect_key) DO UPDATE SET
    updated_at = repository_outbox.updated_at
RETURNING *;

-- name: ClaimRepositoryOutboxBatch :many
UPDATE repository_outbox
SET status = 'delivering',
    lease_id = ?1,
    lease_expires_at = ?2,
    attempt_count = attempt_count + 1,
    updated_at = ?3
WHERE outbox_id IN (
    SELECT candidate.outbox_id
    FROM repository_outbox AS candidate
    WHERE candidate.effect_kind = ?4
      AND (candidate.status = 'pending'
       OR (candidate.status = 'delivering' AND candidate.lease_expires_at < ?3))
    ORDER BY candidate.created_at, candidate.outbox_id
    LIMIT ?5
)
RETURNING *;

-- name: CompleteRepositoryOutboxEffect :execrows
UPDATE repository_outbox
SET status = ?3,
    lease_id = NULL,
    lease_expires_at = NULL,
    last_failure_code = ?4,
    delivered_at = CASE WHEN ?3 = 'delivered' THEN ?5 ELSE delivered_at END,
    updated_at = ?5
WHERE outbox_id = ?1
  AND lease_id = ?2
  AND status = 'delivering';

-- name: CountPendingRepositoryOutbox :one
SELECT count(*) FROM repository_outbox
WHERE repository_id = ?1 AND status IN ('pending', 'delivering');
