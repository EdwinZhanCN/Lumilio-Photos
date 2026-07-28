package sourcing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"github.com/riverqueue/river"
	"go.uber.org/zap"

	"server/internal/db"
	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/queue/jobs"
	"server/internal/storage"
	"server/internal/utils/file"
	"server/internal/utils/hash"
)

// Pipeline task name constants shared across the ingest pipeline.
const (
	TaskMetadata  = "metadata_asset"
	TaskThumbnail = "thumbnail_asset"
	TaskTranscode = "transcode_asset"

	preparedStatusMessage = "Asset ingestion prepared"
	queuedStatusMessage   = "Asset queued for processing"

	ingestCodeCommitFailed = "staging_commit_failed"
	ingestCodeConflict     = "final_path_conflict"
)

// SourceMaterializer validates an IngestSource, materializes the file into the
// repository (staging→inbox for upload/cloud, or in-place registration for scan),
// creates/updates the asset DB record, and enqueues downstream pipeline tasks.
type SourceMaterializer struct {
	database       *db.DB
	queries        *repo.Queries
	stagingManager storage.StagingManager
	queueClient    *river.Client[*sql.Tx]
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	contentLocks   [256]sync.Mutex
}

// NewSourceMaterializer creates a SourceMaterializer with the required dependencies.
func NewSourceMaterializer(
	database *db.DB,
	stagingManager storage.StagingManager,
	queueClient *river.Client[*sql.Tx],
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
		database:       database,
		queries:        database.Queries,
		stagingManager: stagingManager,
		queueClient:    queueClient,
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
	// Stat the staging file for authoritative size. If a prior attempt crossed
	// the filesystem boundary and then crashed, the prepared row carries the
	// target path needed to finish the database/River transaction.
	info, err := os.Stat(source.SourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return m.recoverMovedStaging(ctx, source, repository, validation.AssetType)
		}
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
	if existing != nil && !isRecoverableStagingAsset(existing) {
		return m.reconcileDuplicateStaging(ctx, source, repository, existing, hashes.ContentHash, fileSize)
	}

	targetPath, err := m.stagingManager.ResolveInboxPath(repository.Path, source.OriginalFilename, hashes.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("resolve inbox target: %w", err)
	}
	if existing != nil && existing.StoragePath != nil && *existing.StoragePath != "" {
		targetPath = *existing.StoragePath
	}

	stagingFile := &storage.StagingFile{
		ID:        filepath.Base(source.SourcePath),
		RepoPath:  repository.Path,
		Path:      source.SourcePath,
		Filename:  source.OriginalFilename,
		CreatedAt: source.Timestamp,
	}

	stagingPath, err := relativeRepositoryPath(repository.Path, source.SourcePath)
	if err != nil {
		// Cross-filesystem importers may stage outside the repository. The River
		// payload still carries the source path for retries, but status never
		// persists an absolute host path.
		stagingPath = ""
	}
	preparedStatus, err := buildTrackedIngestStatus(
		validation.AssetType,
		preparedStatusMessage,
		statusdb.IngestPhasePrepared,
		"",
		stagingPath,
		true,
	)
	if err != nil {
		return nil, fmt.Errorf("marshal status: %w", err)
	}
	ownerID := source.OwnerID
	if ownerID == nil {
		ownerID = repository.DefaultOwnerID
	}

	asset := existing
	if asset == nil {
		targetPathCopy := targetPath
		params := repo.CreateAssetParams{
			OwnerID:                 ownerID,
			Type:                    string(validation.AssetType),
			OriginalFilename:        source.OriginalFilename,
			StoragePath:             &targetPathCopy,
			MimeType:                validation.MimeType,
			FileSize:                fileSize,
			ContentHash:             hashes.ContentHash,
			QuickFingerprint:        hashes.QuickFingerprint,
			QuickFingerprintVersion: hashes.QuickFingerprintVersion,
			TakenTime:               dbtypes.NewTimestamp(time.Now()),
			Rating:                  int64Ptr(0),
			RepositoryID:            uuid.NullUUID{UUID: repository.RepoID, Valid: true},
			Status:                  preparedStatus,
		}
		created, createErr := m.createAssetWithMediaItem(ctx, params, repo.InitialMediaRelation(validation, source.OriginalFilename))
		if createErr != nil {
			if !isUniqueConstraintViolation(createErr) {
				return nil, fmt.Errorf("create prepared asset: %w", createErr)
			}
			created, createErr = m.findExistingContent(ctx, repository.RepoID, hashes.ContentHash, fileSize)
			if createErr != nil {
				return nil, createErr
			}
			if created == nil {
				return nil, fmt.Errorf("prepared asset conflict has no matching row")
			}
			if !isRecoverableStagingAsset(created) {
				return m.reconcileDuplicateStaging(ctx, source, repository, created, hashes.ContentHash, fileSize)
			}
			if created.StoragePath == nil || *created.StoragePath == "" {
				return nil, fmt.Errorf("prepared asset %s has no inbox target", created.AssetID)
			}
			targetPath = *created.StoragePath
		}
		asset = created
	}

	_, finalPath, err := resolveRepositoryRelativePath(repository.Path, targetPath)
	if err != nil {
		return nil, fmt.Errorf("resolve prepared inbox target: %w", err)
	}
	if _, statErr := os.Stat(finalPath); statErr != nil {
		if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspect prepared inbox target: %w", statErr)
		}
		if err := m.stagingManager.CommitStagingFile(stagingFile, targetPath); err != nil {
			if _, finalErr := os.Stat(finalPath); finalErr == nil {
				if reconcileErr := verifyFinalAndRemoveStaging("", finalPath, hashes.ContentHash, fileSize); reconcileErr != nil {
					m.markStagingConflict(ctx, asset.AssetID, repository.Path, source.SourcePath, reconcileErr)
					return nil, fmt.Errorf("commit staging file: %w", errors.Join(err, reconcileErr))
				}
				// Even when the destination is a verified partial success, the
				// commit contract failed. Preserve/quarantine any remaining
				// source and make the caller retry the durable reconciliation.
				m.handleStagingFailure(ctx, stagingFile, repository.Path, asset.AssetID, err)
				return nil, fmt.Errorf("commit staging file: %w", err)
			} else if !os.IsNotExist(finalErr) {
				return nil, fmt.Errorf("inspect inbox target after commit failure: %w", finalErr)
			} else {
				m.handleStagingFailure(ctx, stagingFile, repository.Path, asset.AssetID, err)
				return nil, fmt.Errorf("commit staging file: %w", err)
			}
		}
	} else if reconcileErr := verifyFinalAndRemoveStaging(source.SourcePath, finalPath, hashes.ContentHash, fileSize); reconcileErr != nil {
		m.markStagingConflict(ctx, asset.AssetID, repository.Path, source.SourcePath, reconcileErr)
		return nil, fmt.Errorf("reconcile existing inbox target: %w", reconcileErr)
	}

	assetType := dbtypes.AssetType(asset.Type)
	finalized, err := m.finalizeAssetAndEnqueue(ctx, repository, asset, targetPath, assetType)
	if err != nil {
		m.markPipelineTasksFailed(ctx, asset.AssetID, pipelineTaskNames(assetType), err)
		return nil, err
	}

	m.audit(repository.Path).Operation("asset.materialize.staging",
		zap.String("repository_id", repository.RepoID.String()),
		zap.String("asset_id", asset.AssetID.String()),
		zap.String("storage_path", targetPath),
		zap.String("asset_type", string(assetType)),
		zap.String("source_kind", string(source.Kind)),
	)

	return finalized, nil
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

	// source.SourcePath must resolve to an existing file within the repository.
	// Symlinks are accepted only when their resolved target also stays inside it.
	storagePath, fullPath, err := resolveInPlaceSource(repository.Path, source.SourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

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

	statusJSON, err := buildTrackedIngestStatus(
		validation.AssetType,
		"Asset discovery queued for processing",
		statusdb.IngestPhaseInPlaceQueued,
		"",
		"",
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("marshal status: %w", err)
	}

	// Check if an asset already exists at this path
	existing, existingErr := m.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
		RepositoryID: uuid.NullUUID{UUID: repoID, Valid: true},
		StoragePath:  &storagePath,
	})
	if existingErr != nil && !errors.Is(existingErr, sql.ErrNoRows) {
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

		var updated repo.Asset
		updateErr := m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
			var err error
			updated, err = queries.UpdateDiscoveredAssetByID(ctx, repo.UpdateDiscoveredAssetByIDParams{
				AssetID:                 existing.AssetID,
				OriginalFilename:        source.OriginalFilename,
				MimeType:                validation.MimeType,
				FileSize:                info.Size(),
				ContentHash:             hashResult.ContentHash,
				QuickFingerprint:        hashResult.QuickFingerprint,
				QuickFingerprintVersion: hashResult.QuickFingerprintVersion,
				TakenTime:               dbtypes.NewTimestamp(info.ModTime()),
				Status:                  statusJSON,
			})
			if err != nil {
				return err
			}
			return m.enqueuePipelineTx(ctx, tx, repository, &updated, storagePath, assetType)
		})
		if updateErr != nil {
			m.markPipelineTasksFailed(ctx, existing.AssetID, pipelineTaskNames(assetType), updateErr)
			return nil, fmt.Errorf("update discovered asset and enqueue pipeline: %w", updateErr)
		}
		asset := updated

		m.audit(repository.Path).Operation("asset.materialize.inplace_update",
			zap.String("repository_id", repository.RepoID.String()),
			zap.String("asset_id", asset.AssetID.String()),
			zap.String("storage_path", storagePath),
		)
		return &asset, nil
	}

	// New asset — create
	storagePathPtr := storagePath
	var created *repo.Asset
	createErr := m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		asset, err := createAssetWithMediaItem(ctx, queries, repo.CreateAssetParams{
			OwnerID:                 ownerOrRepoDefault(source.OwnerID, repository.DefaultOwnerID),
			Type:                    string(assetType),
			OriginalFilename:        source.OriginalFilename,
			StoragePath:             &storagePathPtr,
			MimeType:                validation.MimeType,
			FileSize:                info.Size(),
			ContentHash:             hashResult.ContentHash,
			QuickFingerprint:        hashResult.QuickFingerprint,
			QuickFingerprintVersion: hashResult.QuickFingerprintVersion,
			TakenTime:               dbtypes.NewTimestamp(info.ModTime()),
			Rating:                  int64Ptr(0),
			RepositoryID:            uuid.NullUUID{UUID: repoID, Valid: true},
			Status:                  statusJSON,
		}, repo.InitialMediaRelation(validation, source.OriginalFilename))
		if err != nil {
			return err
		}
		created = asset
		return m.enqueuePipelineTx(ctx, tx, repository, asset, storagePath, assetType)
	})
	if createErr != nil {
		// Race: another worker may have created the same asset between our lookup and insert
		if isUniqueConstraintViolation(createErr) {
			latest, fetchErr := m.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
				RepositoryID: uuid.NullUUID{UUID: repoID, Valid: true},
				StoragePath:  &storagePath,
			})
			if fetchErr != nil && !errors.Is(fetchErr, sql.ErrNoRows) {
				return nil, fmt.Errorf("fetch discovered asset after unique conflict: %w", fetchErr)
			}
			if fetchErr == nil {
				created = &latest
			} else {
				return nil, nil
			}
		} else {
			return nil, fmt.Errorf("create discovered asset and enqueue pipeline: %w", createErr)
		}
	}
	if created == nil {
		return nil, nil
	}
	asset := created

	m.audit(repository.Path).Operation("asset.materialize.inplace_create",
		zap.String("repository_id", repository.RepoID.String()),
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

func (m *SourceMaterializer) findExistingContent(ctx context.Context, repositoryID uuid.UUID, contentHash string, fileSize int64) (*repo.Asset, error) {
	rows, err := m.queries.GetAssetsByContentHashesAndRepository(ctx, repo.GetAssetsByContentHashesAndRepositoryParams{
		ContentHashes: dbtypes.StringsJSONParam([]string{contentHash}),
		RepositoryID:  uuid.NullUUID{UUID: repositoryID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("find existing staged content: %w", err)
	}
	for _, row := range rows {
		if fileSize > 0 && row.FileSize != fileSize {
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

func (m *SourceMaterializer) createAssetWithMediaItem(ctx context.Context, params repo.CreateAssetParams, relation repo.StackRelation) (*repo.Asset, error) {
	var created *repo.Asset
	err := m.database.WithTx(ctx, func(_ *sql.Tx, queries *repo.Queries) error {
		var err error
		created, err = createAssetWithMediaItem(ctx, queries, params, relation)
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
		MediaItemID:  mediaItemID,
		OwnerID:      asset.OwnerID,
		RepositoryID: asset.RepositoryID,
		MediaKind:    strings.ToLower(asset.Type),
		AssetID:      uuid.NullUUID{UUID: asset.AssetID, Valid: true},
		CreatedAt:    createdAt,
	}); err != nil {
		return nil, fmt.Errorf("create logical media item: %w", err)
	}
	if err := queries.AttachAssetToMediaItem(ctx, repo.AttachAssetToMediaItemParams{
		AssetID:     asset.AssetID,
		MediaItemID: mediaItemID,
		Relation:    string(relation),
		CreatedAt:   createdAt,
	}); err != nil {
		return nil, fmt.Errorf("attach asset to logical media item: %w", err)
	}
	return &asset, nil
}

func (m *SourceMaterializer) finalizeAssetAndEnqueue(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	storagePath string,
	assetType dbtypes.AssetType,
) (*repo.Asset, error) {
	queuedStatus, err := buildTrackedIngestStatus(
		assetType,
		queuedStatusMessage,
		statusdb.IngestPhasePipelineQueued,
		"",
		"",
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("marshal queued status: %w", err)
	}

	var finalized repo.Asset
	err = m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		var updateErr error
		finalized, updateErr = queries.UpdateAssetStoragePathAndStatus(ctx, repo.UpdateAssetStoragePathAndStatusParams{
			AssetID:     asset.AssetID,
			StoragePath: &storagePath,
			Status:      queuedStatus,
		})
		if updateErr != nil {
			return fmt.Errorf("finalize asset path and status: %w", updateErr)
		}
		return m.enqueuePipelineTx(ctx, tx, repository, &finalized, storagePath, assetType)
	})
	if err != nil {
		return nil, fmt.Errorf("commit asset and pipeline jobs: %w", err)
	}
	return &finalized, nil
}

func (m *SourceMaterializer) recoverMovedStaging(
	ctx context.Context,
	source IngestSource,
	repository repo.Repository,
	assetType dbtypes.AssetType,
) (*repo.Asset, error) {
	if source.ContentHash == nil || !hash.ValidateHash(strings.TrimSpace(*source.ContentHash), hash.AlgorithmBLAKE3) {
		return nil, fmt.Errorf("staged file not found and no authoritative content hash is available")
	}
	existing, err := m.findExistingContent(ctx, repository.RepoID, strings.ToLower(strings.TrimSpace(*source.ContentHash)), 0)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("staged file not found and no prepared asset exists")
	}
	if !isRecoverableStagingAsset(existing) {
		return existing, nil
	}
	if recoveredPath, recoverErr := recoverableStagingPath(repository.Path, existing.Status); recoverErr != nil {
		return nil, recoverErr
	} else if recoveredPath != "" {
		if _, statErr := os.Stat(recoveredPath); statErr == nil {
			source.SourcePath = recoveredPath
			return m.materializeFromStaging(ctx, source, repository, &file.ValidationResult{
				Valid:     true,
				AssetType: assetType,
				MimeType:  existing.MimeType,
			})
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("inspect recoverable staging file: %w", statErr)
		}
	}
	if existing.StoragePath == nil || *existing.StoragePath == "" {
		detail := "prepared asset has no inbox target"
		_ = m.markAssetIngestFailure(ctx, existing.AssetID, statusdb.IngestPhaseCommitFailed, ingestCodeCommitFailed, "", detail)
		return nil, errors.New(detail)
	}
	_, finalPath, err := resolveRepositoryRelativePath(repository.Path, *existing.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("resolve prepared inbox path: %w", err)
	}
	if _, err := os.Stat(finalPath); err != nil {
		if os.IsNotExist(err) {
			detail := "prepared asset has neither staging nor inbox file"
			_ = m.markAssetIngestFailure(ctx, existing.AssetID, statusdb.IngestPhaseCommitFailed, ingestCodeCommitFailed, "", detail)
			return nil, errors.New(detail)
		}
		return nil, fmt.Errorf("inspect prepared inbox file: %w", err)
	}
	expectedHash := strings.ToLower(strings.TrimSpace(*source.ContentHash))
	if err := verifyFinalAndRemoveStaging("", finalPath, expectedHash, existing.FileSize); err != nil {
		m.markStagingConflict(ctx, existing.AssetID, repository.Path, "", err)
		return nil, fmt.Errorf("reconcile prepared inbox file: %w", err)
	}
	finalized, err := m.finalizeAssetAndEnqueue(ctx, repository, existing, *existing.StoragePath, assetType)
	if err != nil {
		m.markPipelineTasksFailed(ctx, existing.AssetID, pipelineTaskNames(assetType), err)
		return nil, err
	}
	return finalized, nil
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

// handleStagingFailure attempts to quarantine a failed staging file and marks
// the asset as recoverable. A quarantine failure always leaves the source file
// in place; original media is never deleted as an error fallback.
func (m *SourceMaterializer) handleStagingFailure(
	ctx context.Context,
	stagingFile *storage.StagingFile,
	repoPath string,
	assetID uuid.UUID,
	commitErr error,
) {
	failureDetail := fmt.Sprintf("commit staging to inbox failed: %v", commitErr)
	recoveryPath := stagingFile.Path

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
		failureDetail = fmt.Sprintf("%s; move to failed dir failed: %v", failureDetail, moveErr)
	} else {
		recoveryPath = stagingFile.Path
	}

	recoveryRelative, relativeErr := relativeRepositoryPath(repoPath, recoveryPath)
	if relativeErr != nil {
		failureDetail = fmt.Sprintf("%s; record recovery path failed: %v", failureDetail, relativeErr)
		recoveryRelative = ""
	}
	if markErr := m.markAssetIngestFailure(
		ctx,
		assetID,
		statusdb.IngestPhaseCommitFailed,
		ingestCodeCommitFailed,
		recoveryRelative,
		failureDetail,
	); markErr != nil {
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

func (m *SourceMaterializer) reconcileDuplicateStaging(
	ctx context.Context,
	source IngestSource,
	repository repo.Repository,
	existing *repo.Asset,
	expectedHash string,
	expectedSize int64,
) (*repo.Asset, error) {
	if existing.StoragePath == nil || strings.TrimSpace(*existing.StoragePath) == "" {
		err := errors.New("existing asset has no storage path")
		m.markStagingConflict(ctx, existing.AssetID, repository.Path, source.SourcePath, err)
		return nil, fmt.Errorf("reconcile duplicate staging file: %w", err)
	}
	_, finalPath, err := resolveRepositoryRelativePath(repository.Path, *existing.StoragePath)
	if err != nil {
		m.markStagingConflict(ctx, existing.AssetID, repository.Path, source.SourcePath, err)
		return nil, fmt.Errorf("resolve existing asset path: %w", err)
	}
	if err := verifyFinalAndRemoveStaging(source.SourcePath, finalPath, expectedHash, expectedSize); err != nil {
		m.markStagingConflict(ctx, existing.AssetID, repository.Path, source.SourcePath, err)
		return nil, fmt.Errorf("reconcile duplicate staging file: %w", err)
	}
	return existing, nil
}

func (m *SourceMaterializer) markStagingConflict(
	ctx context.Context,
	assetID uuid.UUID,
	repositoryPath string,
	stagingPath string,
	conflictErr error,
) {
	recoveryPath := ""
	if stagingPath != "" {
		if relative, err := relativeRepositoryPath(repositoryPath, stagingPath); err == nil {
			recoveryPath = relative
		} else {
			m.logger.Warn("failed to record conflicting staging path",
				zap.String("asset_id", assetID.String()),
				zap.String("staging_path", stagingPath),
				zap.Error(err),
			)
		}
	}
	if err := m.markAssetIngestFailure(
		ctx,
		assetID,
		statusdb.IngestPhaseConflict,
		ingestCodeConflict,
		recoveryPath,
		conflictErr.Error(),
	); err != nil {
		m.logger.Warn("failed to mark staging conflict",
			zap.String("asset_id", assetID.String()),
			zap.Error(err),
		)
	}
	m.audit(repositoryPath).Error("asset.materialize.conflict", conflictErr,
		zap.String("asset_id", assetID.String()),
		zap.String("staging_path", stagingPath),
	)
}

func (m *SourceMaterializer) markAssetIngestFailure(
	ctx context.Context,
	assetID uuid.UUID,
	phase statusdb.IngestPhase,
	code string,
	stagingPath string,
	detail string,
) error {
	return m.queries.MutateAssetStatus(ctx, assetID, func(current statusdb.AssetStatus) (statusdb.AssetStatus, error) {
		current.State = statusdb.StateFailed
		current.Message = "Asset ingestion failed"
		current.AddError("materialize_asset", detail)
		current.SetIngestState(phase, code, stagingPath, true)
		return current, nil
	})
}

// resolveRepository looks up a repository by UUID, falling back to primary.
func (m *SourceMaterializer) resolveRepository(ctx context.Context, repoUUID uuid.UUID) (repo.Repository, error) {
	if repoUUID != uuid.Nil {
		return m.queries.GetRepository(ctx, repoUUID)
	}

	repository, err := m.queries.GetPrimaryRepository(ctx)
	if err != nil {
		return repo.Repository{}, fmt.Errorf("no repository available: %w", err)
	}
	return repository, nil
}

// enqueuePipelineTx inserts all core jobs on the same transaction as the asset
// status/path write. Any insert failure rolls the complete unit back.
func (m *SourceMaterializer) enqueuePipelineTx(
	ctx context.Context,
	tx *sql.Tx,
	repository repo.Repository,
	asset *repo.Asset,
	storagePath string,
	assetType dbtypes.AssetType,
) error {
	assetID := asset.AssetID
	commonMeta := jobs.MetadataArgs{
		AssetID:          assetID,
		RepoPath:         repository.Path,
		StoragePath:      storagePath,
		AssetType:        assetType,
		OriginalFilename: asset.OriginalFilename,
		FileSize:         asset.FileSize,
		MimeType:         asset.MimeType,
	}
	commonThumb := jobs.ThumbnailArgs{
		AssetID:     assetID,
		RepoPath:    repository.Path,
		StoragePath: storagePath,
		AssetType:   assetType,
	}
	commonTranscode := jobs.TranscodeArgs{
		AssetID:     assetID,
		RepoPath:    repository.Path,
		StoragePath: storagePath,
		AssetType:   assetType,
	}

	// Metadata is always first
	_, err := m.queueClient.InsertTx(ctx, tx, commonMeta, &river.InsertOpts{Queue: "metadata_asset"})
	if err != nil {
		return fmt.Errorf("enqueue metadata: %w", err)
	}

	switch assetType {
	case dbtypes.AssetTypePhoto:
		_, err = m.queueClient.InsertTx(ctx, tx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}

	case dbtypes.AssetTypeVideo:
		_, err = m.queueClient.InsertTx(ctx, tx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}
		_, err = m.queueClient.InsertTx(ctx, tx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return fmt.Errorf("enqueue transcode: %w", err)
		}

	case dbtypes.AssetTypeAudio:
		_, err = m.queueClient.InsertTx(ctx, tx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return fmt.Errorf("enqueue transcode: %w", err)
		}

	default:
		return fmt.Errorf("unsupported asset type: %s", assetType)
	}

	return nil
}

// markPipelineTasksFailed marks individual pipeline tasks as failed in the
// asset status before they were ever queued.
func (m *SourceMaterializer) markPipelineTasksFailed(ctx context.Context, assetID uuid.UUID, tasks []string, cause error) {
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

// BuildTrackedProcessingStatus builds an initial tracked-processing status JSON blob.
func BuildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	return buildTrackedProcessingStatus(assetType, message)
}

func buildTrackedProcessingStatus(assetType dbtypes.AssetType, message string) ([]byte, error) {
	s := statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType))
	return s.ToJSON()
}

func buildTrackedIngestStatus(
	assetType dbtypes.AssetType,
	message string,
	phase statusdb.IngestPhase,
	code string,
	stagingPath string,
	recoverable bool,
) ([]byte, error) {
	s := statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType))
	s.SetIngestState(phase, code, stagingPath, recoverable)
	return s.ToJSON()
}

func recoverableStagingPath(repositoryPath string, rawStatus []byte) (string, error) {
	status, err := statusdb.FromJSON(rawStatus)
	if err != nil {
		return "", fmt.Errorf("parse recoverable ingest status: %w", err)
	}
	if status.Ingest == nil || !status.Ingest.Recoverable || status.Ingest.StagingPath == "" {
		return "", nil
	}
	_, fullPath, err := resolveRepositoryRelativePath(repositoryPath, status.Ingest.StagingPath)
	if err != nil {
		return "", fmt.Errorf("resolve recoverable staging path: %w", err)
	}
	return fullPath, nil
}

func relativeRepositoryPath(repositoryPath, fullPath string) (string, error) {
	root, err := filepath.Abs(filepath.Clean(repositoryPath))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	path, err := filepath.Abs(filepath.Clean(fullPath))
	if err != nil {
		return "", fmt.Errorf("resolve repository file: %w", err)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("make repository-relative path: %w", err)
	}
	if escapesRepository(relative) {
		return "", errors.New("path escapes repository root")
	}
	return filepath.ToSlash(relative), nil
}

func resolveRepositoryRelativePath(repositoryPath, relativePath string) (string, string, error) {
	if strings.TrimSpace(relativePath) == "" {
		return "", "", errors.New("repository-relative path is empty")
	}
	if storage.IsRootedPath(relativePath) {
		return "", "", errors.New("repository-relative path is rooted or volume-qualified")
	}

	normalized := strings.ReplaceAll(relativePath, `\`, "/")
	cleanRelative := filepath.Clean(filepath.FromSlash(normalized))
	root, err := filepath.Abs(filepath.Clean(repositoryPath))
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	fullPath := filepath.Join(root, cleanRelative)
	contained, err := filepath.Rel(root, fullPath)
	if err != nil {
		return "", "", fmt.Errorf("check repository containment: %w", err)
	}
	if escapesRepository(contained) {
		return "", "", errors.New("path escapes repository root")
	}
	return filepath.ToSlash(contained), fullPath, nil
}

func resolveInPlaceSource(repositoryPath, sourcePath string) (string, string, error) {
	storagePath, fullPath, err := resolveRepositoryRelativePath(repositoryPath, sourcePath)
	if err != nil {
		return "", "", fmt.Errorf("resolve in-place source: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(repositoryPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve repository symlinks: %w", err)
	}
	resolvedRoot, err = filepath.Abs(filepath.Clean(resolvedRoot))
	if err != nil {
		return "", "", fmt.Errorf("resolve repository root: %w", err)
	}
	resolvedFile, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve in-place source symlinks: %w", err)
	}
	resolvedFile, err = filepath.Abs(filepath.Clean(resolvedFile))
	if err != nil {
		return "", "", fmt.Errorf("resolve in-place source: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedFile)
	if err != nil {
		return "", "", fmt.Errorf("check resolved source containment: %w", err)
	}
	if escapesRepository(relative) {
		return "", "", errors.New("in-place source symlink escapes repository root")
	}
	return storagePath, fullPath, nil
}

func escapesRepository(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func verifyFinalAndRemoveStaging(stagingPath, finalPath, expectedHash string, expectedSize int64) error {
	info, err := os.Stat(finalPath)
	if err != nil {
		return fmt.Errorf("inspect final file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("final path is not a regular file")
	}
	if info.Size() != expectedSize {
		return fmt.Errorf("final file size conflict: got %d, want %d", info.Size(), expectedSize)
	}
	actualHash, err := hash.CalculateBLAKE3(finalPath)
	if err != nil {
		return fmt.Errorf("hash final file: %w", err)
	}
	if !strings.EqualFold(actualHash, expectedHash) {
		return errors.New("final file content conflict: BLAKE3 mismatch")
	}
	if stagingPath == "" {
		return nil
	}
	if err := os.Remove(stagingPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove verified duplicate staging file: %w", err)
	}
	return nil
}

func ownerOrRepoDefault(owner *int32, defaultOwner *int32) *int32 {
	if owner != nil {
		return owner
	}
	return defaultOwner
}

func int64Ptr(v int64) *int64 {
	return &v
}

func isSoftDeleted(a repo.Asset) bool {
	return a.IsDeleted
}

func isUniqueConstraintViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	return false
}
