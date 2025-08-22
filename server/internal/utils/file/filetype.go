package file

import (
	"server/internal/db/dbtypes"
	"strings"
)

// determineAssetType 根据 HTTP Header 中的 Content-Type 决定资源类型
func DetermineAssetType(contentType string) dbtypes.AssetType {
	ct := strings.ToLower(strings.TrimSpace(contentType))

	switch {
	case strings.HasPrefix(ct, "image/"):
		return dbtypes.AssetTypePhoto
	case strings.HasPrefix(ct, "video/"):
		return dbtypes.AssetTypeVideo
	case strings.HasPrefix(ct, "audio/"):
		return dbtypes.AssetTypeAudio
	default:
		// TODO: error handeling
		return dbtypes.AssetTypePhoto
	}
}
