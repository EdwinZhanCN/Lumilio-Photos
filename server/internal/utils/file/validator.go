package file

import (
	"fmt"
	"path/filepath"
	"server/internal/db/dbtypes"
	"strings"
)

// SupportedFormats contains all file formats supported by the backend
type SupportedFormats struct {
	Photos map[string]bool
	Videos map[string]bool
	Audios map[string]bool
	RAW    map[string]bool
}

var (
	// Supported photo/image extensions
	supportedPhotoExts = map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".webp": true,
		".gif":  true,
		".bmp":  true,
		".tiff": true,
		".tif":  true,
		".heic": true,
		".heif": true,
	}

	// Supported RAW photo extensions
	supportedRAWExts = map[string]bool{
		".cr2": true, // Canon
		".cr3": true, // Canon
		".nef": true, // Nikon
		".arw": true, // Sony
		".dng": true, // Adobe Digital Negative
		".orf": true, // Olympus
		".rw2": true, // Panasonic
		".pef": true, // Pentax
		".raf": true, // Fujifilm
		".mrw": true, // Minolta/Konica Minolta
		".srw": true, // Samsung
		".rwl": true, // Leica
		".x3f": true, // Sigma
	}

	// Supported video extensions
	supportedVideoExts = map[string]bool{
		".mp4":  true,
		".mov":  true,
		".avi":  true,
		".mkv":  true,
		".webm": true,
		".flv":  true,
		".wmv":  true,
		".m4v":  true,
		".3gp":  true,
		".mpg":  true,
		".mpeg": true,
		".m2ts": true,
		".mts":  true,
		".ogv":  true,
	}

	// Supported audio extensions
	supportedAudioExts = map[string]bool{
		".mp3":  true,
		".aac":  true,
		".m4a":  true,
		".flac": true,
		".wav":  true,
		".ogg":  true,
		".aiff": true,
		".wma":  true,
		".opus": true,
		".oga":  true,
	}

	// MIME type to asset type mapping
	mimeTypeToAssetType = map[string]dbtypes.AssetType{
		// Images
		"image/jpeg":        dbtypes.AssetTypePhoto,
		"image/jpg":         dbtypes.AssetTypePhoto,
		"image/png":         dbtypes.AssetTypePhoto,
		"image/webp":        dbtypes.AssetTypePhoto,
		"image/gif":         dbtypes.AssetTypePhoto,
		"image/bmp":         dbtypes.AssetTypePhoto,
		"image/tiff":        dbtypes.AssetTypePhoto,
		"image/heic":        dbtypes.AssetTypePhoto,
		"image/heif":        dbtypes.AssetTypePhoto,
		"image/x-canon-cr2": dbtypes.AssetTypePhoto,
		"image/x-canon-cr3": dbtypes.AssetTypePhoto,
		"image/x-nikon-nef": dbtypes.AssetTypePhoto,
		"image/x-sony-arw":  dbtypes.AssetTypePhoto,
		"image/x-adobe-dng": dbtypes.AssetTypePhoto,

		// Videos
		"video/mp4":        dbtypes.AssetTypeVideo,
		"video/quicktime":  dbtypes.AssetTypeVideo,
		"video/x-msvideo":  dbtypes.AssetTypeVideo,
		"video/x-matroska": dbtypes.AssetTypeVideo,
		"video/webm":       dbtypes.AssetTypeVideo,
		"video/x-flv":      dbtypes.AssetTypeVideo,
		"video/x-ms-wmv":   dbtypes.AssetTypeVideo,
		"video/mpeg":       dbtypes.AssetTypeVideo,
		"video/3gpp":       dbtypes.AssetTypeVideo,
		"video/ogg":        dbtypes.AssetTypeVideo,

		// Audio
		"audio/mpeg":     dbtypes.AssetTypeAudio,
		"audio/mp3":      dbtypes.AssetTypeAudio,
		"audio/aac":      dbtypes.AssetTypeAudio,
		"audio/mp4":      dbtypes.AssetTypeAudio,
		"audio/x-m4a":    dbtypes.AssetTypeAudio,
		"audio/flac":     dbtypes.AssetTypeAudio,
		"audio/wav":      dbtypes.AssetTypeAudio,
		"audio/x-wav":    dbtypes.AssetTypeAudio,
		"audio/ogg":      dbtypes.AssetTypeAudio,
		"audio/x-aiff":   dbtypes.AssetTypeAudio,
		"audio/x-ms-wma": dbtypes.AssetTypeAudio,
		"audio/opus":     dbtypes.AssetTypeAudio,
	}

	canonicalMimeByExtension = map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		".gif":  "image/gif",
		".bmp":  "image/bmp",
		".tiff": "image/tiff",
		".tif":  "image/tiff",
		".heic": "image/heic",
		".heif": "image/heif",
		".cr2":  "image/x-canon-cr2",
		".cr3":  "image/x-canon-cr3",
		".nef":  "image/x-nikon-nef",
		".arw":  "image/x-sony-arw",
		".dng":  "image/x-adobe-dng",
		".orf":  "image/x-olympus-orf",
		".rw2":  "image/x-panasonic-rw2",
		".pef":  "image/x-pentax-pef",
		".raf":  "image/x-fuji-raf",
		".mrw":  "image/x-minolta-mrw",
		".srw":  "image/x-samsung-srw",
		".rwl":  "image/x-leica-rwl",
		".x3f":  "image/x-sigma-x3f",
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".flv":  "video/x-flv",
		".wmv":  "video/x-ms-wmv",
		".m4v":  "video/mp4",
		".3gp":  "video/3gpp",
		".mpg":  "video/mpeg",
		".mpeg": "video/mpeg",
		".m2ts": "video/mp2t",
		".mts":  "video/mp2t",
		".ogv":  "video/ogg",
		".mp3":  "audio/mpeg",
		".aac":  "audio/aac",
		".m4a":  "audio/mp4",
		".flac": "audio/flac",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".aiff": "audio/x-aiff",
		".wma":  "audio/x-ms-wma",
		".opus": "audio/opus",
		".oga":  "audio/ogg",
	}

	rawFormatByExtension = map[string]string{
		".cr2": "CR2",
		".cr3": "CR3",
		".nef": "NEF",
		".arw": "ARW",
		".dng": "DNG",
		".orf": "ORF",
		".rw2": "RW2",
		".pef": "PEF",
		".raf": "RAF",
		".mrw": "MRW",
		".srw": "SRW",
		".rwl": "RWL",
		".x3f": "X3F",
	}
)

// Validator handles file validation logic
type Validator struct{}

// NewValidator creates a new file validator
func NewValidator() *Validator {
	return &Validator{}
}

// MediaInfo is the canonical media description derived from the filename.
type MediaInfo struct {
	AssetType dbtypes.AssetType
	Extension string
	MimeType  string
	IsRAW     bool
	RawFormat string
}

// ValidationResult contains the result of file validation
type ValidationResult struct {
	Valid       bool
	AssetType   dbtypes.AssetType
	Extension   string
	MimeType    string
	IsRAW       bool
	RawFormat   string
	ErrorReason string
}

// ResolveMedia returns the canonical media info for a supported filename.
func (v *Validator) ResolveMedia(filename string) (*MediaInfo, error) {
	ext := normalizeExtension(filepath.Ext(filename))
	if ext == "" {
		return nil, fmt.Errorf("file has no extension")
	}

	assetType, isSupported := v.GetAssetTypeByExtension(ext)
	if !isSupported {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	mimeType, exists := canonicalMimeByExtension[ext]
	if !exists {
		return nil, fmt.Errorf("no canonical MIME type configured for extension: %s", ext)
	}

	info := &MediaInfo{
		AssetType: assetType,
		Extension: ext,
		MimeType:  mimeType,
		IsRAW:     supportedRAWExts[ext],
		RawFormat: rawFormatByExtension[ext],
	}

	return info, nil
}

// ValidateFile validates a file based on filename only.
func (v *Validator) ValidateFile(filename, contentType string) *ValidationResult {
	info, err := v.ResolveMedia(filename)
	if err != nil {
		return &ValidationResult{
			Valid:       false,
			Extension:   normalizeExtension(filepath.Ext(filename)),
			ErrorReason: err.Error(),
		}
	}

	return &ValidationResult{
		Valid:     true,
		AssetType: info.AssetType,
		Extension: info.Extension,
		MimeType:  info.MimeType,
		IsRAW:     info.IsRAW,
		RawFormat: info.RawFormat,
	}
}

// IsSupported checks if a file extension is supported
func (v *Validator) IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return supportedPhotoExts[ext] ||
		supportedRAWExts[ext] ||
		supportedVideoExts[ext] ||
		supportedAudioExts[ext]
}

// IsSupportedExtension checks if an extension is supported
func (v *Validator) IsSupportedExtension(ext string) bool {
	ext = normalizeExtension(ext)
	return supportedPhotoExts[ext] ||
		supportedRAWExts[ext] ||
		supportedVideoExts[ext] ||
		supportedAudioExts[ext]
}

// GetAssetTypeByExtension determines asset type from file extension
func (v *Validator) GetAssetTypeByExtension(ext string) (dbtypes.AssetType, bool) {
	ext = normalizeExtension(ext)

	if supportedPhotoExts[ext] || supportedRAWExts[ext] {
		return dbtypes.AssetTypePhoto, true
	}
	if supportedVideoExts[ext] {
		return dbtypes.AssetTypeVideo, true
	}
	if supportedAudioExts[ext] {
		return dbtypes.AssetTypeAudio, true
	}

	return "", false
}

// GetAssetTypeByMimeType determines asset type from MIME type
func (v *Validator) GetAssetTypeByMimeType(mimeType string) (dbtypes.AssetType, bool) {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))

	// Try exact match first
	if assetType, exists := mimeTypeToAssetType[mimeType]; exists {
		return assetType, true
	}

	// Fallback to prefix matching
	if strings.HasPrefix(mimeType, "image/") {
		return dbtypes.AssetTypePhoto, true
	}
	if strings.HasPrefix(mimeType, "video/") {
		return dbtypes.AssetTypeVideo, true
	}
	if strings.HasPrefix(mimeType, "audio/") {
		return dbtypes.AssetTypeAudio, true
	}

	return "", false
}

// DetermineAssetType determines asset type from both filename and content type
// This is the main function that should be used throughout the application
func (v *Validator) DetermineAssetType(filename, contentType string) dbtypes.AssetType {
	// Prefer extension-based detection (more reliable)
	if filename != "" {
		if assetType, ok := v.GetAssetTypeByExtension(filepath.Ext(filename)); ok {
			return assetType
		}
	}

	// Fallback to MIME type
	if contentType != "" {
		if assetType, ok := v.GetAssetTypeByMimeType(contentType); ok {
			return assetType
		}
	}

	// Default fallback to photo
	return dbtypes.AssetTypePhoto
}

// IsValidMimeType checks if a MIME type is valid for the given asset type
func (v *Validator) IsValidMimeType(mimeType string, assetType dbtypes.AssetType) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))

	// Check exact match
	if mappedType, exists := mimeTypeToAssetType[mimeType]; exists {
		return mappedType == assetType
	}

	// Check prefix match
	switch assetType {
	case dbtypes.AssetTypePhoto:
		return strings.HasPrefix(mimeType, "image/")
	case dbtypes.AssetTypeVideo:
		return strings.HasPrefix(mimeType, "video/")
	case dbtypes.AssetTypeAudio:
		return strings.HasPrefix(mimeType, "audio/")
	}

	return false
}

// IsRAWFile checks if a file is a RAW camera format
func (v *Validator) IsRAWFile(filename string) bool {
	ext := normalizeExtension(filepath.Ext(filename))
	return supportedRAWExts[ext]
}

// GetMimeTypeFromExtension returns the MIME type for a given extension
func (v *Validator) GetMimeTypeFromExtension(ext string) string {
	ext = normalizeExtension(ext)
	if mimeType, exists := canonicalMimeByExtension[ext]; exists {
		return mimeType
	}

	return "application/octet-stream"
}

// GetSupportedFormats returns all supported formats organized by type
func (v *Validator) GetSupportedFormats() SupportedFormats {
	return SupportedFormats{
		Photos: copyMap(supportedPhotoExts),
		Videos: copyMap(supportedVideoExts),
		Audios: copyMap(supportedAudioExts),
		RAW:    copyMap(supportedRAWExts),
	}
}

// GetSupportedExtensions returns a flat list of all supported extensions
func (v *Validator) GetSupportedExtensions() []string {
	var extensions []string

	for ext := range supportedPhotoExts {
		extensions = append(extensions, ext)
	}
	for ext := range supportedRAWExts {
		extensions = append(extensions, ext)
	}
	for ext := range supportedVideoExts {
		extensions = append(extensions, ext)
	}
	for ext := range supportedAudioExts {
		extensions = append(extensions, ext)
	}

	return extensions
}

// GetSupportedExtensionsByType returns supported extensions for a specific asset type
func (v *Validator) GetSupportedExtensionsByType(assetType dbtypes.AssetType) []string {
	var extensions []string

	switch assetType {
	case dbtypes.AssetTypePhoto:
		for ext := range supportedPhotoExts {
			extensions = append(extensions, ext)
		}
		for ext := range supportedRAWExts {
			extensions = append(extensions, ext)
		}
	case dbtypes.AssetTypeVideo:
		for ext := range supportedVideoExts {
			extensions = append(extensions, ext)
		}
	case dbtypes.AssetTypeAudio:
		for ext := range supportedAudioExts {
			extensions = append(extensions, ext)
		}
	}

	return extensions
}

// Helper function to copy a map
func copyMap(m map[string]bool) map[string]bool {
	result := make(map[string]bool, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func normalizeExtension(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// FormatValidationError creates a user-friendly validation error message
func (v *Validator) FormatValidationError(result *ValidationResult) string {
	if result.Valid {
		return ""
	}

	if result.ErrorReason != "" {
		return result.ErrorReason
	}

	return "file validation failed for unknown reason"
}
