package domainoutbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/pipeline"
)

type Reconciler struct {
	writer   *catalogtx.Writer
	interval time.Duration
}

func NewReconciler(writer *catalogtx.Writer, interval time.Duration) *Reconciler {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Reconciler{writer: writer, interval: interval}
}
func (r *Reconciler) Run(ctx context.Context) error {
	if r == nil || r.writer == nil {
		return errors.New("domain outbox reconciler requires a catalog writer")
	}
	if ctx == nil {
		return errors.New("domain outbox reconciler context is nil")
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		if _, err := r.ReconcileOnce(ctx); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// Catalog truth remains durable when one reconciliation pass fails.
			// Keep the periodic loop alive so a transient lock, connection, or
			// malformed-but-repairable delivery condition cannot disable recovery
			// until the next process restart.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// ReconcileOnce re-arms all asset desired/applied gaps using catalog facts
// alone. It never queries River rows or job state.
func (r *Reconciler) ReconcileOnce(ctx context.Context) (int, error) {
	if r == nil || r.writer == nil {
		return 0, errors.New("domain outbox reconciler requires a catalog writer")
	}
	if ctx == nil {
		return 0, errors.New("domain outbox reconciliation context is nil")
	}
	inserted := 0
	now := time.Now().UTC()
	err := r.writer.Transact(ctx, catalogtx.OperationDomainOutboxReconcile, nil, func(tx *sql.Tx) error {
		ingestRows, err := tx.QueryContext(ctx, `SELECT receipt_id,subject_id FROM catalog_operation_receipts WHERE kind='ingest' AND state='pending' ORDER BY updated_at,receipt_id`)
		if err != nil {
			return err
		}
		type ingestPending struct{ receiptID, commitID string }
		var ingests []ingestPending
		for ingestRows.Next() {
			var item ingestPending
			if err := ingestRows.Scan(&item.receiptID, &item.commitID); err != nil {
				_ = ingestRows.Close()
				return err
			}
			ingests = append(ingests, item)
		}
		if err := ingestRows.Err(); err != nil {
			_ = ingestRows.Close()
			return err
		}
		if err := ingestRows.Close(); err != nil {
			return err
		}
		for _, item := range ingests {
			receiptID, err := uuid.Parse(item.receiptID)
			if err != nil {
				return err
			}
			commitID, err := uuid.Parse(item.commitID)
			if err != nil {
				return err
			}
			envelope, err := pipeline.NewEnvelope("ingest_asset", uuid.New(), receiptID, pipeline.IngestCommand{CommitID: commitID, ReceiptID: receiptID, Admission: pipeline.AdmissionInteractive}, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, item.commitID, 1, envelope); err != nil {
				return err
			}
			inserted++
		}

		rows, err := tx.QueryContext(ctx, `SELECT asset_id, source_content_id, stage, pipeline_version, desired_version FROM asset_pipeline_state WHERE desired_version > applied_version AND terminal_error IS NULL ORDER BY updated_at, asset_id, stage`)
		if err != nil {
			return err
		}
		type pending struct {
			assetID, fence, stage, version string
			desired                        uint64
		}
		var work []pending
		for rows.Next() {
			var item pending
			if err := rows.Scan(&item.assetID, &item.fence, &item.stage, &item.version, &item.desired); err != nil {
				_ = rows.Close()
				return err
			}
			work = append(work, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range work {
			assetID, err := uuid.Parse(item.assetID)
			if err != nil {
				return err
			}
			fenceID, err := uuid.Parse(item.fence)
			if err != nil {
				return err
			}
			fence, err := pipeline.NewSourceFence(fenceID)
			if err != nil {
				return err
			}
			command := pipeline.AssetCommand{AssetID: assetID, Fence: fence, Stage: pipeline.Stage(item.stage), DesiredVersion: item.desired, PipelineVersion: item.version, Admission: pipeline.AdmissionBackground}
			ready, err := pipeline.AssetStageReadyTx(ctx, tx, command)
			if err != nil {
				return err
			}
			if !ready {
				continue
			}
			envelope, err := pipeline.NewEnvelope("asset."+item.stage, uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, item.assetID, item.desired, envelope); err != nil {
				return fmt.Errorf("rearm asset work: %w", err)
			}
			inserted++
		}

		repositoryRows, err := tx.QueryContext(ctx, `
			SELECT state.repository_id,active.requested_epoch
			FROM repository_observation_state AS state
			JOIN repository_scan_runs AS active
			  ON active.run_id=state.active_run_id
			 AND active.repository_id=state.repository_id
			 AND active.status IN ('queued','crawling','catching_up','finalizing')
			WHERE (state.desired_epoch>state.applied_epoch OR state.full_verification_required=1)
			  AND state.terminal_error IS NULL
			ORDER BY state.updated_at,state.repository_id`)
		if err != nil {
			return err
		}
		type repositoryPending struct {
			id    string
			epoch uint64
		}
		var repositories []repositoryPending
		for repositoryRows.Next() {
			var item repositoryPending
			if err := repositoryRows.Scan(&item.id, &item.epoch); err != nil {
				_ = repositoryRows.Close()
				return err
			}
			repositories = append(repositories, item)
		}
		if err := repositoryRows.Err(); err != nil {
			_ = repositoryRows.Close()
			return err
		}
		if err := repositoryRows.Close(); err != nil {
			return err
		}
		for _, item := range repositories {
			id, err := uuid.Parse(item.id)
			if err != nil {
				return err
			}
			command := pipeline.RepositoryCommand{RepositoryID: id, RequestedEpoch: item.epoch, DesiredVersion: item.epoch, Admission: pipeline.AdmissionBackground}
			envelope, err := pipeline.NewEnvelope("repository.scan", uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, item.id, item.epoch, envelope); err != nil {
				return err
			}
			inserted++
		}

		eventRows, err := tx.QueryContext(ctx, `SELECT owner_id,source_revision,projection_version,cursor FROM event_projection_pipeline_state WHERE source_revision>applied_revision AND terminal_error IS NULL ORDER BY updated_at,owner_id`)
		if err != nil {
			return err
		}
		type eventPending struct {
			owner           int32
			source, version uint64
			cursor          sql.NullString
		}
		var events []eventPending
		for eventRows.Next() {
			var item eventPending
			if err := eventRows.Scan(&item.owner, &item.source, &item.version, &item.cursor); err != nil {
				_ = eventRows.Close()
				return err
			}
			events = append(events, item)
		}
		if err := eventRows.Err(); err != nil {
			_ = eventRows.Close()
			return err
		}
		if err := eventRows.Close(); err != nil {
			return err
		}
		for _, item := range events {
			command := pipeline.ProjectionCommand{Kind: "event", Scope: fmt.Sprint(item.owner), SourceRevision: item.source, ProjectionVersion: item.version, Cursor: item.cursor.String, Admission: pipeline.AdmissionBackground}
			envelope, err := pipeline.NewEnvelope("projection.event", uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, command.Scope, item.version, envelope); err != nil {
				return err
			}
			inserted++
		}

		locationRows, err := tx.QueryContext(ctx, `SELECT repository_id,owner_id,source_revision FROM location_projection_state WHERE source_revision>published_revision AND terminal_error IS NULL ORDER BY updated_at,repository_id,owner_id`)
		if err != nil {
			return err
		}
		type locationPending struct {
			repository string
			owner      int32
			revision   uint64
		}
		var locations []locationPending
		for locationRows.Next() {
			var item locationPending
			if err := locationRows.Scan(&item.repository, &item.owner, &item.revision); err != nil {
				_ = locationRows.Close()
				return err
			}
			locations = append(locations, item)
		}
		if err := locationRows.Err(); err != nil {
			_ = locationRows.Close()
			return err
		}
		if err := locationRows.Close(); err != nil {
			return err
		}
		for _, item := range locations {
			scope := item.repository + ":" + fmt.Sprint(item.owner)
			command := pipeline.ProjectionCommand{Kind: "location", Scope: scope, SourceRevision: item.revision, ProjectionVersion: item.revision, Admission: pipeline.AdmissionBackground}
			envelope, err := pipeline.NewEnvelope("projection.location", uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, scope, item.revision, envelope); err != nil {
				return err
			}
			inserted++
		}

		var resolutionSource, resolutionVersion uint64
		if err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version FROM location_resolution_pipeline_state WHERE scope='all' AND projection_version>applied_revision AND terminal_error IS NULL`).Scan(&resolutionSource, &resolutionVersion); err == nil {
			command := pipeline.ProjectionCommand{Kind: "location_resolution", Scope: "all", SourceRevision: resolutionSource, ProjectionVersion: resolutionVersion, Admission: pipeline.AdmissionBackground}
			envelope, err := pipeline.NewEnvelope("projection.location", uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, "all", resolutionVersion, envelope); err != nil {
				return err
			}
			inserted++
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		var ocrRevision uint64
		// The OCR outbox revision is the domain fence. Wall-clock updated_at is
		// only delivery ordering metadata and must not become projection truth.
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(revision),0) FROM ocr_index_outbox`).Scan(&ocrRevision); err != nil {
			return err
		}
		if ocrRevision > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO ocr_projection_pipeline_state(scope,source_revision,projection_version,applied_revision,updated_at) VALUES('all',?,?,0,?) ON CONFLICT(scope) DO UPDATE SET source_revision=MAX(ocr_projection_pipeline_state.source_revision,excluded.source_revision),projection_version=MAX(ocr_projection_pipeline_state.projection_version,excluded.projection_version),updated_at=excluded.updated_at`, ocrRevision, ocrRevision, now.UnixMicro()); err != nil {
				return err
			}
		}
		var ocrSource, ocrVersion uint64
		if err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version FROM ocr_projection_pipeline_state WHERE scope='all' AND projection_version>applied_revision AND terminal_error IS NULL`).Scan(&ocrSource, &ocrVersion); err == nil {
			command := pipeline.ProjectionCommand{Kind: "ocr", Scope: "all", SourceRevision: ocrSource, ProjectionVersion: ocrVersion, Admission: pipeline.AdmissionBackground}
			envelope, err := pipeline.NewEnvelope("projection.ocr", uuid.New(), uuid.New(), command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, "all", ocrVersion, envelope); err != nil {
				return err
			}
			inserted++
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		reindexRows, err := tx.QueryContext(ctx, `SELECT r.receipt_id,q.repository_id,q.tasks,q.page_limit,q.cursor,q.missing_only,q.reset_semantic,q.requested_revision FROM catalog_operation_receipts r JOIN asset_reindex_requests q USING(receipt_id) WHERE r.state='pending' AND q.requested_revision>q.applied_revision ORDER BY r.updated_at,r.receipt_id`)
		if err != nil {
			return err
		}
		type reindexPending struct {
			receipt                    string
			repository, cursor         sql.NullString
			tasksJSON                  string
			limit                      int
			missingOnly, resetSemantic bool
			revision                   uint64
		}
		var reindexes []reindexPending
		for reindexRows.Next() {
			var item reindexPending
			if err := reindexRows.Scan(&item.receipt, &item.repository, &item.tasksJSON, &item.limit, &item.cursor, &item.missingOnly, &item.resetSemantic, &item.revision); err != nil {
				_ = reindexRows.Close()
				return err
			}
			reindexes = append(reindexes, item)
		}
		if err := reindexRows.Err(); err != nil {
			_ = reindexRows.Close()
			return err
		}
		if err := reindexRows.Close(); err != nil {
			return err
		}
		for _, item := range reindexes {
			receiptID, err := uuid.Parse(item.receipt)
			if err != nil {
				return err
			}
			var repositoryID *uuid.UUID
			if item.repository.Valid {
				parsed, err := uuid.Parse(item.repository.String)
				if err != nil {
					return err
				}
				repositoryID = &parsed
			}
			var tasks []string
			if err := json.Unmarshal([]byte(item.tasksJSON), &tasks); err != nil {
				return err
			}
			command := pipeline.ReindexCommand{ReceiptID: receiptID, RepositoryID: repositoryID, Tasks: tasks, Limit: item.limit, Cursor: item.cursor.String, MissingOnly: item.missingOnly, ResetSemantic: item.resetSemantic, RequestedRevision: item.revision, Admission: pipeline.AdmissionMaintenance}
			envelope, err := pipeline.NewEnvelope("projection.asset_reindex", uuid.New(), receiptID, command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, item.receipt, item.revision, envelope); err != nil {
				return err
			}
			inserted++
		}

		backupRows, err := tx.QueryContext(ctx, `SELECT r.receipt_id,b.force FROM catalog_operation_receipts r JOIN catalog_backup_requests b USING(receipt_id) WHERE r.kind='backup' AND r.state='pending' ORDER BY r.updated_at,r.receipt_id`)
		if err != nil {
			return err
		}
		type backupPending struct {
			receipt string
			force   bool
		}
		var backups []backupPending
		for backupRows.Next() {
			var item backupPending
			if err := backupRows.Scan(&item.receipt, &item.force); err != nil {
				_ = backupRows.Close()
				return err
			}
			backups = append(backups, item)
		}
		if err := backupRows.Err(); err != nil {
			_ = backupRows.Close()
			return err
		}
		if err := backupRows.Close(); err != nil {
			return err
		}
		for _, item := range backups {
			receiptID, err := uuid.Parse(item.receipt)
			if err != nil {
				return err
			}
			command := pipeline.BackupCommand{RequestID: receiptID, Force: item.force, Admission: pipeline.AdmissionMaintenance}
			envelope, err := pipeline.NewEnvelope("backup_catalog", uuid.New(), receiptID, command, now)
			if err != nil {
				return err
			}
			if err := pipeline.PublishEnvelopeTx(ctx, tx, item.receipt, 1, envelope); err != nil {
				return err
			}
			inserted++
		}
		return nil
	})
	return inserted, err
}
