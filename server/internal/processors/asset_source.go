package processors

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/google/uuid"
)

var ErrAssetSourceStale = errors.New("asset source observation is stale")

type currentAssetSource struct {
	asset       *repo.Asset
	repository  repo.Repository
	files       *storage.RepositoryFS
	path        storage.RepositoryPath
	localPath   string
	observation storage.FileObservation
}

func (source *currentAssetSource) Close() error {
	if source == nil || source.files == nil {
		return nil
	}
	return source.files.Close()
}

func (ap *AssetProcessor) resolveCurrentAssetSource(ctx context.Context, assetID uuid.UUID, expectedToken, expectedHash string) (*currentAssetSource, error) {
	asset, repository, err := ap.loadAssetAndRepo(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.IsDeleted || asset.StoragePath == nil || strings.TrimSpace(*asset.StoragePath) == "" {
		return nil, ErrAssetSourceStale
	}
	repositoryPath, err := storage.ParseUserMediaPath(*asset.StoragePath)
	if err != nil {
		return nil, err
	}
	indexed, err := ap.queries.GetRepositoryFileIndexEntry(ctx, repo.GetRepositoryFileIndexEntryParams{
		RepositoryID: repository.RepoID,
		StoragePath:  repositoryPath.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("load current asset file index: %w", err)
	}
	if !indexed.AssetID.Valid || indexed.AssetID.UUID != assetID || indexed.State != "present" {
		return nil, ErrAssetSourceStale
	}
	repositoryFS, err := ap.files.Open(repository)
	if err != nil {
		return nil, err
	}
	observation, err := repositoryFS.InspectMedia(ctx, repositoryPath, storage.HashNone)
	if err != nil {
		_ = repositoryFS.Close()
		return nil, err
	}
	if observation.ObservationToken != indexed.ObservationToken {
		_ = repositoryFS.Close()
		return nil, ErrAssetSourceStale
	}
	if expectedToken != "" && expectedToken != observation.ObservationToken {
		movedSameContent := expectedHash != "" && indexed.ContentHash != nil &&
			strings.EqualFold(*indexed.ContentHash, expectedHash) && strings.EqualFold(asset.ContentHash, expectedHash)
		if !movedSameContent {
			_ = repositoryFS.Close()
			return nil, ErrAssetSourceStale
		}
	}
	localPath, err := repositoryFS.LocalMediaPath(repositoryPath)
	if err != nil {
		_ = repositoryFS.Close()
		return nil, err
	}
	return &currentAssetSource{asset: asset, repository: repository, files: repositoryFS, path: repositoryPath, localPath: localPath, observation: observation}, nil
}
