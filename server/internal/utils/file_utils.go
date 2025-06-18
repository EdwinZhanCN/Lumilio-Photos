package utils

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// ListImagesInDir scans the given directory recursively and returns
// a slice of file paths that have image file extensions.
func ListImagesInDir(dir string) ([]string, error) {
	var images []string
	// Define supported image extensions
	supported := map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".gif":  {},
		".bmp":  {},
		".webp": {},
		".tiff": {},
		".tif":  {},
		".heic": {},
	}

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Propagate errors from WalkDir
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := supported[ext]; ok {
			images = append(images, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return images, nil
}
