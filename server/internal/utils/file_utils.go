package utils

import (
	"io/fs"
	"path/filepath"
	"server/internal/db/dbtypes"
	"server/internal/utils/file"
)

// ListImagesInDir scans the given directory recursively and returns
// a slice of file paths that have image file extensions.
// Uses the centralized file validator for consistency.
func ListImagesInDir(dir string) ([]string, error) {
	var images []string
	validator := file.NewValidator()

	// Get all supported photo extensions (including RAW formats)
	supportedExts := validator.GetSupportedExtensionsByType(dbtypes.AssetTypePhoto)
	supported := make(map[string]struct{}, len(supportedExts))
	for _, ext := range supportedExts {
		supported[ext] = struct{}{}
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Propagate errors from WalkDir
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Use validator to check if file is a supported image
		if validator.IsSupportedExtension(filepath.Ext(d.Name())) {
			assetType, ok := validator.GetAssetTypeByExtension(filepath.Ext(d.Name()))
			if ok && assetType == dbtypes.AssetTypePhoto {
				images = append(images, path)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}
