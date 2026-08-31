package processors

import (
	"context"
	"errors"
	"fmt"

	"server/internal/db/repo"
	"server/internal/storage"
	"server/internal/storage/roe/locations"

	"github.com/google/uuid"
)

var ErrAssetSourceStale = errors.New("asset source observation is stale")

type currentAssetSource struct {
	asset       *repo.Asset
	content     repo.ContentObject
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

// resolveCurrentAssetSource verifies immutable logical content identity, then
// leases and opens one active Location immediately before native media access.
// No queue fact or Asset row carries a path or repository context.
func (ap *AssetProcessor) resolveCurrentAssetSource(ctx context.Context, assetID, expectedContentID uuid.UUID) (*currentAssetSource, error) {
	if ap == nil || ap.locationResolver == nil {
		return nil, locations.ErrAssetUnavailable
	}
	asset, err := ap.reader.GetAssetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.IsDeleted || (expectedContentID != uuid.Nil && asset.ContentID != expectedContentID) {
		return nil, ErrAssetSourceStale
	}
	content, err := ap.reader.GetContentObjectByID(ctx, asset.ContentID)
	if err != nil {
		return nil, err
	}
	opened, localPath, err := ap.locationResolver.LocalAssetPath(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if opened.File != nil {
		if err := opened.File.Close(); err != nil {
			_ = opened.Close()
			return nil, err
		}
		opened.File = nil
	}
	observation, err := opened.Repository.InspectMedia(ctx, opened.Path, storage.HashNone)
	if err != nil {
		_ = opened.Close()
		return nil, err
	}
	if observation.Size != content.FileSize || opened.Node.StabilityToken == nil ||
		observation.ObservationToken != *opened.Node.StabilityToken {
		_ = opened.Close()
		return nil, ErrAssetSourceStale
	}
	source := &currentAssetSource{
		asset: &asset, content: content, repository: opened.Catalog,
		files: opened.Repository, path: opened.Path, localPath: localPath,
		observation: observation,
	}
	opened.Repository = nil
	if err := opened.Close(); err != nil {
		_ = source.Close()
		return nil, fmt.Errorf("release source capability: %w", err)
	}
	return source, nil
}
