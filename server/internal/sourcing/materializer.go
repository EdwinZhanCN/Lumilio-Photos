package sourcing

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db"
	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/storage"
	roematerializer "server/internal/storage/roe/materializer"
	fileutil "server/internal/utils/file"
	"server/internal/utils/hash"
)

const (
	TaskMetadata  = "metadata_asset"
	TaskThumbnail = "thumbnail_asset"
	TaskTranscode = "transcode_asset"
)

// SourceMaterializer owns the recoverable private-staging to ROE commit
// boundary for upload and cloud sources. Asset processing is published through
// the ROE outbox; this type never inserts processing jobs directly.
type SourceMaterializer struct {
	database       *db.DB
	stagingManager storage.StagingManager
	files          *storage.RepositoryFSFactory
	publisher      *roematerializer.HashMaterializer
	logger         *zap.Logger
	auditProvider  logging.RepositoryAuditProvider
	capacityGuard  interface {
		CheckRepositoryWriteCapacity(context.Context, string, uint64) (storage.CapacityDecision, error)
	}
}

func NewSourceMaterializer(
	database *db.DB,
	stagingManager storage.StagingManager,
	publisher *roematerializer.HashMaterializer,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *storage.RepositoryFSFactory,
) *SourceMaterializer {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SourceMaterializer{
		database: database, stagingManager: stagingManager, files: files,
		publisher: publisher, logger: logger.With(zap.String("component", "source_materializer")),
		auditProvider: auditProvider,
	}
}

func (m *SourceMaterializer) SetCapacityGuard(guard interface {
	CheckRepositoryWriteCapacity(context.Context, string, uint64) (storage.CapacityDecision, error)
}) {
	m.capacityGuard = guard
}

// MaterializeStaged registers an unjournaled source when needed and resumes the
// durable commit. A returned prepared error means catalog state owns the bytes
// and callers must not remove them.
func (m *SourceMaterializer) MaterializeStaged(ctx context.Context, source IngestSource) (result *repo.Asset, resultErr error) {
	prepared := source.CommitID != uuid.Nil
	defer func() {
		if resultErr != nil {
			resultErr = WithStagingOwnership(resultErr, prepared)
		}
	}()
	if m == nil || m.database == nil || m.stagingManager == nil || m.publisher == nil {
		return nil, errors.New("source materializer unavailable")
	}
	if source.Kind != IngestSourceUpload && source.Kind != IngestSourceCloud {
		return nil, fmt.Errorf("source %q is not staged ingest", source.Kind)
	}
	if source.CommitID != uuid.Nil {
		return m.MaterializeCommit(ctx, source.CommitID)
	}

	validation := fileutil.ValidateFile(source.OriginalFilename, source.ContentType)
	if !validation.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorReason)
	}
	repository, err := m.resolveRepository(ctx, source.RepositoryID)
	if err != nil {
		return nil, err
	}
	ownerID, err := resolvedOwner(source.OwnerID, repository.DefaultOwnerID)
	if err != nil {
		return nil, err
	}
	stagingFile, err := stagingHandle(repository.RepoID, source.StagingPath, source.OriginalFilename, source.Timestamp)
	if err != nil {
		return nil, err
	}
	verified, err := m.hashStaging(ctx, repository, stagingFile)
	if err != nil {
		return nil, err
	}
	if source.ContentHash != nil && !strings.EqualFold(strings.TrimSpace(*source.ContentHash), verified.ContentHash) {
		return nil, errors.New("staged file does not match the server-verified source hash")
	}
	commitID := uuid.New()
	now := dbtypes.NewTimestamp(time.Now().UTC())
	_, err = m.database.Queries.CreateRepositoryStagingCommit(ctx, repo.CreateRepositoryStagingCommitParams{
		CommitID: commitID, RepositoryID: repository.RepoID, OwnerID: ownerID,
		SourceKind: string(source.Kind), StagingPath: stagingFile.PrivatePath,
		OriginalFilename: source.OriginalFilename, MimeType: validation.MimeType,
		FullHash: strings.ToLower(verified.ContentHash), FileSize: verified.FileSize,
		QuickFingerprint:        verified.QuickFingerprint,
		QuickFingerprintVersion: verified.QuickFingerprintVersion, CreatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("journal source staging: %w", err)
	}
	prepared = true
	return m.materializeCommit(ctx, commitID, verified)
}

// MaterializeCommit resumes one staging journal by stable identifier. It is
// safe after crashes before/after the filesystem rename, ROE publication, or
// journal completion.
func (m *SourceMaterializer) MaterializeCommit(ctx context.Context, commitID uuid.UUID) (*repo.Asset, error) {
	return m.materializeCommit(ctx, commitID, nil)
}

func (m *SourceMaterializer) materializeCommit(ctx context.Context, commitID uuid.UUID, preverified *hash.LayeredHashResult) (*repo.Asset, error) {
	record, err := m.database.ReaderQueries.GetRepositoryStagingCommit(ctx, commitID)
	if err != nil {
		return nil, fmt.Errorf("load staging commit: %w", err)
	}
	if record.Status == "completed" && record.AssetID.Valid {
		asset, assetErr := m.database.ReaderQueries.GetAssetByIDAny(ctx, record.AssetID.UUID)
		return &asset, assetErr
	}
	if record.Status == "quarantined" {
		return nil, fmt.Errorf("staging commit %s is quarantined: %s", commitID, valueOrEmptyString(record.FailureDetail))
	}
	record, err = m.database.Queries.ClaimRepositoryStagingCommit(ctx, repo.ClaimRepositoryStagingCommitParams{
		CommitID: commitID, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	})
	if err != nil {
		return nil, fmt.Errorf("claim staging commit: %w", err)
	}
	repository, err := m.database.ReaderQueries.GetRepository(ctx, record.RepositoryID)
	if err != nil {
		return nil, err
	}
	validation := fileutil.ValidateFile(record.OriginalFilename, record.MimeType)
	if !validation.Valid {
		return nil, m.quarantine(ctx, repository, record, "unsupported_media", errors.New(validation.ErrorReason))
	}
	stagingFile, err := stagingHandle(record.RepositoryID, record.StagingPath, record.OriginalFilename, record.CreatedAt.Time)
	if err != nil {
		return nil, m.quarantine(ctx, repository, record, "invalid_staging_handle", err)
	}
	target := ""
	if record.TargetPath != nil {
		target = *record.TargetPath
	} else {
		target, err = m.stagingManager.ResolveInboxPath(repository, record.OriginalFilename, record.FullHash)
		if err != nil {
			return nil, err
		}
		record, err = m.database.Queries.SetRepositoryStagingCommitTarget(ctx, repo.SetRepositoryStagingCommitTargetParams{
			CommitID: commitID, TargetPath: &target, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		})
		if err != nil {
			return nil, err
		}
	}

	verified := preverified
	opened, openErr := m.stagingManager.OpenStagingFile(repository, stagingFile)
	stagingExists := openErr == nil
	if openErr != nil && !errors.Is(openErr, fs.ErrNotExist) {
		return nil, openErr
	}
	if stagingExists {
		if verified == nil {
			verified, err = hashOpenedStaging(opened)
		} else {
			err = opened.Close()
		}
		if err != nil {
			return nil, err
		}
		if verified.FileSize != record.FileSize || !strings.EqualFold(verified.ContentHash, record.FullHash) {
			return nil, m.quarantine(ctx, repository, record, "staging_content_changed", errors.New("staging bytes no longer match their durable identity"))
		}
	}

	finalObservation, finalExists, inspectErr := m.inspectFinal(ctx, repository, target, storage.HashFull)
	if inspectErr != nil {
		return nil, inspectErr
	}
	if finalExists {
		if finalObservation.Size != record.FileSize || finalObservation.ContentHash == nil ||
			!strings.EqualFold(*finalObservation.ContentHash, record.FullHash) {
			return nil, m.quarantine(ctx, repository, record, "target_conflict", errors.New("inbox target contains different bytes"))
		}
		if stagingExists {
			if err := m.stagingManager.RemoveStagingFile(repository, stagingFile); err != nil {
				return nil, err
			}
		}
	} else {
		if !stagingExists {
			return nil, m.quarantine(ctx, repository, record, "source_missing", errors.New("neither staged nor committed source exists"))
		}
		if err := m.stagingManager.CommitStagingFile(repository, stagingFile, target); err != nil {
			return nil, m.quarantine(ctx, repository, record, "staging_commit_failed", err)
		}
	}
	if _, err := m.database.Queries.MarkRepositoryStagingCommitOnDisk(ctx, repo.MarkRepositoryStagingCommitOnDiskParams{
		CommitID: commitID, TargetPath: &target, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
	}); err != nil {
		return nil, err
	}
	finalObservation, finalExists, err = m.inspectFinal(ctx, repository, target, storage.HashNone)
	if err != nil || !finalExists || finalObservation.Size != record.FileSize {
		return nil, fmt.Errorf("verify committed source stability: %w", err)
	}
	result, err := m.publisher.PublishKnownContent(ctx, roematerializer.KnownContent{
		RepositoryID: record.RepositoryID, OwnerID: record.OwnerID,
		Source: record.SourceKind, SourceEventKey: "staging:" + commitID.String(),
		RelativePath: target, OriginalFilename: record.OriginalFilename,
		MimeType: record.MimeType, AssetType: string(validation.AssetType),
		FullHash: record.FullHash, FileSize: record.FileSize,
		QuickFingerprint:        record.QuickFingerprint,
		QuickFingerprintVersion: record.QuickFingerprintVersion,
		Observation:             finalObservation,
	})
	if err != nil {
		return nil, err
	}
	completedAt := dbtypes.NewTimestamp(time.Now().UTC())
	completedAtMicros := completedAt.Time.UnixMicro()
	if _, err := m.database.Queries.CompleteRepositoryStagingCommit(ctx, repo.CompleteRepositoryStagingCommitParams{
		CommitID: commitID, NodeID: uuid.NullUUID{UUID: result.NodeID, Valid: true},
		AssetID: uuid.NullUUID{UUID: result.AssetID, Valid: true}, CompletedAt: &completedAtMicros,
	}); err != nil {
		return nil, err
	}
	asset, err := m.database.ReaderQueries.GetAssetByIDAny(ctx, result.AssetID)
	if err != nil {
		return nil, err
	}
	m.audit(repository.Path).Operation("asset.materialize.staging",
		zap.String("repository_id", repository.RepoID.String()),
		zap.String("asset_id", asset.AssetID.String()),
		zap.String("commit_id", commitID.String()),
		zap.String("source_kind", record.SourceKind),
	)
	return &asset, nil
}

func (m *SourceMaterializer) hashStaging(ctx context.Context, repository repo.Repository, stagingFile *storage.StagingFile) (*hash.LayeredHashResult, error) {
	opened, err := m.stagingManager.OpenStagingFile(repository, stagingFile)
	if err != nil {
		return nil, err
	}
	info, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	if m.capacityGuard != nil {
		size := uint64(0)
		if info.Size() > 0 {
			size = uint64(info.Size())
		}
		if _, err := m.capacityGuard.CheckRepositoryWriteCapacity(ctx, repository.RepoID.String(), size); err != nil {
			_ = opened.Close()
			return nil, err
		}
	}
	return hashOpenedStaging(opened)
}

func hashOpenedStaging(opened *storage.RepositoryFile) (*hash.LayeredHashResult, error) {
	before, err := opened.Stat()
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	result, hashErr := hash.CalculateLayeredBLAKE3Reader(opened, before.Size())
	after, statErr := opened.Stat()
	closeErr := opened.Close()
	if err := errors.Join(hashErr, statErr, closeErr); err != nil {
		return nil, err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, storage.ErrRepositoryFileUnstable
	}
	return result, nil
}

func (m *SourceMaterializer) inspectFinal(ctx context.Context, repository repo.Repository, target string, mode storage.HashMode) (storage.FileObservation, bool, error) {
	repositoryPath, err := storage.ParseUserMediaPath(target)
	if err != nil {
		return storage.FileObservation{}, false, err
	}
	repositoryFS, err := m.files.OpenContext(ctx, repository)
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
	observation, err := repositoryFS.InspectMedia(ctx, repositoryPath, mode)
	if err != nil {
		return storage.FileObservation{}, true, err
	}
	if err := repositoryFS.Revalidate(ctx, observation); err != nil {
		return storage.FileObservation{}, true, err
	}
	return observation, true, nil
}

func (m *SourceMaterializer) quarantine(ctx context.Context, repository repo.Repository, record repo.RepositoryStagingCommit, code string, cause error) error {
	stagingPath := record.StagingPath
	handle, handleErr := stagingHandle(record.RepositoryID, record.StagingPath, record.OriginalFilename, record.CreatedAt.Time)
	if handleErr == nil {
		if moveErr := m.stagingManager.MoveStagingToFailed(repository, handle); moveErr == nil {
			stagingPath = handle.PrivatePath
		} else if !errors.Is(moveErr, fs.ErrNotExist) {
			cause = errors.Join(cause, fmt.Errorf("quarantine staging bytes: %w", moveErr))
		}
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	detail := cause.Error()
	_, updateErr := m.database.Queries.QuarantineRepositoryStagingCommit(ctx, repo.QuarantineRepositoryStagingCommitParams{
		CommitID: record.CommitID, StagingPath: stagingPath, FailureCode: &code,
		FailureDetail: &detail, UpdatedAt: now,
	})
	if updateErr != nil {
		return errors.Join(cause, updateErr)
	}
	m.logger.Warn("staging commit quarantined", zap.String("commit_id", record.CommitID.String()), zap.String("code", code), zap.Error(cause))
	return cause
}

func stagingHandle(repositoryID uuid.UUID, privatePath, filename string, createdAt time.Time) (*storage.StagingFile, error) {
	parsed, err := storage.ParsePrivateRepositoryPath(privatePath)
	if err != nil {
		return nil, err
	}
	return &storage.StagingFile{
		ID: path.Base(parsed.String()), RepositoryID: repositoryID,
		PrivatePath: parsed.String(), Filename: path.Base(strings.ReplaceAll(filename, "\\", "/")),
		CreatedAt: createdAt,
	}, nil
}

func resolvedOwner(sourceOwner, repositoryDefault *int32) (int32, error) {
	if sourceOwner != nil && *sourceOwner > 0 {
		return *sourceOwner, nil
	}
	if repositoryDefault != nil && *repositoryDefault > 0 {
		return *repositoryDefault, nil
	}
	return 0, errors.New("source has no resolved owner")
}

func (m *SourceMaterializer) resolveRepository(ctx context.Context, repositoryID uuid.UUID) (repo.Repository, error) {
	if repositoryID != uuid.Nil {
		return m.database.ReaderQueries.GetRepository(ctx, repositoryID)
	}
	repository, err := m.database.ReaderQueries.GetPrimaryRepository(ctx)
	if err != nil {
		return repo.Repository{}, fmt.Errorf("no repository available: %w", err)
	}
	return repository, nil
}

func (m *SourceMaterializer) audit(repositoryPath string) logging.RepositoryAuditLogger {
	if m.auditProvider == nil {
		return logging.NoopRepositoryAuditLogger()
	}
	return m.auditProvider.ForPath(repositoryPath)
}

func valueOrEmptyString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
	return statusdb.NewTrackedProcessingStatus(message, pipelineTaskNames(assetType)).ToJSON()
}
