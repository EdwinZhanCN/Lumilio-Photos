package file

import (
	"server/internal/models"
	"strings"
)

// determineAssetType 根据 HTTP Header 中的 Content-Type 决定资源类型
func DetermineAssetType(contentType string) models.AssetType {
	ct := strings.ToLower(strings.TrimSpace(contentType))

	switch {
	case strings.HasPrefix(ct, "image/"):
		return models.AssetTypePhoto
	case strings.HasPrefix(ct, "video/"):
		return models.AssetTypeVideo
	case strings.HasPrefix(ct, "audio/"):
		return models.AssetTypeAudio
	default:
		// TODO: error handeling
		return models.AssetTypePhoto
	}
}
