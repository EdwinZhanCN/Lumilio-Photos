package processors

import (
	"time"

	"server/internal/utils/exif"
)

// Thumbnail target sizes reused across photo and video thumbnail generation.
var thumbnailSizes = map[string][2]int{
	"small":  {400, 400},
	"medium": {800, 800},
	"large":  {1920, 1920},
}

// createEXIFConfig centralizes EXIF extraction settings for photos.
func (ap *AssetProcessor) createEXIFConfig() *exif.Config {
	return &exif.Config{
		ExifToolPath: ap.toolsConfig.ExifToolCommand(),
		MaxFileSize:  2 * 1024 * 1024 * 1024, // 2GB
		Timeout:      60 * time.Second,
		BufferSize:   128 * 1024,
		FastMode:     false, // Full EXIF for photos
		IncludeRaw:   true,
	}
}
