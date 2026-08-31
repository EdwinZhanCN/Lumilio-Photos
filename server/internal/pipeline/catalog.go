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
)

// PublishEnvelopeTx is the only generic part of the catalog handoff. Its
// input is already a domain envelope, never River arguments or insert options.
func PublishEnvelopeTx(ctx context.Context, tx *sql.Tx, subject string, desiredVersion uint64, envelope Envelope) error {
	if ctx == nil || tx == nil || subject == "" || desiredVersion == 0 || envelope.Version != EnvelopeVersion || envelope.Kind == "" || envelope.TraceID == uuid.Nil || envelope.CorrelationID == uuid.Nil || len(envelope.Payload) == 0 {
		return errors.New("invalid domain outbox publication")
	}
	if err := validateEncodedCommand(envelope.Kind, envelope.Payload); err != nil {
		return fmt.Errorf("validate domain command: %w", err)
	}
	commandSubject, commandVersion, err := encodedCommandIdentity(envelope.Kind, envelope.Payload)
	if err != nil {
		return fmt.Errorf("read domain command identity: %w", err)
	}
	if subject != commandSubject || desiredVersion != commandVersion {
		return fmt.Errorf("domain outbox identity (%q,%d) does not match command (%q,%d)", subject, desiredVersion, commandSubject, commandVersion)
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode domain envelope: %w", err)
	}
	now := time.Now().UTC().UnixMicro()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO domain_outbox (outbox_id,envelope_version,command_kind,subject_key,desired_version,envelope,available_at,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(command_kind,subject_key,desired_version) DO UPDATE SET
			delivered_at = NULL, available_at = excluded.available_at,
			last_error = NULL, updated_at = excluded.updated_at`,
		uuid.NewString(), envelope.Version, envelope.Kind, subject, desiredVersion,
		string(encoded), now, now, now)
	return err
}

// encodedCommandIdentity extracts the catalog uniqueness key duplicated in a
// domain_outbox row. Keeping this check at publication time prevents a caller
// from creating a durable row whose routing identity disagrees with its typed
// command; the River adapter repeats the same check at delivery time.
func encodedCommandIdentity(kind string, payload []byte) (string, uint64, error) {
	switch kind {
	case "ingest_asset":
		var command IngestCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.CommitID.String(), 1, nil
	case "asset.analyze", "asset.derivatives", "asset.transcode", "asset.enrich":
		var command AssetCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.AssetID.String(), command.DesiredVersion, nil
	case "repository.scan":
		var command RepositoryCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.RepositoryID.String(), command.RequestedEpoch, nil
	case "projection.event", "projection.location", "projection.ocr":
		var command ProjectionCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.Scope, command.ProjectionVersion, nil
	case "projection.asset_reindex":
		var command ReindexCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.ReceiptID.String(), command.RequestedRevision, nil
	case "backup_catalog":
		var command BackupCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			return "", 0, err
		}
		return command.RequestID.String(), 1, nil
	default:
		return "", 0, fmt.Errorf("unsupported domain command kind %q", kind)
	}
}

func RequestIngestTx(ctx context.Context, tx *sql.Tx, commitID, receiptID uuid.UUID, traceID uuid.UUID) error {
	if tx == nil || commitID == uuid.Nil || receiptID == uuid.Nil {
		return errors.New("ingest request requires commit and receipt IDs")
	}
	if traceID == uuid.Nil {
		traceID = uuid.New()
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts (receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES (?,?,?,1,'pending',?,?)`, receiptID.String(), "ingest", commitID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return fmt.Errorf("create ingest receipt: %w", err)
	}
	envelope, err := NewEnvelope("ingest_asset", traceID, receiptID, IngestCommand{CommitID: commitID, ReceiptID: receiptID, Admission: AdmissionInteractive}, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, commitID.String(), 1, envelope)
}

func RequestAssetStagesTx(ctx context.Context, tx *sql.Tx, assetID, contentID uuid.UUID, stages []Stage, pipelineVersion string, admission AdmissionClass, correlationID uuid.UUID) error {
	if tx == nil || assetID == uuid.Nil || contentID == uuid.Nil || strings.TrimSpace(pipelineVersion) == "" || len(stages) == 0 || !admission.Valid() {
		return errors.New("invalid asset pipeline request")
	}
	fence, _ := NewSourceFence(contentID)
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	now := time.Now().UTC()
	seenStages := make(map[Stage]struct{}, len(stages))
	commands := make([]AssetCommand, 0, len(stages))
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
			INSERT INTO asset_pipeline_state (asset_id,source_content_id,stage,pipeline_version,desired_version,applied_version,terminal_error,updated_at)
			VALUES (?,?,?,?,1,0,NULL,?)
			ON CONFLICT(asset_id,stage) DO UPDATE SET source_content_id=excluded.source_content_id,pipeline_version=excluded.pipeline_version,desired_version=asset_pipeline_state.desired_version+1,terminal_error=NULL,updated_at=excluded.updated_at
			RETURNING desired_version`, assetID.String(), contentID.String(), string(stage), pipelineVersion, now.UnixMicro()).Scan(&desired); err != nil {
			return fmt.Errorf("request asset %s: %w", stage, err)
		}
		command := AssetCommand{AssetID: assetID, Fence: fence, Stage: stage, DesiredVersion: desired, PipelineVersion: pipelineVersion, Admission: admission}
		commands = append(commands, command)
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
	for _, command := range commands {
		ready, err := AssetStageReadyTx(ctx, tx, command)
		if err != nil {
			return err
		}
		if !ready {
			continue
		}
		envelope, err := NewEnvelope("asset."+string(command.Stage), uuid.New(), correlationID, command, now)
		if err != nil {
			return err
		}
		if err := PublishEnvelopeTx(ctx, tx, assetID.String(), command.DesiredVersion, envelope); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE assets SET status='{"state":"processing"}',updated_at=? WHERE asset_id=?`, now.UnixMicro(), assetID.String())
	return err
}

// AssetStageReadyTx is the catalog-owned asset DAG. It deliberately models
// prerequisites as desired/applied facts instead of relying on River retries
// to discover missing derivatives. Enrich can be requested alone for an
// already-ready generation; it waits only when the current generation has an
// outstanding prerequisite.
func AssetStageReadyTx(ctx context.Context, tx *sql.Tx, command AssetCommand) (bool, error) {
	if tx == nil || command.AssetID == uuid.Nil || command.Fence.UUID() == uuid.Nil || command.DesiredVersion == 0 || strings.TrimSpace(command.PipelineVersion) == "" {
		return false, errors.New("invalid asset stage readiness command")
	}
	if command.Stage != StageEnrich {
		return true, nil
	}
	var assetType string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM assets WHERE asset_id=?`, command.AssetID.String()).Scan(&assetType); errors.Is(err, sql.ErrNoRows) {
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
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version,terminal_error FROM asset_pipeline_state WHERE asset_id=? AND stage=?`, command.AssetID.String(), string(stage)).Scan(&fence, &version, &desired, &applied, &terminal)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, err
		}
		if fence == command.Fence.String() && version == command.PipelineVersion && (terminal.Valid || applied < desired) {
			return false, nil
		}
	}
	return true, nil
}

// PublishReadyAssetStagesTx is called by a successful prerequisite commit and
// by reconciliation. It emits only stages whose catalog prerequisites are now
// satisfied; it never inserts River jobs itself.
func PublishReadyAssetStagesTx(ctx context.Context, tx *sql.Tx, assetID uuid.UUID, admission AdmissionClass) error {
	if tx == nil || assetID == uuid.Nil || !admission.Valid() {
		return errors.New("invalid ready asset stage publication")
	}
	rows, err := tx.QueryContext(ctx, `SELECT source_content_id,pipeline_version,desired_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich' AND desired_version>applied_version AND terminal_error IS NULL`, assetID.String())
	if err != nil {
		return err
	}
	defer rows.Close()
	now := time.Now().UTC()
	for rows.Next() {
		var fenceText, version string
		var desired uint64
		if err := rows.Scan(&fenceText, &version, &desired); err != nil {
			return err
		}
		fenceID, err := uuid.Parse(fenceText)
		if err != nil {
			return err
		}
		fence, err := NewSourceFence(fenceID)
		if err != nil {
			return err
		}
		command := AssetCommand{AssetID: assetID, Fence: fence, Stage: StageEnrich, DesiredVersion: desired, PipelineVersion: version, Admission: admission}
		ready, err := AssetStageReadyTx(ctx, tx, command)
		if err != nil || !ready {
			if err != nil {
				return err
			}
			continue
		}
		envelope, err := NewEnvelope("asset.enrich", uuid.New(), uuid.New(), command, now)
		if err != nil {
			return err
		}
		if err := PublishEnvelopeTx(ctx, tx, assetID.String(), desired, envelope); err != nil {
			return err
		}
	}
	return rows.Err()
}

func RequestEventProjectionTx(ctx context.Context, tx *sql.Tx, ownerID int32, sourceRevision uint64, force bool, correlationID uuid.UUID) error {
	if tx == nil || ownerID <= 0 || sourceRevision == 0 {
		return errors.New("invalid event projection request")
	}
	now := time.Now().UTC()
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	var effectiveRevision, desired uint64
	if err := tx.QueryRowContext(ctx, `INSERT INTO event_projection_pipeline_state (owner_id,source_revision,projection_version,applied_revision,updated_at) VALUES (?,?,1,0,?) ON CONFLICT(owner_id) DO UPDATE SET source_revision=MAX(event_projection_pipeline_state.source_revision,excluded.source_revision),projection_version=event_projection_pipeline_state.projection_version+1,applied_revision=CASE WHEN ? THEN 0 ELSE event_projection_pipeline_state.applied_revision END,cursor=CASE WHEN ? THEN NULL ELSE event_projection_pipeline_state.cursor END,terminal_error=NULL,updated_at=excluded.updated_at RETURNING source_revision,projection_version`, ownerID, sourceRevision, now.UnixMicro(), force, force).Scan(&effectiveRevision, &desired); err != nil {
		return err
	}
	command := ProjectionCommand{Kind: "event", Scope: fmt.Sprint(ownerID), SourceRevision: effectiveRevision, ProjectionVersion: desired, Admission: AdmissionBackground}
	if force {
		command.Admission = AdmissionInteractive
	}
	envelope, err := NewEnvelope("projection.event", uuid.New(), correlationID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, command.Scope, desired, envelope)
}

func RequestLocationProjectionTx(ctx context.Context, tx *sql.Tx, repositoryID uuid.UUID, ownerID int32, correlationID uuid.UUID) error {
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
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	now := time.Now().UTC()
	command := ProjectionCommand{Kind: "location", Scope: repositoryID.String() + ":" + fmt.Sprint(ownerID), SourceRevision: revision, ProjectionVersion: revision, Admission: AdmissionBackground}
	envelope, err := NewEnvelope("projection.location", uuid.New(), correlationID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, command.Scope, revision, envelope)
}

func PublishRepositoryObservationTx(ctx context.Context, tx *sql.Tx, repositoryID uuid.UUID, requestedEpoch uint64, admission AdmissionClass, correlationID uuid.UUID) error {
	if tx == nil || repositoryID == uuid.Nil || requestedEpoch == 0 || !admission.Valid() {
		return errors.New("invalid repository observation request")
	}
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	now := time.Now().UTC()
	command := RepositoryCommand{RepositoryID: repositoryID, RequestedEpoch: requestedEpoch, DesiredVersion: requestedEpoch, Admission: admission}
	envelope, err := NewEnvelope("repository.scan", uuid.New(), correlationID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, repositoryID.String(), requestedEpoch, envelope)
}

func RequestLocationResolutionTx(ctx context.Context, tx *sql.Tx, geocodingRevision uint64, correlationID uuid.UUID) error {
	if tx == nil || geocodingRevision == 0 {
		return errors.New("invalid geocoding revision")
	}
	if correlationID == uuid.Nil {
		correlationID = uuid.New()
	}
	now := time.Now().UTC()
	var effectiveRevision, desired uint64
	if err := tx.QueryRowContext(ctx, `INSERT INTO location_resolution_pipeline_state(scope,source_revision,projection_version,applied_revision,updated_at) VALUES('all',?,1,0,?) ON CONFLICT(scope) DO UPDATE SET source_revision=MAX(location_resolution_pipeline_state.source_revision,excluded.source_revision),projection_version=location_resolution_pipeline_state.projection_version+1,terminal_error=NULL,updated_at=excluded.updated_at RETURNING source_revision,projection_version`, geocodingRevision, now.UnixMicro()).Scan(&effectiveRevision, &desired); err != nil {
		return err
	}
	command := ProjectionCommand{Kind: "location_resolution", Scope: "all", SourceRevision: effectiveRevision, ProjectionVersion: desired, Admission: AdmissionBackground}
	envelope, err := NewEnvelope("projection.location", uuid.New(), correlationID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, command.Scope, desired, envelope)
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
	var repository any
	if repositoryID != nil {
		repository = repositoryID.String()
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at)VALUES(?, 'reindex', ?,1,'pending',?,?)`, receiptID.String(), receiptID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO asset_reindex_requests(receipt_id,repository_id,tasks,page_limit,missing_only,reset_semantic,updated_at)VALUES(?,?,?,?,?,?,?)`, receiptID.String(), repository, string(encodedTasks), limit, missingOnly, resetSemantic, now.UnixMicro()); err != nil {
		return err
	}
	command := ReindexCommand{ReceiptID: receiptID, RepositoryID: repositoryID, Tasks: tasks, Limit: limit, MissingOnly: missingOnly, ResetSemantic: resetSemantic, RequestedRevision: 1, Admission: AdmissionInteractive}
	envelope, err := NewEnvelope("projection.asset_reindex", uuid.New(), receiptID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, receiptID.String(), 1, envelope)
}

func AdvanceReindexTx(ctx context.Context, tx *sql.Tx, command ReindexCommand, nextCursor string) error {
	nextCursor = strings.TrimSpace(nextCursor)
	if tx == nil || command.ReceiptID == uuid.Nil || command.RequestedRevision == 0 || strings.TrimSpace(nextCursor) == "" || !command.Admission.Valid() {
		return errors.New("invalid reindex continuation")
	}
	nextRevision := command.RequestedRevision + 1
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE asset_reindex_requests SET cursor=?,requested_revision=?,updated_at=? WHERE receipt_id=? AND requested_revision=? AND applied_revision<?`, nextCursor, nextRevision, now.UnixMicro(), command.ReceiptID.String(), command.RequestedRevision, command.RequestedRevision)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return nil
	}
	command.Cursor = nextCursor
	command.RequestedRevision = nextRevision
	envelope, err := NewEnvelope("projection.asset_reindex", uuid.New(), command.ReceiptID, command, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, command.ReceiptID.String(), nextRevision, envelope)
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
	return RequestBackupWithAdmissionTx(ctx, tx, receiptID, force, AdmissionInteractive)
}

func RequestBackupWithAdmissionTx(ctx context.Context, tx *sql.Tx, receiptID uuid.UUID, force bool, admission AdmissionClass) error {
	if tx == nil || receiptID == uuid.Nil || !admission.Valid() {
		return errors.New("invalid backup receipt")
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES(?, 'backup', ?, 1, 'pending', ?, ?)`, receiptID.String(), receiptID.String(), now.UnixMicro(), now.UnixMicro()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_backup_requests(receipt_id,force) VALUES(?,?)`, receiptID.String(), force); err != nil {
		return err
	}
	envelope, err := NewEnvelope("backup_catalog", uuid.New(), receiptID, BackupCommand{RequestID: receiptID, Force: force, Admission: admission}, now)
	if err != nil {
		return err
	}
	return PublishEnvelopeTx(ctx, tx, receiptID.String(), 1, envelope)
}
