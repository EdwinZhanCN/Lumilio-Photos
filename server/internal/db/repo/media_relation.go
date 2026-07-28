package repo

import (
	"path/filepath"
	"strings"

	"server/internal/utils/file"
)

// InitialMediaRelation classifies the media_item_assets relation assigned when
// an asset first becomes a component of a logical media item:
//
//	RAW file    → raw_original
//	JPEG file   → jpeg_original
//	other photo → original
//	video/audio → original
//
// This is the single place where component relations are derived from file
// facts. Materializer (ingest), the metadata reconciler, and the stack service
// must all go through this function instead of re-deriving RAW/JPEG rules from
// extensions on their own. Later pipeline stages may reclassify a component
// (edited_version, live_photo_*, alternative) but never back to a less precise
// value.
func InitialMediaRelation(validation *file.ValidationResult, filename string) StackRelation {
	isRAW := file.IsRAWFile(filename)
	mimeType := ""
	if validation != nil {
		isRAW = isRAW || validation.IsRAW
		mimeType = validation.MimeType
	}
	if isRAW {
		return StackRelationRawOriginal
	}
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".jpg" || ext == ".jpeg" || strings.EqualFold(strings.TrimSpace(mimeType), "image/jpeg") {
		return StackRelationJpegOriginal
	}
	return StackRelationOriginal
}
