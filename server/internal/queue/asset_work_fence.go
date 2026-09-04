package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

var (
	// ErrAssetWorkStale marks durable work whose Asset content revision or last
	// active Location disappeared after the job was accepted. Retrying cannot
	// make that historical job current again, so workers must complete it as a
	// no-op instead of feeding River's exponential backoff.
	ErrAssetWorkStale = errors.New("asset work revision is stale")

	// ErrDerivedAssetNotReady marks a current Asset whose upstream derived file
	// has not landed yet. This is dependency ordering, not a failed attempt;
	// workers snooze briefly without consuming the retry budget.
	ErrDerivedAssetNotReady = errors.New("derived asset is not ready")
)

type assetWorkValidator interface {
	ValidateAssetWork(context.Context, uuid.UUID, uuid.UUID) error
}

type AssetWorkReader interface {
	GetAssetByID(context.Context, uuid.UUID) (repo.Asset, error)
	GetPreferredActiveAssetOccurrence(context.Context, uuid.UUID) (repo.ActiveAssetOccurrence, error)
}

func validateCurrentAssetWork(
	ctx context.Context,
	reader AssetWorkReader,
	assetID uuid.UUID,
	expectedContentID uuid.UUID,
) (repo.Asset, error) {
	if reader == nil {
		return repo.Asset{}, fmt.Errorf("asset work validator has no catalog queries")
	}
	asset, err := reader.GetAssetByID(ctx, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return repo.Asset{}, ErrAssetWorkStale
	}
	if err != nil {
		return repo.Asset{}, fmt.Errorf("load asset work revision: %w", err)
	}
	if expectedContentID != uuid.Nil && asset.ContentID != expectedContentID {
		return repo.Asset{}, ErrAssetWorkStale
	}
	if _, err := reader.GetPreferredActiveAssetOccurrence(ctx, assetID); errors.Is(err, sql.ErrNoRows) {
		return repo.Asset{}, ErrAssetWorkStale
	} else if err != nil {
		return repo.Asset{}, fmt.Errorf("validate active asset occurrence: %w", err)
	}
	return asset, nil
}

// validateLoaderAssetWork keeps test doubles small while making every runtime
// DBMLImageLoader-backed worker revision-fenced both before inference and
// immediately before persistence.
func validateLoaderAssetWork(
	ctx context.Context,
	loader MLImageLoader,
	assetID uuid.UUID,
	expectedContentID uuid.UUID,
) (current bool, err error) {
	validator, ok := loader.(assetWorkValidator)
	if !ok {
		return true, nil
	}
	if err := validator.ValidateAssetWork(ctx, assetID, expectedContentID); err != nil {
		if errors.Is(err, ErrAssetWorkStale) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
