package file

import (
	"server/internal/db/dbtypes"
)

var defaultValidator = NewValidator()

// DetermineAssetType determines the asset type based on content type and filename
// This function is kept for backward compatibility but now uses the centralized validator
func DetermineAssetType(contentType string) dbtypes.AssetType {
	assetType, ok := defaultValidator.GetAssetTypeByMimeType(contentType)
	if ok {
		return assetType
	}
	// Default fallback to photo (maintains backward compatibility)
	return dbtypes.AssetTypePhoto
}

// DetermineAssetTypeWithFilename determines asset type from both filename and content type
// This is the recommended function to use as it's more accurate
func DetermineAssetTypeWithFilename(filename, contentType string) dbtypes.AssetType {
	return defaultValidator.DetermineAssetType(filename, contentType)
}

// ValidateFile validates a file and returns detailed validation result
func ValidateFile(filename, contentType string) *ValidationResult {
	return defaultValidator.ValidateFile(filename, contentType)
}

// IsSupported checks if a filename has a supported extension
func IsSupported(filename string) bool {
	return defaultValidator.IsSupported(filename)
}

// IsSupportedExtension checks if an extension is supported
func IsSupportedExtension(ext string) bool {
	return defaultValidator.IsSupportedExtension(ext)
}

// IsRAWFile checks if a file is a RAW camera format
func IsRAWFile(filename string) bool {
	return defaultValidator.IsRAWFile(filename)
}

// GetSupportedExtensions returns all supported file extensions
func GetSupportedExtensions() []string {
	return defaultValidator.GetSupportedExtensions()
}

// GetSupportedExtensionsByType returns supported extensions for a specific asset type
func GetSupportedExtensionsByType(assetType dbtypes.AssetType) []string {
	return defaultValidator.GetSupportedExtensionsByType(assetType)
}
