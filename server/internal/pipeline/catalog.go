package pipeline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"server/internal/workqos"
)

func RequestIngestTx(ctx context.Context, tx *sql.Tx, commitID, receiptID uuid.UUID) error {
	if tx == nil || commitID == uuid.Nil || receiptID == uuid.Nil {
		return errors.New("ingest request requires commit and receipt IDs")
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts (receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES (?,?,?,1,'pending',?,?)`, receiptID.String(), "ingest", commitID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return fmt.Errorf("create ingest receipt: %w", err)
	}
	return nil
}

func RequestAssetStagesTx(ctx context.Context, tx *sql.Tx, assetID, contentID uuid.UUID, stages []Stage, pipelineVersion string, qos workqos.Class, correlationID uuid.UUID) error {
	priority, err := qos.Priority()
	if tx == nil || assetID == uuid.Nil || contentID == uuid.Nil || strings.TrimSpace(pipelineVersion) == "" || len(stages) == 0 || err != nil {
		return errors.New("invalid asset pipeline request")
	}
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	now := time.Now().UTC()
	seenStages := make(map[Stage]struct{}, len(stages))
	for _, stage := range stages {
		if stage != StageAnalyze && stage != StageDerivatives && stage != StageTranscode && stage != StageEnrich {
			return fmt.Errorf("invalid asset stage %q", stage)
		}
		if _, seen := seenStages[stage]; seen {
			continue
		}
		seenStages[stage] = struct{}{}
		var desired uint64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO asset_pipeline_state (asset_id,source_content_id,stage,pipeline_version,desired_version,applied_version,priority,terminal_error,updated_at)
			VALUES (?,?,?,?,1,0,?,NULL,?)
			ON CONFLICT(asset_id,stage) DO UPDATE SET source_content_id=excluded.source_content_id,pipeline_version=excluded.pipeline_version,desired_version=asset_pipeline_state.desired_version+1,priority=CASE WHEN asset_pipeline_state.desired_version>asset_pipeline_state.applied_version THEN MIN(asset_pipeline_state.priority,excluded.priority) ELSE excluded.priority END,terminal_error=NULL,updated_at=excluded.updated_at
			RETURNING desired_version`, assetID.String(), contentID.String(), string(stage), pipelineVersion, priority, now.UnixMicro()).Scan(&desired); err != nil {
			return fmt.Errorf("request asset %s: %w", stage, err)
		}
		// A correlation ID that names a reprocess/retry receipt binds that
		// user-visible operation to the exact stage generation requested here.
		// Other callers use arbitrary correlation IDs and produce no row.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO asset_pipeline_receipt_stages(receipt_id,asset_id,stage,desired_version)
			SELECT receipt_id,?,?,? FROM catalog_operation_receipts
			WHERE receipt_id=? AND kind IN ('reprocess','retry','reindex')
			ON CONFLICT(receipt_id,asset_id,stage) DO UPDATE SET desired_version=excluded.desired_version`,
			assetID.String(), string(stage), desired, correlationID.String()); err != nil {
			return fmt.Errorf("bind asset stage receipt: %w", err)
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE assets SET status='{"state":"processing"}',updated_at=? WHERE asset_id=?`, now.UnixMicro(), assetID.String())
	return err
}

// AssetStageReady is the catalog-owned asset DAG. It deliberately models
// prerequisites as desired/applied facts instead of relying on River retries
// to discover missing derivatives. Enrich can be requested alone for an
// already-ready generation; it waits only when the current generation has an
// outstanding prerequisite.
type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func AssetStageReady(ctx context.Context, queryer Queryer, assetID, sourceFence uuid.UUID, stage Stage, desiredVersion uint64, pipelineVersion string) (bool, error) {
	if queryer == nil || assetID == uuid.Nil || sourceFence == uuid.Nil || desiredVersion == 0 || strings.TrimSpace(pipelineVersion) == "" {
		return false, errors.New("invalid asset stage readiness identity")
	}
	if stage != StageEnrich {
		return true, nil
	}
	var assetType string
	if err := queryer.QueryRowContext(ctx, `SELECT type FROM assets WHERE asset_id=?`, assetID.String()).Scan(&assetType); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	prerequisites := []Stage{}
	switch assetType {
	case "PHOTO":
		prerequisites = []Stage{StageDerivatives}
	case "VIDEO":
		prerequisites = []Stage{StageDerivatives, StageTranscode}
	case "AUDIO":
		prerequisites = []Stage{StageTranscode}
	}
	for _, stage := range prerequisites {
		var fence, version string
		var desired, applied uint64
		var terminal sql.NullString
		err := queryer.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version,terminal_error FROM asset_pipeline_state WHERE asset_id=? AND stage=?`, assetID.String(), string(stage)).Scan(&fence, &version, &desired, &applied, &terminal)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		if fence == sourceFence.String() && version == pipelineVersion && (terminal.Valid || applied < desired) {
			return false, nil
		}
	}
	return true, nil
}

func RequestEventProjectionTx(ctx context.Context, tx *sql.Tx, ownerID int32, sourceRevision uint64, force bool) error {
	if tx == nil || ownerID <= 0 || sourceRevision == 0 {
		return errors.New("invalid event projection request")
	}
	now := time.Now().UTC()
	qos := workqos.Background
	if force {
		qos = workqos.Interactive
	}
	priority, _ := qos.Priority()
	_, err := tx.ExecContext(ctx, `INSERT INTO event_projection_pipeline_state (owner_id,source_revision,projection_version,applied_revision,priority,updated_at) VALUES (?,?,1,0,?,?) ON CONFLICT(owner_id) DO UPDATE SET source_revision=MAX(event_projection_pipeline_state.source_revision,excluded.source_revision),projection_version=event_projection_pipeline_state.projection_version+1,applied_revision=CASE WHEN ? THEN 0 ELSE event_projection_pipeline_state.applied_revision END,cursor=CASE WHEN ? THEN NULL ELSE event_projection_pipeline_state.cursor END,priority=CASE WHEN event_projection_pipeline_state.source_revision>event_projection_pipeline_state.applied_revision THEN MIN(event_projection_pipeline_state.priority,excluded.priority) ELSE excluded.priority END,terminal_error=NULL,updated_at=excluded.updated_at`, ownerID, sourceRevision, priority, now.UnixMicro(), force, force)
	return err
}

func RequestLocationProjectionTx(ctx context.Context, tx *sql.Tx, repositoryID uuid.UUID, ownerID int32) error {
	if tx == nil || repositoryID == uuid.Nil || ownerID <= 0 {
		return errors.New("invalid location projection request")
	}
	var revision uint64
	if err := tx.QueryRowContext(ctx, `SELECT source_revision FROM location_projection_state WHERE repository_id=? AND owner_id=?`, repositoryID.String(), ownerID).Scan(&revision); err != nil {
		return fmt.Errorf("read location source revision: %w", err)
	}
	if revision == 0 {
		return errors.New("location projection source revision is not initialized")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE location_projection_state SET terminal_error=NULL WHERE repository_id=? AND owner_id=?`, repositoryID.String(), ownerID); err != nil {
		return fmt.Errorf("clear location terminal failure: %w", err)
	}
	return nil
}

func RequestLocationResolutionTx(ctx context.Context, tx *sql.Tx, geocodingRevision uint64) error {
	if tx == nil || geocodingRevision == 0 {
		return errors.New("invalid geocoding revision")
	}
	now := time.Now().UTC()
	_, err := tx.ExecContext(ctx, `INSERT INTO location_resolution_pipeline_state(scope,source_revision,projection_version,applied_revision,updated_at) VALUES('all',?,1,0,?) ON CONFLICT(scope) DO UPDATE SET source_revision=MAX(location_resolution_pipeline_state.source_revision,excluded.source_revision),projection_version=location_resolution_pipeline_state.projection_version+1,terminal_error=NULL,updated_at=excluded.updated_at`, geocodingRevision, now.UnixMicro())
	return err
}

func RequestReindexTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, repositoryID *uuid.UUID, tasks []string, limit int, missingOnly, resetSemantic bool) error {
	if tx == nil || receiptID == uuid.Nil || len(tasks) == 0 || limit < 1 || limit > 500 {
		return errors.New("invalid reindex request")
	}
	if repositoryID != nil && *repositoryID == uuid.Nil {
		return errors.New("invalid reindex repository")
	}
	for _, task := range tasks {
		if strings.TrimSpace(task) == "" {
			return errors.New("reindex task name is empty")
		}
	}
	now := time.Now().UTC()
	encodedTasks, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	priority, _ := workqos.Interactive.Priority()
	var repository any
	if repositoryID != nil {
		repository = repositoryID.String()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at)VALUES(?, 'reindex', ?,1,'pending',?,?)`, receiptID.String(), receiptID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO asset_reindex_requests(receipt_id,repository_id,tasks,page_limit,missing_only,reset_semantic,priority,updated_at)VALUES(?,?,?,?,?,?,?,?)`, receiptID.String(), repository, string(encodedTasks), limit, missingOnly, resetSemantic, priority, now.UnixMicro()); err != nil {
		return err
	}
	return nil
}

func AdvanceReindexTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, requestedRevision uint64, nextCursor string) error {
	nextCursor = strings.TrimSpace(nextCursor)
	if tx == nil || receiptID == uuid.Nil || requestedRevision == 0 || strings.TrimSpace(nextCursor) == "" {
		return errors.New("invalid reindex continuation")
	}
	nextRevision := requestedRevision + 1
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE asset_reindex_requests SET cursor=?,requested_revision=?,updated_at=? WHERE receipt_id=? AND requested_revision=? AND applied_revision<?`, nextCursor, nextRevision, now.UnixMicro(), receiptID.String(), requestedRevision, requestedRevision)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return nil
	}
	return nil
}

func FinishReindexTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, requestedRevision uint64) error {
	if tx == nil || receiptID == uuid.Nil || requestedRevision == 0 {
		return errors.New("invalid reindex completion")
	}
	now := time.Now().UTC().UnixMicro()
	result, err := tx.ExecContext(ctx, `UPDATE asset_reindex_requests SET applied_revision=requested_revision,updated_at=? WHERE receipt_id=? AND requested_revision=?`, now, receiptID.String(), requestedRevision)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state='completed',applied_version=desired_version,updated_at=? WHERE receipt_id=? AND state='pending' AND NOT EXISTS(SELECT 1 FROM asset_pipeline_receipt_stages link JOIN asset_pipeline_state stage ON stage.asset_id=link.asset_id AND stage.stage=link.stage WHERE link.receipt_id=? AND stage.applied_version<link.desired_version)`, now, receiptID.String(), receiptID.String())
	return err
}

func RequestBackupTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, force bool) error {
	return RequestBackupWithQoSTx(ctx, tx, receiptID, force, workqos.Interactive)
}

func RequestBackupWithQoSTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, force bool, qos workqos.Class) error {
	priority, err := qos.Priority()
	if tx == nil || receiptID == uuid.Nil || err != nil {
		return errors.New("invalid backup receipt")
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES(?, 'backup', ?, 1, 'pending', ?, ?)`, receiptID.String(), receiptID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_backup_requests(receipt_id,force,priority) VALUES(?,?,?)`, receiptID.String(), force, priority); err != nil {
		return err
	}
	return nil
}
