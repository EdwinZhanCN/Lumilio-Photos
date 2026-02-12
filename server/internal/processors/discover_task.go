package processors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"server/internal/db/dbtypes"
	"server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/utils/file"
	"server/internal/utils/hash"
)

// ProcessDiscoveredAsset ingests files discovered by repository tree monitoring.
func (ap *AssetProcessor) ProcessDiscoveredAsset(ctx context.Context, args jobs.DiscoverAssetArgs) error {
	repoUUID, err := uuid.Parse(strings.TrimSpace(args.RepositoryID))
	if err != nil {
		return fmt.Errorf("invalid repository id: %w", err)
	}

	storagePath, err := sanitizeDiscoveredPath(args.RelativePath)
	if err != nil {
		return err
	}
	repoID := pgtype.UUID{Bytes: repoUUID, Valid: true}
	operation := normalizeDiscoverOperation(args.Operation)

	if operation == jobs.DiscoverOperationDelete {
		_, err = ap.queries.SoftDeleteAssetByRepositoryAndStoragePath(ctx, repo.SoftDeleteAssetByRepositoryAndStoragePathParams{
			RepositoryID: repoID,
			StoragePath:  &storagePath,
		})
		if err != nil {
			return fmt.Errorf("soft delete discovered asset (%s): %w", storagePath, err)
		}
		return nil
	}

	repository, err := ap.queries.GetRepository(ctx, repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	fullPath := filepath.Join(repository.Path, filepath.FromSlash(storagePath))

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat discovered file: %w", err)
	}
	if info.IsDir() {
		return nil
	}

	filename := strings.TrimSpace(args.FileName)
	if filename == "" {
		filename = filepath.Base(storagePath)
	}

	contentType := strings.TrimSpace(args.ContentType)
	validation := file.ValidateFile(filename, contentType)
	if !validation.Valid {
		// Discovery queue should not retry unsupported files.
		return nil
	}

	if contentType == "" || contentType == "application/octet-stream" {
		contentType = file.NewValidator().GetMimeTypeFromExtension(validation.Extension)
	}

	hashResult, err := hash.CalculateFileHash(fullPath, hash.AlgorithmBLAKE3, true)
	if err != nil {
		return fmt.Errorf("calculate hash: %w", err)
	}

	initialStatus := status.NewProcessingStatus("Asset discovery ingestion started")
	statusJSON, err := initialStatus.ToJSONB()
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	rating := int32(0)
	storagePathPtr := storagePath
	hashPtr := hashResult.Hash
	createdOrUpdatedAsset := (*repo.Asset)(nil)

	existing, err := ap.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
		RepositoryID: repoID,
		StoragePath:  &storagePath,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("find discovered asset by path: %w", err)
	}
	if err == nil {
		wasDeleted := isSoftDeleted(existing)
		hashUnchanged := existing.Hash != nil && *existing.Hash == hashResult.Hash
		if !wasDeleted && hashUnchanged && existing.FileSize == info.Size() && strings.EqualFold(existing.MimeType, contentType) {
			return nil
		}

		updated, updateErr := ap.queries.UpdateDiscoveredAssetByID(ctx, repo.UpdateDiscoveredAssetByIDParams{
			AssetID:          existing.AssetID,
			OriginalFilename: filename,
			MimeType:         contentType,
			FileSize:         info.Size(),
			Hash:             &hashPtr,
			TakenTime:        pgtype.Timestamptz{Time: info.ModTime().UTC(), Valid: true},
			Status:           statusJSON,
		})
		if updateErr != nil {
			return fmt.Errorf("update discovered asset: %w", updateErr)
		}
		createdOrUpdatedAsset = &updated
	}

	if createdOrUpdatedAsset == nil {
		asset, err := ap.assetService.CreateAssetRecord(ctx, repo.CreateAssetParams{
			OwnerID:          nil,
			Type:             string(validation.AssetType),
			OriginalFilename: filename,
			StoragePath:      &storagePathPtr,
			MimeType:         contentType,
			FileSize:         info.Size(),
			Hash:             &hashPtr,
			TakenTime:        pgtype.Timestamptz{Time: info.ModTime().UTC(), Valid: true},
			Rating:           &rating,
			RepositoryID:     repository.RepoID,
			Status:           statusJSON,
		})
		if err != nil {
			if isUniqueConstraintViolation(err) {
				latestAsset, fetchErr := ap.queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
					RepositoryID: repoID,
					StoragePath:  &storagePath,
				})
				if fetchErr == nil {
					createdOrUpdatedAsset = &latestAsset
				} else if !errors.Is(fetchErr, pgx.ErrNoRows) {
					return fmt.Errorf("fetch discovered asset after unique conflict: %w", fetchErr)
				}
			} else {
				return fmt.Errorf("create discovered asset: %w", err)
			}
		}
		if createdOrUpdatedAsset == nil {
			if asset == nil {
				return nil
			}
			createdOrUpdatedAsset = asset
		}
	}

	return ap.enqueueDiscoveredDownstream(ctx, repository, createdOrUpdatedAsset, storagePath, fullPath)
}

func (ap *AssetProcessor) enqueueDiscoveredDownstream(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	storagePath string,
	fullPath string,
) error {
	if asset == nil {
		return fmt.Errorf("discovered asset is nil")
	}

	pgID := asset.AssetID
	assetType := dbtypes.AssetType(asset.Type)
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

	_, err := ap.queueClient.Insert(ctx, commonMeta, &river.InsertOpts{Queue: "metadata_asset"})
	if err != nil {
		return fmt.Errorf("enqueue metadata: %w", err)
	}

	switch assetType {
	case dbtypes.AssetTypePhoto:
		_, err = ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}

		if err = ap.enqueueMLJobs(ctx, asset, fullPath); err != nil {
			return fmt.Errorf("enqueue ML jobs: %w", err)
		}
	case dbtypes.AssetTypeVideo:
		_, err = ap.queueClient.Insert(ctx, commonThumb, &river.InsertOpts{Queue: "thumbnail_asset"})
		if err != nil {
			return fmt.Errorf("enqueue thumbnails: %w", err)
		}
		_, err = ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return fmt.Errorf("enqueue transcode: %w", err)
		}
	case dbtypes.AssetTypeAudio:
		_, err = ap.queueClient.Insert(ctx, commonTranscode, &river.InsertOpts{Queue: "transcode_asset"})
		if err != nil {
			return fmt.Errorf("enqueue transcode: %w", err)
		}
	default:
		return fmt.Errorf("unsupported asset type: %s", assetType)
	}

	return nil
}

func normalizeDiscoverOperation(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", jobs.DiscoverOperationUpsert:
		return jobs.DiscoverOperationUpsert
	case jobs.DiscoverOperationDelete:
		return jobs.DiscoverOperationDelete
	default:
		return jobs.DiscoverOperationUpsert
	}
}

func sanitizeDiscoveredPath(path string) (string, error) {
	raw := strings.TrimSpace(path)
	if raw == "" {
		return "", fmt.Errorf("empty discovered path")
	}

	clean := filepath.Clean(filepath.FromSlash(raw))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." {
		return "", fmt.Errorf("invalid discovered relative path: %s", path)
	}
	if strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("discovered path escapes repository: %s", path)
	}

	normalized := filepath.ToSlash(clean)

	// Discovery only operates in user workspace, excluding internal system/upload areas.
	if normalized == ".lumilio" || strings.HasPrefix(normalized, ".lumilio/") {
		return "", fmt.Errorf("discovered path under system directory is not allowed: %s", path)
	}
	if normalized == "inbox" || strings.HasPrefix(normalized, "inbox/") {
		return "", fmt.Errorf("discovered path under inbox is not allowed: %s", path)
	}

	return normalized, nil
}

func isSoftDeleted(asset repo.Asset) bool {
	return asset.IsDeleted != nil && *asset.IsDeleted
}

func isUniqueConstraintViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
