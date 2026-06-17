package repo

import (
	"fmt"

	"server/internal/db/dbtypes"
)

// SetPhotoMetadata sets the photo-specific metadata on the asset.
// Passing a nil meta clears any existing specific metadata.
func (a *Asset) SetPhotoMetadata(meta *dbtypes.PhotoSpecificMetadata) error {
	if a == nil {
		return fmt.Errorf("asset is nil")
	}

	if meta == nil {
		a.SpecificMetadata = nil
		return nil
	}

	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return fmt.Errorf("marshal photo metadata: %w", err)
	}

	a.SpecificMetadata = sm
	return nil
}
