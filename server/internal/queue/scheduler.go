package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"server/internal/db/catalogtx"
	"server/internal/pipeline"
	"server/internal/queue/jobs"
	"server/internal/workqos"
)

type derivedWork struct {
	args river.JobArgs
	qos  workqos.Class
}

// Scheduler derives disposable River work directly from Catalog state. A
// missing or rebuilt QueueDB only removes delivery rows; the next pass derives
// the same work again from desired/applied state.
type Scheduler struct {
	reader    *catalogtx.Reader
	writer    *catalogtx.Writer
	client    *river.Client[*sql.Tx]
	batchSize int
	interval  time.Duration
	wake      <-chan struct{}
}

func NewScheduler(reader *catalogtx.Reader, writer *catalogtx.Writer, client *river.Client[*sql.Tx], wake <-chan struct{}, batchSize int, interval time.Duration) (*Scheduler, error) {
	if reader == nil || reader.Pool() == nil || writer == nil || client == nil || wake == nil || batchSize < 1 || interval <= 0 {
		return nil, errors.New("catalog scheduler requires catalog capabilities, River client, and positive bounds")
	}
	return &Scheduler{reader: reader, writer: writer, client: client, wake: wake, batchSize: batchSize, interval: interval}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s == nil || s.reader == nil || s.writer == nil || s.client == nil {
		return errors.New("catalog scheduler is not configured")
	}
	if ctx == nil {
		return errors.New("catalog scheduler context is nil")
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if _, err := s.ScheduleOnce(ctx); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			// Catalog remains authoritative after a transient QueueDB or catalog
			// error. The next bounded pass retries derivation.
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		case <-s.wake:
		}
	}
}

func (s *Scheduler) ScheduleOnce(ctx context.Context) (int, error) {
	if s == nil || s.reader == nil || s.writer == nil || s.client == nil {
		return 0, errors.New("catalog scheduler is not configured")
	}
	if ctx == nil {
		return 0, errors.New("catalog scheduling context is nil")
	}
	if err := s.ensureOCRState(ctx); err != nil {
		return 0, err
	}
	work, err := s.derive(ctx)
	if err != nil {
		return 0, err
	}
	inserted := 0
	for start := 0; start < len(work); start += s.batchSize {
		end := start + s.batchSize
		if end > len(work) {
			end = len(work)
		}
		params := make([]river.InsertManyParams, 0, end-start)
		for _, item := range work[start:end] {
			priority, err := item.qos.Priority()
			if err != nil {
				return inserted, err
			}
			params = append(params, river.InsertManyParams{Args: item.args, InsertOpts: &river.InsertOpts{Priority: priority}})
		}
		results, err := s.client.InsertMany(ctx, params)
		if err != nil {
			return inserted, fmt.Errorf("insert derived catalog work: %w", err)
		}
		for _, result := range results {
			if result != nil && !result.UniqueSkippedAsDuplicate {
				inserted++
			}
		}
	}
	return inserted, nil
}

func (s *Scheduler) ensureOCRState(ctx context.Context) error {
	return s.writer.Transact(ctx, catalogtx.OperationCatalogWorkStateRepair, nil, func(tx *sql.Tx) error {
		var revision uint64
		if err := tx.QueryRowContext(ctx, `SELECT coalesce(max(revision),0) FROM ocr_index_outbox`).Scan(&revision); err != nil {
			return err
		}
		if revision == 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO ocr_projection_pipeline_state(scope,source_revision,projection_version,applied_revision,updated_at) VALUES('all',?,?,0,?) ON CONFLICT(scope) DO UPDATE SET source_revision=MAX(ocr_projection_pipeline_state.source_revision,excluded.source_revision),projection_version=MAX(ocr_projection_pipeline_state.projection_version,excluded.projection_version),updated_at=excluded.updated_at`, revision, revision, time.Now().UTC().UnixMicro())
		return err
	})
}

func (s *Scheduler) derive(ctx context.Context) ([]derivedWork, error) {
	db := s.reader.Pool()
	work := make([]derivedWork, 0, s.batchSize)

	ingestRows, err := db.QueryContext(ctx, `SELECT receipt_id,subject_id FROM catalog_operation_receipts WHERE kind='ingest' AND state='pending' ORDER BY updated_at,receipt_id LIMIT ?`, s.batchSize)
	if err != nil {
		return nil, err
	}
	for ingestRows.Next() {
		var receiptRaw, commitRaw string
		if err := ingestRows.Scan(&receiptRaw, &commitRaw); err != nil {
			_ = ingestRows.Close()
			return nil, err
		}
		receiptID, err := uuid.Parse(receiptRaw)
		if err != nil {
			_ = ingestRows.Close()
			return nil, err
		}
		commitID, err := uuid.Parse(commitRaw)
		if err != nil {
			_ = ingestRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: jobs.IngestAssetArgs{CommitID: commitID, ReceiptID: receiptID}, qos: workqos.Interactive})
	}
	if err := ingestRows.Err(); err != nil {
		_ = ingestRows.Close()
		return nil, err
	}
	if err := ingestRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	assetRows, err := db.QueryContext(ctx, `SELECT asset_id,source_content_id,stage,pipeline_version,desired_version,priority FROM asset_pipeline_state WHERE desired_version>applied_version AND terminal_error IS NULL ORDER BY updated_at,asset_id,stage LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for assetRows.Next() {
		var assetRaw, fenceRaw, stageRaw, version string
		var desired uint64
		var priority int
		if err := assetRows.Scan(&assetRaw, &fenceRaw, &stageRaw, &version, &desired, &priority); err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		assetID, err := uuid.Parse(assetRaw)
		if err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		fence, err := uuid.Parse(fenceRaw)
		if err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		ready, err := pipeline.AssetStageReady(ctx, db, assetID, fence, pipeline.Stage(stageRaw), desired, version)
		if err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		if !ready {
			continue
		}
		assetArgs, err := assetJobArgs(assetID, fence, desired, version, pipeline.Stage(stageRaw))
		if err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		qos, err := workqos.FromPriority(priority)
		if err != nil {
			_ = assetRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: assetArgs, qos: qos})
	}
	if err := assetRows.Err(); err != nil {
		_ = assetRows.Close()
		return nil, err
	}
	if err := assetRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	repositoryRows, err := db.QueryContext(ctx, `SELECT state.repository_id,active.requested_epoch,active.mode FROM repository_observation_state AS state JOIN repository_scan_runs AS active ON active.run_id=state.active_run_id AND active.repository_id=state.repository_id AND active.status IN ('queued','crawling','catching_up','finalizing') WHERE (state.desired_epoch>state.applied_epoch OR state.full_verification_required=1) AND state.terminal_error IS NULL ORDER BY state.updated_at,state.repository_id LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for repositoryRows.Next() {
		var repositoryRaw string
		var mode string
		var epoch uint64
		if err := repositoryRows.Scan(&repositoryRaw, &epoch, &mode); err != nil {
			_ = repositoryRows.Close()
			return nil, err
		}
		repositoryID, err := uuid.Parse(repositoryRaw)
		if err != nil {
			_ = repositoryRows.Close()
			return nil, err
		}
		qos := workqos.Background
		if mode == "manual" {
			qos = workqos.Interactive
		}
		work = append(work, derivedWork{args: jobs.ScanRepositoryBatchArgs{RepositoryID: repositoryID, RequestedEpoch: epoch, DesiredVersion: epoch}, qos: qos})
	}
	if err := repositoryRows.Err(); err != nil {
		_ = repositoryRows.Close()
		return nil, err
	}
	if err := repositoryRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	var eventRows *sql.Rows
	eventRows, err = db.QueryContext(ctx, `SELECT owner_id,source_revision,projection_version,cursor,priority FROM event_projection_pipeline_state WHERE source_revision>applied_revision AND terminal_error IS NULL ORDER BY updated_at,owner_id LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for eventRows.Next() {
		var owner int32
		var source, version uint64
		var cursor sql.NullString
		var priority int
		if err := eventRows.Scan(&owner, &source, &version, &cursor, &priority); err != nil {
			_ = eventRows.Close()
			return nil, err
		}
		qos, err := workqos.FromPriority(priority)
		if err != nil {
			_ = eventRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: jobs.RebuildProjectionBatchArgs{ProjectionKind: "event", Scope: strconv.FormatInt(int64(owner), 10), SourceRevision: source, ProjectionVersion: version, Cursor: cursor.String}, qos: qos})
	}
	if err := eventRows.Err(); err != nil {
		_ = eventRows.Close()
		return nil, err
	}
	if err := eventRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	locationRows, err := db.QueryContext(ctx, `SELECT repository_id,owner_id,source_revision FROM location_projection_state WHERE source_revision>published_revision AND terminal_error IS NULL ORDER BY updated_at,repository_id,owner_id LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for locationRows.Next() {
		var repositoryRaw string
		var owner int32
		var revision uint64
		if err := locationRows.Scan(&repositoryRaw, &owner, &revision); err != nil {
			_ = locationRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: jobs.RebuildProjectionBatchArgs{ProjectionKind: "location", Scope: repositoryRaw + ":" + strconv.FormatInt(int64(owner), 10), SourceRevision: revision, ProjectionVersion: revision}, qos: workqos.Background})
	}
	if err := locationRows.Err(); err != nil {
		_ = locationRows.Close()
		return nil, err
	}
	if err := locationRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	var resolutionSource, resolutionVersion uint64
	if err := db.QueryRowContext(ctx, `SELECT source_revision,projection_version FROM location_resolution_pipeline_state WHERE scope='all' AND projection_version>applied_revision AND terminal_error IS NULL`).Scan(&resolutionSource, &resolutionVersion); err == nil {
		work = append(work, derivedWork{args: jobs.RebuildProjectionBatchArgs{ProjectionKind: "location_resolution", Scope: "all", SourceRevision: resolutionSource, ProjectionVersion: resolutionVersion}, qos: workqos.Background})
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	var ocrSource, ocrVersion uint64
	if err := db.QueryRowContext(ctx, `SELECT source_revision,projection_version FROM ocr_projection_pipeline_state WHERE scope='all' AND projection_version>applied_revision AND terminal_error IS NULL`).Scan(&ocrSource, &ocrVersion); err == nil {
		work = append(work, derivedWork{args: jobs.RebuildProjectionBatchArgs{ProjectionKind: "ocr", Scope: "all", SourceRevision: ocrSource, ProjectionVersion: ocrVersion}, qos: workqos.Background})
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	reindexRows, err := db.QueryContext(ctx, `SELECT r.receipt_id,q.requested_revision,q.cursor,q.priority FROM catalog_operation_receipts r JOIN asset_reindex_requests q USING(receipt_id) WHERE r.state='pending' AND q.requested_revision>q.applied_revision ORDER BY r.updated_at,r.receipt_id LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for reindexRows.Next() {
		var receiptRaw string
		var revision uint64
		var cursor sql.NullString
		var priority int
		if err := reindexRows.Scan(&receiptRaw, &revision, &cursor, &priority); err != nil {
			_ = reindexRows.Close()
			return nil, err
		}
		receiptID, err := uuid.Parse(receiptRaw)
		if err != nil {
			_ = reindexRows.Close()
			return nil, err
		}
		qos, err := workqos.FromPriority(priority)
		if err != nil {
			_ = reindexRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: jobs.RebuildProjectionBatchArgs{ProjectionKind: "asset_reindex", Scope: receiptID.String(), SourceRevision: revision, ProjectionVersion: revision, Cursor: cursor.String}, qos: qos})
	}
	if err := reindexRows.Err(); err != nil {
		_ = reindexRows.Close()
		return nil, err
	}
	if err := reindexRows.Close(); err != nil {
		return nil, err
	}
	if len(work) >= s.batchSize {
		return work, nil
	}

	backupRows, err := db.QueryContext(ctx, `SELECT r.receipt_id,b.force,b.priority FROM catalog_operation_receipts r JOIN catalog_backup_requests b USING(receipt_id) WHERE r.kind='backup' AND r.state='pending' ORDER BY r.updated_at,r.receipt_id LIMIT ?`, s.batchSize-len(work))
	if err != nil {
		return nil, err
	}
	for backupRows.Next() {
		var receiptRaw string
		var force bool
		var priority int
		if err := backupRows.Scan(&receiptRaw, &force, &priority); err != nil {
			_ = backupRows.Close()
			return nil, err
		}
		receiptID, err := uuid.Parse(receiptRaw)
		if err != nil {
			_ = backupRows.Close()
			return nil, err
		}
		qos, err := workqos.FromPriority(priority)
		if err != nil {
			_ = backupRows.Close()
			return nil, err
		}
		work = append(work, derivedWork{args: jobs.BackupCatalogArgs{RequestID: receiptID, Force: force}, qos: qos})
	}
	if err := backupRows.Err(); err != nil {
		_ = backupRows.Close()
		return nil, err
	}
	if err := backupRows.Close(); err != nil {
		return nil, err
	}
	return work, nil
}

func assetJobArgs(assetID, sourceFence uuid.UUID, desired uint64, version string, stage pipeline.Stage) (river.JobArgs, error) {
	switch stage {
	case pipeline.StageAnalyze:
		return jobs.AnalyzeAssetArgs{AssetID: assetID, SourceFence: sourceFence, DesiredVersion: desired, PipelineVersion: version}, nil
	case pipeline.StageDerivatives:
		return jobs.GenerateAssetDerivativesArgs{AssetID: assetID, SourceFence: sourceFence, DesiredVersion: desired, PipelineVersion: version}, nil
	case pipeline.StageTranscode:
		return jobs.TranscodeMediaArgs{AssetID: assetID, SourceFence: sourceFence, DesiredVersion: desired, PipelineVersion: version}, nil
	case pipeline.StageEnrich:
		return jobs.EnrichAssetArgs{AssetID: assetID, SourceFence: sourceFence, DesiredVersion: desired, PipelineVersion: version}, nil
	default:
		return nil, fmt.Errorf("unsupported Catalog asset stage %q", stage)
	}
}

// BackupScheduler owns the periodic creation of a Catalog backup receipt.
// The Scheduler above then derives its disposable River work from that row.
type BackupScheduler struct {
	writer   *catalogtx.Writer
	interval time.Duration
}

func NewBackupScheduler(writer *catalogtx.Writer, interval time.Duration) (*BackupScheduler, error) {
	if writer == nil || interval <= 0 {
		return nil, errors.New("backup scheduler requires a catalog writer and positive interval")
	}
	return &BackupScheduler{writer: writer, interval: interval}, nil
}

func (s *BackupScheduler) Request(ctx context.Context, force bool) (uuid.UUID, error) {
	if s == nil || s.writer == nil {
		return uuid.Nil, errors.New("backup scheduler is not configured")
	}
	if ctx == nil {
		return uuid.Nil, errors.New("backup scheduler context is nil")
	}
	receiptID := uuid.New()
	err := s.writer.Transact(ctx, catalogtx.OperationBackupRequest, nil, func(tx *sql.Tx) error {
		return pipeline.RequestBackupWithQoSTx(ctx, tx, receiptID, force, workqos.Maintenance)
	})
	return receiptID, err
}

func (s *BackupScheduler) Run(ctx context.Context) error {
	if s == nil || s.writer == nil {
		return errors.New("backup scheduler is not configured")
	}
	if ctx == nil {
		return errors.New("backup scheduler context is nil")
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		if _, err := s.Request(ctx, false); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
