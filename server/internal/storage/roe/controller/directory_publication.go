package controller

import (
	"context"
	"database/sql"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/pathsemantics"
)

// directoryBatchPublication is the bounded result of one staged directory
// page. The page is assembled outside the catalog transaction; publication
// uses a temporary table and set-based statements so the writer does not
// execute one SQL statement per observed entry.
type directoryBatchPublication struct {
	rowsApplied    int
	bytesQueued    int64
	directoryCount int64
	fileCount      int64
}

const createDirectoryPublicationStage = `
CREATE TEMP TABLE IF NOT EXISTS roe_directory_publication_stage (
    ordinal INTEGER PRIMARY KEY,
    next_offset INTEGER NOT NULL,
    observation_id TEXT NOT NULL,
    candidate_node_id TEXT NOT NULL,
    resolved_node_id TEXT,
    resolved_owner_id INTEGER,
    path_hint TEXT NOT NULL,
    name TEXT NOT NULL,
    name_key TEXT NOT NULL,
    entry_kind TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    modified_at_ns INTEGER,
    changed_at_ns INTEGER,
    native_identity_kind TEXT,
    native_identity_value TEXT,
    volume_identity TEXT,
    observation_token TEXT NOT NULL,
    source_event_key TEXT NOT NULL,
    processing_state TEXT NOT NULL,
    failure_code TEXT,
    revision INTEGER,
    already_present INTEGER NOT NULL DEFAULT 0,
    was_existing INTEGER NOT NULL DEFAULT 0,
    before_token TEXT,
    should_hash INTEGER NOT NULL DEFAULT 0
);`

// publishDirectoryBatchSetBased stages all observations in one bounded page,
// resolves node identity in SQL, and then applies nodes, observations,
// frontiers in set-based statements. The only Go
// loop is the in-memory staging conversion; it does not issue catalog SQL per
// observation.
func (applier *commitApplier) publishDirectoryBatchSetBased(
	ctx context.Context,
	tx *sql.Tx,
	queries *repo.Queries,
	run repo.RepositoryScanRun,
	parentNodeID uuid.UUID,
	frontier repo.RepositoryScanFrontier,
	batch storage.DirectoryReadBatch,
	semantics pathsemantics.Semantics,
	ownerID *int64,
	now dbtypes.Timestamp,
) (directoryBatchPublication, error) {
	var publication directoryBatchPublication
	if _, err := tx.ExecContext(ctx, createDirectoryPublicationStage); err != nil {
		return publication, fmt.Errorf("create repository publication stage: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM temp.roe_directory_publication_stage`); err != nil {
		return publication, fmt.Errorf("clear repository publication stage: %w", err)
	}
	if len(batch.Entries) == 0 {
		return publication, nil
	}

	firstRevision, err := queries.AllocateRepositoryObservationRevisionRange(ctx, repo.AllocateRepositoryObservationRevisionRangeParams{
		RepositoryID: run.RepositoryID,
		NextRevision: int64(len(batch.Entries)),
		UpdatedAt:    now,
	})
	if err != nil {
		return publication, fmt.Errorf("allocate observation revision range: %w", err)
	}

	const stageColumnCount = 18
	placeholders := make([]string, 0, len(batch.Entries))
	args := make([]any, 0, len(batch.Entries)*stageColumnCount)
	for ordinal, directoryEntry := range batch.Entries {
		observation := directoryEntry.Observation
		name := path.Base(observation.Path.String())
		nameKey, nameErr := semantics.NameKey(name)
		if nameErr != nil {
			return publication, nameErr
		}
		nodeID := uuid.New()
		observationID := uuid.New()
		sourceEventKey := fmt.Sprintf("crawl:%s:%s:%d", run.RunID, parentNodeID, directoryEntry.NextOffset)
		placeholders = append(placeholders, "("+strings.TrimRight(strings.Repeat("?,", stageColumnCount), ",")+")")
		args = append(args,
			ordinal,
			directoryEntry.NextOffset,
			observationID.String(),
			nodeID.String(),
			observation.Path.String(),
			name,
			nameKey,
			observationNodeKind(observation.EntryKind),
			observation.Size,
			optionalInt64Value(observation.ModTimeNS),
			optionalInt64PointerValue(observation.ChangeTimeNS),
			optionalStringPointerValue(observation.FileIdentityKind),
			optionalStringPointerValue(observation.FileIdentity),
			optionalVolumeIdentityValue(observation),
			observation.ObservationToken,
			sourceEventKey,
			"applied",
			nil,
		)
	}
	insertStageSQL := `INSERT INTO temp.roe_directory_publication_stage (
    ordinal, next_offset, observation_id, candidate_node_id, path_hint, name,
    name_key, entry_kind, file_size, modified_at_ns, changed_at_ns,
    native_identity_kind, native_identity_value, volume_identity,
    observation_token, source_event_key, processing_state,
    failure_code
) VALUES ` + strings.Join(placeholders, ",")
	if _, err := tx.ExecContext(ctx, insertStageSQL, args...); err != nil {
		return publication, fmt.Errorf("stage repository directory page: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage
SET revision = ? + ordinal
`, firstRevision); err != nil {
		return publication, fmt.Errorf("assign repository observation revisions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET already_present = CASE WHEN EXISTS (
    SELECT 1
    FROM repository_observations observation
    WHERE observation.repository_id = ?
      AND observation.source = 'crawl'
      AND observation.source_event_key = stage.source_event_key
) THEN 1 ELSE 0 END
`, run.RepositoryID); err != nil {
		return publication, fmt.Errorf("mark existing repository observations: %w", err)
	}

	// Name identity wins. If the name is new, a unique native identity is the
	// rename/move fallback; ambiguous identity intentionally receives a new
	// node, matching the pre-staging resolver.
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET resolved_node_id = COALESCE(
    (
        SELECT node.node_id
        FROM repository_nodes node
        WHERE node.repository_id = ?
          AND node.parent_node_id = ?
          AND node.name_key = stage.name_key
          AND node.lifecycle = 'active'
    ),
    (
        SELECT CASE WHEN count(*) = 1 THEN min(node.node_id) END
        FROM repository_nodes node
        WHERE node.repository_id = ?
          AND node.lifecycle = 'active'
          AND stage.volume_identity IS NOT NULL
          AND stage.native_identity_kind IS NOT NULL
          AND stage.native_identity_value IS NOT NULL
          AND node.volume_identity = stage.volume_identity
          AND node.native_identity_kind = stage.native_identity_kind
          AND node.native_identity_value = stage.native_identity_value
    ),
    stage.candidate_node_id
)
WHERE stage.already_present = 0
`, run.RepositoryID, parentNodeID, run.RepositoryID); err != nil {
		return publication, fmt.Errorf("resolve staged repository nodes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET was_existing = CASE WHEN EXISTS (
    SELECT 1 FROM repository_nodes node
    WHERE node.repository_id = ? AND node.node_id = stage.resolved_node_id
) THEN 1 ELSE 0 END,
before_token = (
    SELECT node.stability_token
    FROM repository_nodes node
    WHERE node.repository_id = ? AND node.node_id = stage.resolved_node_id
)
WHERE stage.already_present = 0
`, run.RepositoryID, run.RepositoryID); err != nil {
		return publication, fmt.Errorf("capture staged repository node state: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO repository_nodes (
    node_id, repository_id, parent_node_id, name, name_key, kind, lifecycle,
    native_identity_kind, native_identity_value, volume_identity,
    observation_revision, stability_token, file_size, modified_at_ns,
    changed_at_ns, last_seen_run_id, created_at, updated_at
)
SELECT
    stage.resolved_node_id, ?, ?, stage.name, stage.name_key, stage.entry_kind,
    'active', stage.native_identity_kind, stage.native_identity_value,
    stage.volume_identity, stage.revision, stage.observation_token,
    stage.file_size, stage.modified_at_ns, stage.changed_at_ns, ?, ?, ?
FROM temp.roe_directory_publication_stage stage
WHERE stage.already_present = 0
ON CONFLICT(node_id) DO UPDATE SET
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
`, run.RepositoryID, parentNodeID, run.RunID, now, now); err != nil {
		return publication, fmt.Errorf("publish staged repository nodes: %w", err)
	}

	// A repository node owns its attribution across re-observations. An upload
	// can establish that attribution before an in-flight crawl reaches the same
	// node; letting the crawl's repository default replace it would split one
	// physical file into owner-specific Assets and strand the upload Asset.
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET resolved_owner_id = CASE WHEN stage.entry_kind = 'file' THEN COALESCE(
    (
        SELECT asset.owner_id
        FROM asset_locations location
        JOIN assets asset ON asset.asset_id = location.asset_id
        WHERE location.node_id = stage.resolved_node_id
          AND location.unbound_observation_revision IS NULL
    ),
    ?
) END
WHERE stage.already_present = 0
`, optionalInt64PointerValue(ownerID)); err != nil {
		return publication, fmt.Errorf("resolve staged repository owners: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET processing_state = CASE WHEN
        stage.entry_kind = 'file' AND stage.resolved_owner_id IS NULL
    THEN 'terminal_unsupported' ELSE 'applied' END,
    failure_code = CASE WHEN
        stage.entry_kind = 'file' AND stage.resolved_owner_id IS NULL
    THEN 'default_owner_required' ELSE NULL END
WHERE stage.already_present = 0
`); err != nil {
		return publication, fmt.Errorf("classify staged repository ownership: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE temp.roe_directory_publication_stage AS stage
SET should_hash = CASE WHEN
    stage.entry_kind = 'file'
    AND stage.resolved_owner_id IS NOT NULL
    AND (
        stage.before_token IS NULL
        OR stage.before_token <> stage.observation_token
        OR NOT EXISTS (
            SELECT 1 FROM asset_locations location
            WHERE location.node_id = stage.resolved_node_id
              AND location.unbound_observation_revision IS NULL
        )
    )
THEN 1 ELSE 0 END
WHERE stage.already_present = 0
	`); err != nil {
		return publication, fmt.Errorf("classify staged repository hash work: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO repository_observations (
    observation_id, repository_id, revision, run_id, source,
    source_event_key, path_hint, parent_node_id, name, name_key, entry_kind,
    file_size, modified_at_ns, changed_at_ns, native_identity_kind,
    native_identity_value, stability_token_before, stability_token_after,
    resolved_owner_id, mapped_node_id, processing_state, created_at
)
SELECT
    stage.observation_id, ?, stage.revision, ?, 'crawl',
    stage.source_event_key, stage.path_hint, ?, stage.name, stage.name_key,
    stage.entry_kind, stage.file_size, stage.modified_at_ns,
    stage.changed_at_ns, stage.native_identity_kind,
    stage.native_identity_value, stage.before_token, stage.observation_token,
    stage.resolved_owner_id, stage.resolved_node_id, 'pending', ?
FROM temp.roe_directory_publication_stage stage
WHERE stage.already_present = 0
ON CONFLICT (repository_id, source, source_event_key)
    WHERE source_event_key IS NOT NULL
DO UPDATE SET created_at = repository_observations.created_at
	`, run.RepositoryID, run.RunID, parentNodeID, now); err != nil {
		return publication, fmt.Errorf("publish staged repository observations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE repository_observations AS observation
SET mapped_node_id = (
        SELECT stage.resolved_node_id
        FROM temp.roe_directory_publication_stage stage
        WHERE stage.source_event_key = observation.source_event_key
    ),
    processing_state = (
        SELECT stage.processing_state
        FROM temp.roe_directory_publication_stage stage
        WHERE stage.source_event_key = observation.source_event_key
    ),
    failure_code = (
        SELECT stage.failure_code
        FROM temp.roe_directory_publication_stage stage
        WHERE stage.source_event_key = observation.source_event_key
    ),
    processed_at = ?
WHERE observation.repository_id = ?
  AND observation.source = 'crawl'
  AND observation.processing_state IN ('pending', 'retryable_error')
  AND EXISTS (
      SELECT 1
      FROM temp.roe_directory_publication_stage stage
      WHERE stage.already_present = 0
        AND stage.source_event_key = observation.source_event_key
  )
`, now.Time.UnixMicro(), run.RepositoryID); err != nil {
		return publication, fmt.Errorf("complete staged repository observations: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO repository_scan_frontier (
    run_id, directory_node_id, state, purpose, created_at, updated_at
)
SELECT ?, stage.resolved_node_id, 'pending', 'crawl', ?, ?
FROM temp.roe_directory_publication_stage stage
WHERE stage.already_present = 0
  AND stage.entry_kind = 'directory'
  AND (? = 'crawl' OR stage.was_existing = 0)
ON CONFLICT (run_id, directory_node_id) DO UPDATE SET
    updated_at = repository_scan_frontier.updated_at
`, run.RunID, now, now, frontier.Purpose); err != nil {
		return publication, fmt.Errorf("enqueue staged repository directories: %w", err)
	}
	var rows, directories, files int64
	if err := tx.QueryRowContext(ctx, `
SELECT
    COALESCE(SUM(CASE WHEN already_present = 0 THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN already_present = 0 AND entry_kind = 'directory' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN already_present = 0 AND entry_kind = 'file' THEN 1 ELSE 0 END), 0),
    COALESCE(SUM(CASE WHEN already_present = 0 AND should_hash = 1 THEN file_size ELSE 0 END), 0)
FROM temp.roe_directory_publication_stage
`).Scan(&rows, &directories, &files, &publication.bytesQueued); err != nil {
		return publication, fmt.Errorf("measure staged repository publication: %w", err)
	}
	publication.rowsApplied = int(rows)
	publication.directoryCount = directories
	publication.fileCount = files
	return publication, nil
}

func optionalInt64Value(value int64) any {
	return value
}

func optionalInt64PointerValue(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalStringPointerValue(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func optionalVolumeIdentityValue(observation storage.FileObservation) any {
	return optionalStringPointerValue(observationVolumeIdentity(observation))
}
