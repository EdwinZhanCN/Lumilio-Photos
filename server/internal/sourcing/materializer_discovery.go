package sourcing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	statusdb "server/internal/db/dbtypes/status"
	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/utils/file"
	"server/internal/utils/hash"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MaterializeDiscovered consumes one generation-bound file-index candidate.
// Stale jobs are successful no-ops; only the current indexed observation may
// create or update an Asset.
func (m *SourceMaterializer) MaterializeDiscovered(
	ctx context.Context,
	repositoryID uuid.UUID,
	storagePath string,
	scanID uuid.UUID,
	observationToken string,
) (*repo.Asset, error) {
	if m == nil || m.database == nil || m.queries == nil || m.files == nil {
		return nil, fmt.Errorf("repository discovery materializer unavailable")
	}
	repositoryPath, err := storage.ParseUserMediaPath(storagePath)
	if err != nil {
		return nil, err
	}
	indexed, err := m.queries.GetRepositoryFileIndexEntry(ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: repositoryID,
		StoragePath:  repositoryPath.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load discovery file index: %w", err)
	}
	if indexed.State != "present" || !indexed.LastSeenScanID.Valid || indexed.LastSeenScanID.UUID != scanID || indexed.ObservationToken != observationToken {
		return nil, nil
	}

	repository, err := m.queries.GetRepository(ctx, repositoryID)
	if err != nil {
		return nil, fmt.Errorf("load discovery repository: %w", err)
	}
	repositoryFS, err := m.files.Open(repository)
	if err != nil {
		return nil, err
	}
	defer repositoryFS.Close()
	mode := storage.HashFull
	if indexed.FileSize > hash.QuickHashThreshold {
		mode = storage.HashQuickAndFull
	}
	observation, err := repositoryFS.InspectMedia(ctx, repositoryPath, mode)
	if err != nil {
		return nil, fmt.Errorf("inspect discovered media: %w", err)
	}
	observation.ScanID = scanID
	if observation.ObservationToken != observationToken {
		return nil, nil
	}
	if err := repositoryFS.Revalidate(ctx, observation); err != nil {
		return nil, nil
	}
	if observation.ContentHash == nil {
		return nil, fmt.Errorf("discovered media inspection returned no full content hash")
	}

	filename := path.Base(repositoryPath.String())
	validation := file.ValidateFile(filename, "")
	if !validation.Valid {
		return nil, fmt.Errorf("file validation failed: %s", validation.ErrorReason)
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
		return nil, fmt.Errorf("marshal discovery status: %w", err)
	}

	var materialized *repo.Asset
	err = m.database.WithTx(ctx, func(tx *sql.Tx, queries *repo.Queries) error {
		currentIndex, err := queries.GetRepositoryFileIndexEntry(ctx, repo.GetRepositoryFileIndexEntryParams{
			RepositoryID: repositoryID,
			StoragePath:  repositoryPath.String(),
		})
		if err != nil {
			return err
		}
		if currentIndex.State != "present" || !currentIndex.LastSeenScanID.Valid || currentIndex.LastSeenScanID.UUID != scanID || currentIndex.ObservationToken != observationToken {
			return nil
		}

		pathValue := repositoryPath.String()
		existing, findErr := queries.GetAssetByRepositoryAndStoragePathAny(ctx, repo.GetAssetByRepositoryAndStoragePathAnyParams{
			RepositoryID: uuid.NullUUID{UUID: repositoryID, Valid: true},
			StoragePath:  &pathValue,
		})
		switch {
		case findErr == nil:
			if !existing.IsDeleted && existing.ContentHash == *observation.ContentHash &&
				existing.FileSize == observation.Size && strings.EqualFold(existing.MimeType, validation.MimeType) {
				if _, err := queries.BindRepositoryFileIndexAsset(ctx, repo.BindRepositoryFileIndexAssetParams{
					AssetID:      uuid.NullUUID{UUID: existing.AssetID, Valid: true},
					UpdatedAt:    dbtypes.NewTimestamp(time.Now().UTC()),
					RepositoryID: repositoryID,
					StoragePath:  pathValue,
				}); err != nil {
					return err
				}
				materialized = &existing
				return nil
			}
			updated, err := queries.UpdateDiscoveredAssetByID(ctx, repo.UpdateDiscoveredAssetByIDParams{
				AssetID:                 existing.AssetID,
				OriginalFilename:        filename,
				MimeType:                validation.MimeType,
				FileSize:                observation.Size,
				ContentHash:             *observation.ContentHash,
				QuickFingerprint:        observation.QuickFingerprint,
				QuickFingerprintVersion: observation.QuickFingerprintVer,
				TakenTime:               dbtypes.NewTimestamp(time.Unix(0, observation.ModTimeNS)),
				Status:                  statusJSON,
			})
			if err != nil {
				return err
			}
			if _, err := queries.BindRepositoryFileIndexAsset(ctx, repo.BindRepositoryFileIndexAssetParams{
				AssetID:      uuid.NullUUID{UUID: updated.AssetID, Valid: true},
				UpdatedAt:    dbtypes.NewTimestamp(time.Now().UTC()),
				RepositoryID: repositoryID,
				StoragePath:  pathValue,
			}); err != nil {
				return err
			}
			if err := m.enqueuePipelineTx(ctx, tx, &updated, validation.AssetType, observationToken); err != nil {
				return err
			}
			materialized = &updated
			return nil
		case !errors.Is(findErr, sql.ErrNoRows):
			return findErr
		}

		created, err := createAssetWithMediaItem(ctx, queries, repo.CreateAssetParams{
			OwnerID:                 repository.DefaultOwnerID,
			Type:                    string(validation.AssetType),
			OriginalFilename:        filename,
			StoragePath:             &pathValue,
			MimeType:                validation.MimeType,
			FileSize:                observation.Size,
			ContentHash:             *observation.ContentHash,
			QuickFingerprint:        observation.QuickFingerprint,
			QuickFingerprintVersion: observation.QuickFingerprintVer,
			TakenTime:               dbtypes.NewTimestamp(time.Unix(0, observation.ModTimeNS)),
			Rating:                  int64Ptr(0),
			RepositoryID:            uuid.NullUUID{UUID: repositoryID, Valid: true},
			Status:                  statusJSON,
		}, repo.InitialMediaRelation(validation, filename))
		if err != nil {
			return err
		}
		if _, err := queries.BindRepositoryFileIndexAsset(ctx, repo.BindRepositoryFileIndexAssetParams{
			AssetID:      uuid.NullUUID{UUID: created.AssetID, Valid: true},
			UpdatedAt:    dbtypes.NewTimestamp(time.Now().UTC()),
			RepositoryID: repositoryID,
			StoragePath:  pathValue,
		}); err != nil {
			return err
		}
		if err := m.enqueuePipelineTx(ctx, tx, created, validation.AssetType, observationToken); err != nil {
			return err
		}
		materialized = created
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("materialize indexed discovery: %w", err)
	}
	if materialized != nil {
		m.audit(repository.Path).Operation("asset.materialize.discovery",
			zap.String("repository_id", repositoryID.String()),
			zap.String("asset_id", materialized.AssetID.String()),
			zap.String("storage_path", repositoryPath.String()),
		)
	}
	return materialized, nil
}
