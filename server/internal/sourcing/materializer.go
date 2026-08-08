package sourcing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"server/internal/db"
	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/file"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

const (
	TaskMetadata  = "metadata_asset"
	TaskThumbnail = "thumbnail_asset"
	TaskTranscode = "transcode_asset"

	preparedStatusMessage = "Asset ingestion prepared"
	queuedStatusMessage   = "Asset queued for processing"

	ingestCodeCommitFailed = "staging_commit_failed"
	ingestCodeConflict     = "final_path_conflict"
)

// SourceMaterializer owns staged ingest. Files already in the user workspace
// use MaterializeDiscovered instead; there is no path-based skip-commit mode.
type SourceMaterializer struct {
	database       *db.DB
	queries        *repo.Queries
	stagingManager storage.StagingManager
	files          *storage.RepositoryFSFactory
	queueClient    *river.Client[*sql.Tx]
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	contentLocks   [256]sync.Mutex
}

func NewSourceMaterializer(
	database *db.DB,
	stagingManager storage.StagingManager,
	queueClient *river.Client[*sql.Tx],
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *storage.RepositoryFSFactory,
) *SourceMaterializer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger, false)
	}
	return &SourceMaterializer{
		database:       database,
		queries:        database.Queries,
		stagingManager: stagingManager,
		files:          files,
		queueClient:    queueClient,
		logger:         logger.With(zap.String("component", "source_materializer")),
		auditProvider:  auditProvider,
	}
}

// MaterializeStaged validates and commits one private-workspace file into
// Inbox. The prepared asset is the durable claim across the filesystem/SQLite
// boundary; a retry either resumes from staging or verifies the committed file.
func (m *SourceMaterializer) MaterializeStaged(ctx context.Context, source IngestSource) (*repo.Asset, error) {
	if source.Kind != IngestSourceUpload && source.Kind != IngestSourceCloud {
		return nil, fmt.Errorf("source %q is not staged ingest", source.Kind)
	}
	validation := file.ValidateFile(source.OriginalFilename, source.ContentType)
	if !validation.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorReason)
	}
	repository, err := m.resolveRepository(ctx, source.RepositoryID)
	if err != nil {
		return nil, err
	}
	stagingFile, err := stagingHandle(repository.RepoID, source)
	if err != nil {
		return nil, err
	}

	opened, err := m.stagingManager.OpenStagingFile(repository, stagingFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && source.ContentHash != nil {
			return m.recoverPreparedIngest(ctx, repository, source, validation.AssetType)
		}
		return nil, fmt.Errorf("open staged file: %w", err)
	}
	infoBefore, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, fmt.Errorf("stat staged file: %w", err)
	}
	hashes, err := hash.CalculateLayeredBLAKE3Reader(opened, infoBefore.Size())
	if err != nil {
		_ = opened.Close()
		return nil, fmt.Errorf("hash staged file: %w", err)
	}
	infoAfter, err := opened.Stat()
	closeErr := opened.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if infoBefore.Size() != infoAfter.Size() || !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		return nil, storage.ErrRepositoryFileUnstable
	}
	if source.ContentHash != nil && hash.ValidateHash(strings.TrimSpace(*source.ContentHash), hash.AlgorithmBLAKE3) &&
		!strings.EqualFold(strings.TrimSpace(*source.ContentHash), hashes.ContentHash) {
		return nil, errors.New("staged file does not match the server-verified upload hash")
	}

	lockIndex, _ := strconv.ParseUint(hashes.ContentHash[:2], 16, 8)
	m.contentLocks[lockIndex].Lock()
	defer m.contentLocks[lockIndex].Unlock()

	recoverable, duplicate, err := m.findContentCandidates(ctx, repository.RepoID, hashes.ContentHash, hashes.FileSize, stagingFile.PrivatePath)
	if err != nil {
		return nil, err
	}
	if duplicate != nil {
		verified, verifyErr := m.verifyAssetFile(ctx, repository, duplicate, hashes.ContentHash, hashes.FileSize)
		if verifyErr == nil && verified {
			if err := m.stagingManager.RemoveStagingFile(repository, stagingFile); err != nil {
				return nil, err
			}
			return duplicate, nil
		}
	}

	targetPath, err := m.stagingManager.ResolveInboxPath(repository, source.OriginalFilename, hashes.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("resolve inbox target: %w", err)
	}
	asset := recoverable
	if asset != nil && asset.StoragePath != nil && *asset.StoragePath != "" {
		targetPath = *asset.StoragePath
	}
	if asset == nil {
		asset, err = m.createPreparedAsset(ctx, repository, source, validation, stagingFile, targetPath, hashes)
		if err != nil {
			return nil, err
		}
	}

	observation, exists, err := m.inspectFinal(ctx, repository, targetPath, hashes.FileSize, hashes.ContentHash)
	if err != nil {
		m.markStagingConflict(ctx, repository, asset.AssetID, stagingFile.PrivatePath, err)
		return nil, err
	}
	if !exists {
		commitErr := m.stagingManager.CommitStagingFile(repository, stagingFile, targetPath)
		if commitErr != nil {
			verifiedObservation, committed, verifyErr := m.inspectFinal(ctx, repository, targetPath, hashes.FileSize, hashes.ContentHash)
			if committed && verifyErr == nil {
				observation = verifiedObservation
				_ = m.stagingManager.RemoveStagingFile(repository, stagingFile)
			} else {
				m.handleStagingFailure(ctx, repository, stagingFile, asset.AssetID, commitErr)
				return nil, fmt.Errorf("commit staging file: %w", errors.Join(commitErr, verifyErr))
			}
		} else {
			observation, exists, err = m.inspectFinal(ctx, repository, targetPath, hashes.FileSize, hashes.ContentHash)
			if err != nil || !exists {
				return nil, fmt.Errorf("verify committed file: %w", err)
			}
		}
	} else if err := m.stagingManager.RemoveStagingFile(repository, stagingFile); err != nil {
		return nil, fmt.Errorf("remove verified duplicate staging file: %w", err)
	}

	finalized, err := m.finalizeAssetAndEnqueue(ctx, repository, asset, targetPath, validation.AssetType, observation)
	if err != nil {
		return nil, err
	}
	m.audit(repository.Path).Operation("asset.materialize.staging",
		zap.String("repository_id", repository.RepoID.String()),
		zap.String("asset_id", finalized.AssetID.String()),
		zap.String("storage_path", targetPath),
		zap.String("source_kind", string(source.Kind)),
	)
	return finalized, nil
}

func stagingHandle(repositoryID uuid.UUID, source IngestSource) (*storage.StagingFile, error) {
	privatePath, err := storage.ParsePrivateRepositoryPath(source.StagingPath)
	if err != nil {
		return nil, err
	}
	return &storage.StagingFile{
		ID:           path.Base(privatePath.String()),
		RepositoryID: repositoryID,
		PrivatePath:  privatePath.String(),
		Filename:     path.Base(strings.ReplaceAll(source.OriginalFilename, "\\", "/")),
		CreatedAt:    source.Timestamp,
	}, nil
}

func (m *SourceMaterializer) createPreparedAsset(
	ctx context.Context,
	repository repo.Repository,
	source IngestSource,
	validation *file.ValidationResult,
	stagingFile *storage.StagingFile,
	targetPath string,
	hashes *hash.LayeredHashResult,
) (*repo.Asset, error) {
	statusJSON, err := buildTrackedIngestStatus(validation.AssetType, preparedStatusMessage,
		statusdb.IngestPhasePrepared, "", stagingFile.PrivatePath, true)
	if err != nil {
		return nil, err
	}
	target := targetPath
	created, err := m.createAssetWithMediaItem(ctx, repo.CreateAssetParams{
		OwnerID:                 ownerOrRepoDefault(source.OwnerID, repository.DefaultOwnerID),
		Type:                    string(validation.AssetType),
		OriginalFilename:        source.OriginalFilename,
		StoragePath:             &target,
		MimeType:                validation.MimeType,
		FileSize:                hashes.FileSize,
		ContentHash:             hashes.ContentHash,
		QuickFingerprint:        hashes.QuickFingerprint,
		QuickFingerprintVersion: hashes.QuickFingerprintVersion,
		TakenTime:               dbtypes.NewTimestamp(time.Now().UTC()),
		Rating:                  int64Ptr(0),
		RepositoryID:            uuid.NullUUID{UUID: repository.RepoID, Valid: true},
		Status:                  statusJSON,
	}, repo.InitialMediaRelation(validation, source.OriginalFilename))
	if err != nil {
		return nil, fmt.Errorf("create prepared asset: %w", err)
	}
	return created, nil
}

func (m *SourceMaterializer) inspectFinal(ctx context.Context, repository repo.Repository, storagePath string, expectedSize int64, expectedHash string) (storage.FileObservation, bool, error) {
	repositoryPath, err := storage.ParseUserMediaPath(storagePath)
	if err != nil {
		return storage.FileObservation{}, false, err
	}
	repositoryFS, err := m.files.Open(repository)
	if err != nil {
		return storage.FileObservation{}, false, err
	}
	defer repositoryFS.Close()
	if _, err := repositoryFS.StatMedia(repositoryPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return storage.FileObservation{}, false, nil
		}
		return storage.FileObservation{}, false, err
	}
	mode := storage.HashFull
	if expectedSize > hash.QuickHashThreshold {
		mode = storage.HashQuickAndFull
	}
	observation, err := repositoryFS.InspectMedia(ctx, repositoryPath, mode)
	if err != nil {
		return storage.FileObservation{}, true, err
	}
	if observation.Size != expectedSize {
		return storage.FileObservation{}, true, fmt.Errorf("final file size conflict: got %d, want %d", observation.Size, expectedSize)
	}
	if observation.ContentHash == nil || !strings.EqualFold(*observation.ContentHash, expectedHash) {
		return storage.FileObservation{}, true, errors.New("final file content conflict: BLAKE3 mismatch")
	}
	if err := repositoryFS.Revalidate(ctx, observation); err != nil {
		return storage.FileObservation{}, true, err
	}
	return observation, true, nil
}

func (m *SourceMaterializer) verifyAssetFile(ctx context.Context, repository repo.Repository, asset *repo.Asset, expectedHash string, expectedSize int64) (bool, error) {
	if asset == nil || asset.IsDeleted || asset.StoragePath == nil || *asset.StoragePath == "" {
		return false, nil
	}
	_, exists, err := m.inspectFinal(ctx, repository, *asset.StoragePath, expectedSize, expectedHash)
	return exists && err == nil, err
}

func (m *SourceMaterializer) recoverPreparedIngest(ctx context.Context, repository repo.Repository, source IngestSource, assetType dbtypes.AssetType) (*repo.Asset, error) {
	if source.ContentHash == nil || !hash.ValidateHash(strings.TrimSpace(*source.ContentHash), hash.AlgorithmBLAKE3) {
		return nil, errors.New("staged file is missing and no authoritative content hash is available")
	}
	expectedHash := strings.ToLower(strings.TrimSpace(*source.ContentHash))
	recoverable, duplicate, err := m.findContentCandidates(ctx, repository.RepoID, expectedHash, 0, source.StagingPath)
	if err != nil {
		return nil, err
	}
	if recoverable == nil {
		if duplicate != nil {
			verified, verifyErr := m.verifyAssetFile(ctx, repository, duplicate, expectedHash, duplicate.FileSize)
			if verifyErr != nil {
				return nil, verifyErr
			}
			if verified {
				return duplicate, nil
			}
		}
		return nil, errors.New("staged file is missing and no prepared asset exists")
	}
	if recoveryPath := recoverableStagingPath(recoverable.Status); recoveryPath != "" && recoveryPath != source.StagingPath {
		source.StagingPath = recoveryPath
		if opened, openErr := m.stagingManager.OpenStagingFile(repository, &storage.StagingFile{
			ID: recoveryPath, RepositoryID: repository.RepoID, PrivatePath: recoveryPath, Filename: source.OriginalFilename,
		}); openErr == nil {
			_ = opened.Close()
			return m.MaterializeStaged(ctx, source)
		}
	}
	if recoverable.StoragePath == nil || *recoverable.StoragePath == "" {
		return nil, errors.New("prepared asset has no inbox target")
	}
	observation, exists, err := m.inspectFinal(ctx, repository, *recoverable.StoragePath, recoverable.FileSize, recoverable.ContentHash)
	if err != nil || !exists {
		return nil, fmt.Errorf("prepared asset has neither staging nor a verified inbox file: %w", err)
	}
	return m.finalizeAssetAndEnqueue(ctx, repository, recoverable, *recoverable.StoragePath, assetType, observation)
}

func (m *SourceMaterializer) findContentCandidates(ctx context.Context, repositoryID uuid.UUID, contentHash string, fileSize int64, stagingPath string) (*repo.Asset, *repo.Asset, error) {
	rows, err := m.queries.GetAssetsByContentHashesAndRepository(ctx, repo.GetAssetsByContentHashesAndRepositoryParams{
		ContentHashes: dbtypes.StringsJSONParam([]string{contentHash}),
		RepositoryID:  uuid.NullUUID{UUID: repositoryID, Valid: true},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("find existing staged content: %w", err)
	}
	var recoverable, duplicate *repo.Asset
	for _, row := range rows {
		if fileSize > 0 && row.FileSize != fileSize {
			continue
		}
		asset, err := m.queries.GetAssetByID(ctx, row.AssetID)
		if err != nil {
			return nil, nil, err
		}
		if isRecoverableStagingAsset(&asset) {
			if recoverable == nil || recoverableStagingPath(asset.Status) == stagingPath {
				copy := asset
				recoverable = &copy
			}
			if recoverableStagingPath(asset.Status) == stagingPath {
				continue
			}
		}
		if !asset.IsDeleted && duplicate == nil {
			copy := asset
			duplicate = &copy
		}
	}
	return recoverable, duplicate, nil
}

func (m *SourceMaterializer) createAssetWithMediaItem(ctx context.Context, params repo.CreateAssetParams, relation repo.StackRelation) (*repo.Asset, error) {
	var created *repo.Asset
	err := m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		var err error
		created, err = createAssetWithMediaItem(ctx, queries, params, relation)
		if err != nil || created.OwnerID == nil {
			return err
		}
		capturedAt := created.UploadTime.Time.UTC().UnixMicro()
		now := time.Now().UTC().UnixMicro()
		if _, err := tx.ExecContext(ctx, `
INSERT INTO event_dirty_ranges(
 dirty_range_id,owner_id,range_start,range_end,reason,created_at
) VALUES(?,?,?,?,?,?)`, uuid.NewString(), *created.OwnerID, capturedAt, capturedAt, "media_item_created", now); err != nil {
			return fmt.Errorf("mark Event range dirty: %w", err)
		}
		args := jobs.EventRebuildArgs{OwnerID: *created.OwnerID}
		opts := args.InsertOpts()
		_, err = m.queueClient.InsertTx(ctx, tx, args, &opts)
		return err
	})
	return created, err
}

func createAssetWithMediaItem(ctx context.Context, queries *repo.Queries, params repo.CreateAssetParams, relation repo.StackRelation) (*repo.Asset, error) {
	if params.AssetID == uuid.Nil {
		params.AssetID = uuid.New()
	}
	asset, err := queries.CreateAsset(ctx, params)
	if err != nil {
		return nil, err
	}
	mediaItemID := uuid.New()
	createdAt := dbtypes.NewTimestamp(time.Now().UTC())
	if err := queries.CreateMediaItemForAsset(ctx, repo.CreateMediaItemForAssetParams{
		MediaItemID: mediaItemID, OwnerID: asset.OwnerID, RepositoryID: asset.RepositoryID,
		MediaKind: strings.ToLower(asset.Type), AssetID: uuid.NullUUID{UUID: asset.AssetID, Valid: true}, CreatedAt: createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create logical media item: %w", err)
	}
	if err := queries.AttachAssetToMediaItem(ctx, repo.AttachAssetToMediaItemParams{
		AssetID: asset.AssetID, MediaItemID: mediaItemID, Relation: string(relation), CreatedAt: createdAt,
	}); err != nil {
		return nil, fmt.Errorf("attach asset to logical media item: %w", err)
	}
	return &asset, nil
}

func (m *SourceMaterializer) finalizeAssetAndEnqueue(ctx context.Context, repository repo.Repository, asset *repo.Asset, storagePath string, assetType dbtypes.AssetType, observation storage.FileObservation) (*repo.Asset, error) {
	queuedStatus, err := buildTrackedIngestStatus(assetType, queuedStatusMessage, statusdb.IngestPhasePipelineQueued, "", "", false)
	if err != nil {
		return nil, err
	}
	var finalized repo.Asset
	err = m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		finalized, err = queries.UpdateAssetStoragePathAndStatus(ctx, repo.UpdateAssetStoragePathAndStatusParams{
			AssetID: asset.AssetID, StoragePath: &storagePath, Status: queuedStatus,
		})
		if err != nil {
			return err
		}
		if _, err := queries.UpsertRepositoryFileObservation(ctx, repo.UpsertRepositoryFileObservationParams{
			RepositoryID: repository.RepoID, StoragePath: storagePath,
			AssetID: uuid.NullUUID{UUID: asset.AssetID, Valid: true}, EntryKind: string(observation.EntryKind),
			FileSize: observation.Size, ModifiedAtNs: observation.ModTimeNS, ChangedAtNs: observation.ChangeTimeNS,
			FileIdentityKind: observation.FileIdentityKind, FileIdentityValue: observation.FileIdentity,
			ObservationToken: observation.ObservationToken, QuickFingerprint: observation.QuickFingerprint,
			QuickFingerprintVersion: observation.QuickFingerprintVer, ContentHash: observation.ContentHash,
			State: "present", UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		}); err != nil {
			return err
		}
		return m.enqueuePipelineTx(ctx, tx, &finalized, assetType, observation.ObservationToken)
	})
	if err != nil {
		return nil, fmt.Errorf("finalize asset, file index, and pipeline: %w", err)
	}
	return &finalized, nil
}

func (m *SourceMaterializer) handleStagingFailure(ctx context.Context, repository repo.Repository, stagingFile *storage.StagingFile, assetID uuid.UUID, commitErr error) {
	detail := fmt.Sprintf("commit staging to inbox failed: %v", commitErr)
	if moveErr := m.stagingManager.MoveStagingToFailed(repository, stagingFile); moveErr != nil {
		detail += fmt.Sprintf("; move to failed dir failed: %v", moveErr)
		m.logger.Warn("failed to quarantine staging file", zap.String("staging_path", stagingFile.PrivatePath), zap.Error(moveErr))
	}
	if err := m.markAssetIngestFailure(ctx, assetID, statusdb.IngestPhaseCommitFailed, ingestCodeCommitFailed, stagingFile.PrivatePath, detail); err != nil {
		m.logger.Warn("failed to mark recoverable ingest", zap.String("asset_id", assetID.String()), zap.Error(err))
	}
	m.audit(repository.Path).Error("asset.materialize.commit_staging", commitErr, zap.String("asset_id", assetID.String()))
}

func (m *SourceMaterializer) markStagingConflict(ctx context.Context, repository repo.Repository, assetID uuid.UUID, stagingPath string, conflictErr error) {
	if err := m.markAssetIngestFailure(ctx, assetID, statusdb.IngestPhaseConflict, ingestCodeConflict, stagingPath, conflictErr.Error()); err != nil {
		m.logger.Warn("failed to mark staging conflict", zap.String("asset_id", assetID.String()), zap.Error(err))
	}
	m.audit(repository.Path).Error("asset.materialize.conflict", conflictErr, zap.String("asset_id", assetID.String()))
}

func (m *SourceMaterializer) markAssetIngestFailure(ctx context.Context, assetID uuid.UUID, phase statusdb.IngestPhase, code, stagingPath, detail string) error {
	return m.queries.MutateAssetStatus(ctx, assetID, func(current statusdb.AssetStatus) (statusdb.AssetStatus, error) {
		current.State = statusdb.StateFailed
		current.Message = "Asset ingestion failed"
		current.AddError("materialize_asset", detail)
		current.SetIngestState(phase, code, stagingPath, true)
		return current, nil
	})
}

func isRecoverableStagingAsset(asset *repo.Asset) bool {
	if asset == nil {
		return false
	}
	status, err := statusdb.FromJSON(asset.Status)
	if err != nil || status.Ingest == nil || !status.Ingest.Recoverable {
		return false
	}
	switch status.Ingest.Phase {
	case statusdb.IngestPhasePrepared, statusdb.IngestPhaseCommitFailed, statusdb.IngestPhaseConflict:
		return true
	default:
		return false
	}
}

func recoverableStagingPath(rawStatus []byte) string {
	status, err := statusdb.FromJSON(rawStatus)
	if err != nil || status.Ingest == nil || !status.Ingest.Recoverable {
		return ""
	}
	parsed, err := storage.ParsePrivateRepositoryPath(status.Ingest.StagingPath)
	if err != nil {
		return ""
	}
	return parsed.String()
}

func (m *SourceMaterializer) resolveRepository(ctx context.Context, repositoryID uuid.UUID) (repo.Repository, error) {
	if repositoryID != uuid.Nil {
		return m.queries.GetRepository(ctx, repositoryID)
	}
	repository, err := m.queries.GetPrimaryRepository(ctx)
	if err != nil {
		return repo.Repository{}, fmt.Errorf("no repository available: %w", err)
	}
	return repository, nil
}

// enqueuePipelineTx is kept transaction-local; payloads are reduced to stable
// asset identity in the downstream-job migration slice.

func (m *SourceMaterializer) enqueuePipelineTx(ctx context.Context, tx *sql.Tx, asset *repo.Asset, assetType dbtypes.AssetType, observationToken string) error {
	metadata := jobs.MetadataArgs{AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash}
	thumbnail := jobs.ThumbnailArgs{AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash}
	transcode := jobs.TranscodeArgs{AssetID: asset.AssetID, ObservationToken: observationToken, ExpectedContentHash: asset.ContentHash}
	if _, err := m.queueClient.InsertTx(ctx, tx, metadata, &river.InsertOpts{Queue: "metadata_asset"}); err != nil {
		return fmt.Errorf("enqueue metadata: %w", err)
	}
	switch assetType {
	case dbtypes.AssetTypePhoto:
		_, err := m.queueClient.InsertTx(ctx, tx, thumbnail, &river.InsertOpts{Queue: "thumbnail_asset"})
		return err
	case dbtypes.AssetTypeVideo:
		if _, err := m.queueClient.InsertTx(ctx, tx, thumbnail, &river.InsertOpts{Queue: "thumbnail_asset"}); err != nil {
			return err
		}
		_, err := m.queueClient.InsertTx(ctx, tx, transcode, &river.InsertOpts{Queue: "transcode_asset"})
		return err
	case dbtypes.AssetTypeAudio:
		_, err := m.queueClient.InsertTx(ctx, tx, transcode, &river.InsertOpts{Queue: "transcode_asset"})
		return err
	default:
		return fmt.Errorf("unsupported asset type: %s", assetType)
	}
}

func (m *SourceMaterializer) audit(repositoryPath string) logging.RepositoryAuditLogger {
	return m.auditProvider.ForPath(repositoryPath)
}

func PipelineTaskNames(assetType dbtypes.AssetType) []string { return pipelineTaskNames(assetType) }

func pipelineTaskNames(assetType dbtypes.AssetType) []string {
	switch assetType {
	case dbtypes.AssetTypePhoto:
		return []string{TaskMetadata, TaskThumbnail}
	case dbtypes.AssetTypeVideo:
		return []string{TaskMetadata, TaskThumbnail, TaskTranscode}
	case dbtypes.AssetTypeAudio:
		return []string{TaskMetadata, TaskTranscode}
	default:
		return []string{TaskMetadata}
	}
}

func BuildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	return buildTrackedProcessingStatus(assetType, message)
}

func buildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	return statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType)).ToJSON()
}

func buildTrackedIngestStatus(assetType dbtypes.AssetType, message string, phase statusdb.IngestPhase, code, stagingPath string, recoverable bool) ([]byte, error) {
	status := statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType))
	status.SetIngestState(phase, code, stagingPath, recoverable)
	return status.ToJSON()
}

func ownerOrRepoDefault(owner, repositoryDefault *int32) *int32 {
	if owner != nil {
		return owner
	}
	return repositoryDefault
}

func int64Ptr(value int64) *int64 { return &value }
