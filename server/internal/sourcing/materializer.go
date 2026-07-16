package sourcing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils/file"
	"server/internal/utils/hash"
)

// Pipeline task name constants shared across the ingest pipeline.
const (
	TaskMetadata  = "metadata_asset"
	TaskThumbnail = "thumbnail_asset"
	TaskTranscode = "transcode_asset"
)

// SourceMaterializer validates an IngestSource, materializes the file into the
// repository (staging→inbox for upload/cloud, or in-place registration for scan),
// creates/updates the asset DB record, and enqueues downstream pipeline tasks.
type SourceMaterializer struct {
	queries        *repo.Queries
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
	assetService   service.AssetService
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	contentLocks   [256]sync.Mutex
}

// NewSourceMaterializer creates a SourceMaterializer with the required dependencies.
func NewSourceMaterializer(
	queries *repo.Queries,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	assetService service.AssetService,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
) *SourceMaterializer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger, false)
	}
	return &SourceMaterializer{
		queries:        queries,
		stagingManager: stagingManager,
		queueClient:    queueClient,
		assetService:   assetService,
		logger:         logger.With(zap.String("component", "source_materializer")),
		auditProvider:  auditProvider,
	}
}

// Materialize processes an IngestSource through validation, file materialization,
// DB record creation/update, and pipeline enqueuing.
//
// Returns nil asset with nil error when the asset is unchanged (scan skip) or
// the source file has disappeared.
func (m *SourceMaterializer) Materialize(ctx context.Context, source IngestSource) (*repo.Asset, error) {
	// 1. Validate file type
	validation := file.ValidateFile(source.OriginalFilename, source.ContentType)
	if !validation.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorReason)
	}

	// 2. Resolve repository
	repository, err := m.resolveRepository(ctx, source.RepositoryID)
	if err != nil {
		return nil, err
	}

	// 3. Branch on source kind
	switch source.Kind {
	case IngestSourceUpload:
		return m.materializeFromStaging(ctx, source, repository, validation)
	case IngestSourceCloud:
		if source.SkipCommit {
			return m.materializeInPlace(ctx, source, repository, validation)
		}
		return m.materializeFromStaging(ctx, source, repository, validation)
	case IngestSourceScan:
		return m.materializeInPlace(ctx, source, repository, validation)
	default:
		return nil, fmt.Errorf("unsupported ingest source kind: %s", source.Kind)
	}
}

// ---------------------------------------------------------------------------
// Staging path (upload / cloud) — file is in .lumilio/staging/incoming/
// and must be committed to the inbox.
// ---------------------------------------------------------------------------

func (m *SourceMaterializer) materializeFromStaging(
	ctx context.Context,
	source IngestSource,
	repository repo.Repository,
	validation *file.ValidationResult,
) (*repo.Asset, error) {
	// Stat staging file for authoritative size
	info, err := os.Stat(source.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("staged file not found: %w", err)
	}
	fileSize := info.Size()

	// Upload handlers pass a server-verified full hash; non-HTTP staging sources
	// are hashed here. Client fingerprints never populate ContentHash.
	hashes, err := m.resolveLayeredHash(source)
	if err != nil {
		return nil, fmt.Errorf("calculate layered hash: %w", err)
	}
	lockIndex, _ := strconv.ParseUint(hashes.ContentHash[:2], 16, 8)
	m.contentLocks[lockIndex].Lock()
	defer m.contentLocks[lockIndex].Unlock()

	existing, err := m.findExistingContent(ctx, repository.RepoID, hashes.ContentHash, fileSize)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if removeErr := os.Remove(source.SourcePath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("remove duplicate staging file: %w", removeErr)
		}
		return existing, nil
	}

	// Build staging file handle
	stagingFile := &storage.StagingFile{
		ID:        filepath.Base(source.SourcePath),
		RepoPath:  repository.Path,
		Path:      source.SourcePath,
		Filename:  source.OriginalFilename,
		CreatedAt: source.Timestamp,
	}

	// Initial tracked status
	statusJSON, err := buildTrackedProcessingStatus(validation.AssetType, "Asset ingestion started")
	if err != nil {
		return nil, fmt.Errorf("marshal status: %w", err)
	}

	// Resolve owner: explicit OwnerID, otherwise repository default
	ownerID := source.OwnerID
	if ownerID == nil {
		ownerID = repository.DefaultOwnerID
	}

	// Create asset record (storage path not yet known)
	asset, err := m.assetService.CreateAssetRecord(ctx, repo.CreateAssetParams{
		OwnerID:                 ownerID,
		Type:                    string(validation.AssetType),
		OriginalFilename:        source.OriginalFilename,
		StoragePath:             nil,
		MimeType:                validation.MimeType,
		FileSize:                fileSize,
		ContentHash:             hashes.ContentHash,
		QuickFingerprint:        hashes.QuickFingerprint,
		QuickFingerprintVersion: hashes.QuickFingerprintVersion,
		TakenTime:               pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Rating:                  int32Ptr(0),
		RepositoryID:            repository.RepoID,
		Status:                  statusJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("create asset: %w", err)
	}

	// Commit staging → inbox; the storage path is determined by the repo's storage strategy
	storageRelPath, err := m.stagingManager.CommitStagingFileToInbox(stagingFile, hashes.ContentHash)
	if err != nil {
		m.handleStagingFailure(ctx, stagingFile, repository.Path, asset.AssetID, err)
		return asset, nil // asset record exists but is in failed state
	}

	// Update asset with the resolved storage path
	_, err = m.queries.UpdateAssetStoragePathAndStatus(ctx, repo.UpdateAssetStoragePathAndStatusParams{
		AssetID:     asset.AssetID,
		StoragePath: &storageRelPath,
		Status:      statusJSON,
	})
	if err != nil {
		m.markPipelineTasksFailed(ctx, asset.AssetID, pipelineTaskNames(validation.AssetType), fmt.Errorf("update asset storage path: %w", err))
		return nil, fmt.Errorf("update asset storage path: %w", err)
	}

	// Enqueue downstream pipeline
	assetType := dbtypes.AssetType(asset.Type)
	if err := m.enqueuePipeline(ctx, repository, asset, storageRelPath, assetType); err != nil {
		return nil, err
	}

	m.audit(repository.Path).Operation("asset.materialize.staging",
		zap.String("repository_id", uuid.UUID(repository.RepoID.Bytes).String()),
		zap.String("asset_id", asset.AssetID.String()),
		zap.String("storage_path", storageRelPath),
		zap.String("asset_type", string(assetType)),
		zap.String("source_kind", string(source.Kind)),
	)

	return asset, nil
}

// ---------------------------------------------------------------------------
// In-place path (scan) — file is already in the user workspace and should be
// registered without moving it.
// ---------------------------------------------------------------------------

func (m *SourceMaterializer) materializeInPlace(
	ctx context.Context,
	source IngestSource,
	repository repo.Repository,
	validation *file.ValidationResult,
) (*repo.Asset, error) {
	repoID := repository.RepoID

	// source.SourcePath must be a repository-relative path
	storagePath := filepath.ToSlash(filepath.Clean(source.SourcePath))
	fullPath := filepath.Join(repository.Path, filepath.FromSlash(storagePath))

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // file disappeared, nothing to do
		}
		return nil, fmt.Errorf("stat discovered file: %w", err)
	}
	if info.IsDir() {
		return nil, nil
	}

	// Compute hash
	hashResult, err := hash.CalculateLayeredBLAKE3(fullPath)
	if err != nil {
		return nil, fmt.Errorf("calculate hash: %w", err)
	}

	statusJSON, err := buildTrackedProcessingStatus(validation.AssetType, "Asset discovery ingestion started")
	if err != nil {
		return nil, fmt.Errorf("marshal status: %w", err)
	}

	// Check if an asset already exists at this path
	existing, existingErr := m.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
		RepositoryID: repoID,
		StoragePath:  &storagePath,
	})
	if existingErr != nil && !errors.Is(existingErr, pgx.ErrNoRows) {
		return nil, fmt.Errorf("find discovered asset by path: %w", existingErr)
	}

	assetType := validation.AssetType

	// Existing asset — skip if unchanged, otherwise update
	if existingErr == nil {
		if !isSoftDeleted(existing) &&
			existing.ContentHash == hashResult.ContentHash &&
			existing.FileSize == info.Size() &&
			strings.EqualFold(existing.MimeType, validation.MimeType) {
			return nil, nil // unchanged
		}

		updated, updateErr := m.queries.UpdateDiscoveredAssetByID(ctx, repo.UpdateDiscoveredAssetByIDParams{
			AssetID:                 existing.AssetID,
			OriginalFilename:        source.OriginalFilename,
			MimeType:                validation.MimeType,
			FileSize:                info.Size(),
			ContentHash:             hashResult.ContentHash,
			QuickFingerprint:        hashResult.QuickFingerprint,
			QuickFingerprintVersion: hashResult.QuickFingerprintVersion,
			TakenTime:               pgtype.Timestamptz{Time: info.ModTime().UTC(), Valid: true},
			Status:                  statusJSON,
		})
		if updateErr != nil {
			return nil, fmt.Errorf("update discovered asset: %w", updateErr)
		}
		asset := updated

		if err := m.enqueuePipeline(ctx, repository, &asset, storagePath, assetType); err != nil {
			return nil, err
		}

		m.audit(repository.Path).Operation("asset.materialize.inplace_update",
			zap.String("repository_id", uuid.UUID(repository.RepoID.Bytes).String()),
			zap.String("asset_id", asset.AssetID.String()),
			zap.String("storage_path", storagePath),
		)
		return &asset, nil
	}

	// New asset — create
	storagePathPtr := storagePath
	created, createErr := m.assetService.CreateAssetRecord(ctx, repo.CreateAssetParams{
		OwnerID:                 ownerOrRepoDefault(source.OwnerID, repository.DefaultOwnerID),
		Type:                    string(assetType),
		OriginalFilename:        source.OriginalFilename,
		StoragePath:             &storagePathPtr,
		MimeType:                validation.MimeType,
		FileSize:                info.Size(),
		ContentHash:             hashResult.ContentHash,
		QuickFingerprint:        hashResult.QuickFingerprint,
		QuickFingerprintVersion: hashResult.QuickFingerprintVersion,
		TakenTime:               pgtype.Timestamptz{Time: info.ModTime().UTC(), Valid: true},
		Rating:                  int32Ptr(0),
		RepositoryID:            repoID,
		Status:                  statusJSON,
	})
	if createErr != nil {
		// Race: another worker may have created the same asset between our lookup and insert
		if isUniqueConstraintViolation(createErr) {
			latest, fetchErr := m.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
				RepositoryID: repoID,
				StoragePath:  &storagePath,
			})
			if fetchErr != nil && !errors.Is(fetchErr, pgx.ErrNoRows) {
				return nil, fmt.Errorf("fetch discovered asset after unique conflict: %w", fetchErr)
			}
			if fetchErr == nil {
				created = &latest
			} else {
				return nil, nil
			}
		} else {
			return nil, fmt.Errorf("create discovered asset: %w", createErr)
		}
	}
	if created == nil {
		return nil, nil
	}
	asset := created

	if err := m.enqueuePipeline(ctx, repository, asset, storagePath, assetType); err != nil {
		return nil, err
	}

	m.audit(repository.Path).Operation("asset.materialize.inplace_create",
		zap.String("repository_id", uuid.UUID(repository.RepoID.Bytes).String()),
		zap.String("asset_id", asset.AssetID.String()),
		zap.String("storage_path", storagePath),
		zap.String("asset_type", string(assetType)),
		zap.String("source_kind", string(source.Kind)),
	)

	return asset, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m *SourceMaterializer) findExistingContent(ctx context.Context, repositoryID pgtype.UUID, contentHash string, fileSize int64) (*repo.Asset, error) {
	rows, err := m.queries.GetAssetsByContentHashesAndRepository(ctx, repo.GetAssetsByContentHashesAndRepositoryParams{
		ContentHashes: []string{contentHash},
		RepositoryID:  repositoryID,
	})
	if err != nil {
		return nil, fmt.Errorf("find existing staged content: %w", err)
	}
	for _, row := range rows {
		if row.FileSize != fileSize {
			continue
		}
		asset, err := m.queries.GetAssetByID(ctx, row.AssetID)
		if err != nil {
			return nil, fmt.Errorf("load existing staged content: %w", err)
		}
		return &asset, nil
	}
	return nil, nil
}

func (m *SourceMaterializer) resolveLayeredHash(source IngestSource) (*hash.LayeredHashResult, error) {
	info, err := os.Stat(source.SourcePath)
	if err != nil {
		return nil, fmt.Errorf("stat source for hashing: %w", err)
	}
	if source.ContentHash == nil || !hash.ValidateHash(strings.TrimSpace(*source.ContentHash), hash.AlgorithmBLAKE3) {
		return hash.CalculateLayeredBLAKE3(source.SourcePath)
	}
	result := &hash.LayeredHashResult{
		ContentHash: strings.ToLower(strings.TrimSpace(*source.ContentHash)),
		FileSize:    info.Size(),
	}
	if info.Size() > hash.QuickHashThreshold && source.QuickFingerprint != nil && source.QuickFingerprintVersion != nil &&
		*source.QuickFingerprintVersion == hash.QuickFingerprintVersion &&
		hash.ValidateHash(strings.TrimSpace(*source.QuickFingerprint), hash.AlgorithmBLAKE3) {
		quick := strings.ToLower(strings.TrimSpace(*source.QuickFingerprint))
		version := hash.QuickFingerprintVersion
		result.QuickFingerprint = &quick
		result.QuickFingerprintVersion = &version
	}
	return result, nil
}

// handleStagingFailure attempts to move a failed staging file to the failed
// directory and marks the asset record as failed.
func (m *SourceMaterializer) handleStagingFailure(
	ctx context.Context,
	stagingFile *storage.StagingFile,
	repoPath string,
	assetID pgtype.UUID,
	commitErr error,
) {
	failureDetail := fmt.Sprintf("commit staging to inbox failed: %v", commitErr)

	if moveErr := m.stagingManager.MoveStagingToFailed(stagingFile); moveErr != nil {
		m.logger.Warn("failed to move staging file to failed dir",
			zap.String("operation", "source.materialize"),
			zap.String("staging_path", stagingFile.Path),
			zap.Error(moveErr),
		)
		m.audit(repoPath).Error("asset.materialize.move_failed", moveErr,
			zap.String("asset_id", assetID.String()),
			zap.String("staging_path", stagingFile.Path),
		)
		if removeErr := os.Remove(stagingFile.Path); removeErr != nil && !os.IsNotExist(removeErr) {
			m.logger.Warn("failed to remove staging file after move failure",
				zap.String("operation", "source.materialize"),
				zap.String("staging_path", stagingFile.Path),
				zap.Error(removeErr),
			)
		}
		failureDetail = fmt.Sprintf("%s; move to failed dir failed: %v", failureDetail, moveErr)
	}

	if markErr := m.markAssetFailed(ctx, assetID, "commit_staging", failureDetail); markErr != nil {
		m.logger.Warn("failed to mark asset as failed after staging commit error",
			zap.String("operation", "source.materialize"),
			zap.String("asset_id", assetID.String()),
			zap.Error(markErr),
		)
	}
	m.audit(repoPath).Error("asset.materialize.commit_staging", commitErr,
		zap.String("asset_id", assetID.String()),
		zap.String("original_filename", stagingFile.Filename),
	)
}

// resolveRepository looks up a repository by UUID, falling back to primary.
func (m *SourceMaterializer) resolveRepository(ctx context.Context, repoUUID uuid.UUID) (repo.Repository, error) {
	if repoUUID != uuid.Nil {
		repoUUIDPg := pgtype.UUID{Bytes: repoUUID, Valid: true}
		return m.queries.GetRepository(ctx, repoUUIDPg)
	}

	repository, err := m.queries.GetPrimaryRepository(ctx)
	if err != nil {
		return repo.Repository{}, fmt.Errorf("no repository available: %w", err)
	}
	return repository, nil
}

// enqueuePipeline inserts downstream River jobs for the asset pipeline.
func (m *SourceMaterializer) enqueuePipeline(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	storagePath string,
	assetType dbtypes.AssetType,
) error {
	pgID := asset.AssetID
	commonMeta := jobs.MetadataArgs{
		AssetID:          pgID,
		RepoPath:         repository.Path,
		StoragePath:      storagePath,
		AssetType:        assetType,
		OriginalFilename: asset.OriginalFilename,
		FileSize:         asset.FileSize,
		MimeType:         asset.MimeType,
	}
	commonThumb := jobs.ThumbnailArgs{
		AssetID:     pgID,
		RepoPath:    repository.Path,
		StoragePath: storagePath,
		AssetType:   assetType,
	}
	commonTranscode := jobs.TranscodeArgs{
		AssetID:     pgID,
		RepoPath:    repository.Path,
		StoragePath: storagePath,
		AssetType:   assetType,
	}

	// Metadata is always first
	_, err := m.queueClient.Insert(ctx, commonMeta, &river.InsertOpts{Queue: "metadata_asset"})
	if err != nil {
		m.markPipelineTasksFailed(ctx, asset.AssetID, pipelineTaskNames(assetType), fmt.Errorf("enqueue metadata: %w", err))
		return fmt.Errorf("enqueue metadata: %w", err)
	}

	switch assetType {
	case dbtypes.AssetTypePhoto:
		_, err = m.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			m.markPipelineTasksFailed(ctx, asset.AssetID, []string{TaskThumbnail}, fmt.Errorf("enqueue thumbnails: %w", err))
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}

	case dbtypes.AssetTypeVideo:
		_, err = m.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			m.markPipelineTasksFailed(ctx, asset.AssetID, []string{TaskThumbnail, TaskTranscode}, fmt.Errorf("enqueue thumbnails: %w", err))
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}
		_, err = m.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			m.markPipelineTasksFailed(ctx, asset.AssetID, []string{TaskTranscode}, fmt.Errorf("enqueue transcode: %w", err))
			return fmt.Errorf("enqueue transcode: %w", err)
		}

	case dbtypes.AssetTypeAudio:
		_, err = m.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			m.markPipelineTasksFailed(ctx, asset.AssetID, []string{TaskTranscode}, fmt.Errorf("enqueue transcode: %w", err))
			return fmt.Errorf("enqueue transcode: %w", err)
		}

	default:
		return fmt.Errorf("unsupported asset type: %s", assetType)
	}

	return nil
}

// markAssetFailed updates the asset status to failed with a single error detail.
func (m *SourceMaterializer) markAssetFailed(ctx context.Context, assetID pgtype.UUID, taskName string, detail string) error {
	failedStatus := statusdb.NewFailedStatus("Asset ingestion failed", []statusdb.ErrorDetail{
		{
			Task:  taskName,
			Error: detail,
			Time:  time.Now().Format(time.RFC3339),
		},
	})
	statusJSON, err := failedStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("marshal failed status: %w", err)
	}

	_, err = m.queries.UpdateAssetStatusWithErrors(ctx, repo.UpdateAssetStatusWithErrorsParams{
		AssetID: assetID,
		Status:  statusJSON,
	})
	return err
}

// markPipelineTasksFailed marks individual pipeline tasks as failed in the
// asset status before they were ever queued.
func (m *SourceMaterializer) markPipelineTasksFailed(ctx context.Context, assetID pgtype.UUID, tasks []string, cause error) {
	if len(tasks) == 0 {
		return
	}
	detail := "pipeline task failed before it could be queued"
	if cause != nil {
		detail = cause.Error()
	}

	if mutateErr := m.queries.MutateAssetStatus(ctx, assetID, func(current statusdb.AssetStatus) (statusdb.AssetStatus, error) {
		for _, taskName := range tasks {
			current.MarkTaskFailed(taskName, fmt.Sprintf("%s failed before queueing", taskName), detail)
		}
		return current, nil
	}); mutateErr != nil {
		m.logger.Warn("failed to mark pipeline tasks as failed",
			zap.String("asset_id", assetID.String()),
			zap.Error(mutateErr),
		)
	}
}

func (m *SourceMaterializer) audit(repoPath string) logging.RepositoryAuditLogger {
	return m.auditProvider.ForPath(repoPath)
}

// ---------------------------------------------------------------------------
// package-level helpers (shared with callers)
// ---------------------------------------------------------------------------

// PipelineTaskNames returns the ordered list of task names for a given asset type.
func PipelineTaskNames(assetType dbtypes.AssetType) []string {
	return pipelineTaskNames(assetType)
}

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

// BuildTrackedProcessingStatus builds an initial tracked-processing status JSONB blob.
func BuildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	return buildTrackedProcessingStatus(assetType, message)
}

func buildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	s := statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType))
	return s.ToJSONB()
}

func ownerOrRepoDefault(owner *int32, defaultOwner *int32) *int32 {
	if owner != nil {
		return owner
	}
	return defaultOwner
}

func int32Ptr(v int32) *int32 {
	return &v
}

func isSoftDeleted(a repo.Asset) bool {
	return a.IsDeleted != nil && *a.IsDeleted
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
