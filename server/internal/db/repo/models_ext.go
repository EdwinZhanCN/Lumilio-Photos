package repo

import (
	"fmt"
	"path/filepath"
	"strings"

	"server/internal/db/dbtypes"
)

// IsPrimaryRepository returns true if the repository name or the last
// path component is "primary" (case-insensitive).
func IsPrimaryRepository(name, path string) bool {
	if strings.EqualFold(strings.TrimSpace(name), "primary") {
		return true
	}

	base := filepath.Base(strings.TrimSpace(path))
	return strings.EqualFold(base, "primary")
}

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
