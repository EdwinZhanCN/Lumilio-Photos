package processors

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/queue/jobs"
	"server/internal/sourcing"
)

// ProcessDiscoveredAsset ingests files discovered by repository tree monitoring.
// Delete operations are handled directly; upsert operations delegate to the SourceMaterializer.
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
		repository, repoErr := ap.queries.GetRepository(ctx, repoID)
		if repoErr == nil {
			ap.repoAudit(repository.Path).Operation("asset.discover.delete",
				zap.String("repository_id", args.RepositoryID),
				zap.String("storage_path", storagePath),
			)
		}
		return nil
	}

	// Upsert: delegate to the materializer (file validation, hash, create-or-update, pipeline)
	filename := strings.TrimSpace(args.FileName)
	if filename == "" {
		filename = filepath.Base(storagePath)
	}

	_, err = ap.materializer.Materialize(ctx, sourcing.IngestSource{
		RepositoryID:     repoUUID,
		Kind:             sourcing.IngestSourceScan,
		SourcePath:       storagePath, // repo-relative path
		OriginalFilename: filename,
		ContentType:      args.ContentType,
		Timestamp:        args.DetectedAt,
	})
	return err
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
