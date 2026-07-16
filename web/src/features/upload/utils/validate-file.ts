/**
 * Upload file validation utilities
 * Matches backend validation logic from server/internal/utils/file/validator.go
 */

import {
  acceptFileExtensions,
  getFileExtension,
  isSupportedExtension,
  getAssetTypeFromExtension,
  getCanonicalMimeTypeForFilename,
  supportedRAWExtensions,
} from "./accept-file-extensions";

/**
 * Validation result interface
 */
export interface FileValidationResult {
  valid: boolean;
  assetType: "photo" | "video" | "audio" | "unknown";
  extension: string;
  mimeType: string;
  isRAW: boolean;
  errorReason?: string;
}

/**
 * Validate a file based on filename and MIME type
 * Matches backend validation logic: extension is the single source of truth.
 */
export const validateFile = (file: File): FileValidationResult => {
  const extension = getFileExtension(file.name);
  const mimeType = getCanonicalMimeTypeForFilename(file.name);

  // Check if extension is empty
  if (!extension) {
    return {
      valid: false,
      assetType: "unknown",
      extension: "",
      mimeType: "",
      isRAW: false,
      errorReason: "File has no extension",
    };
  }

  // Determine asset type by extension (more reliable)
  const assetType = getAssetTypeFromExtension(file.name);
  if (assetType === "unknown") {
    return {
      valid: false,
      assetType,
      extension,
      mimeType,
      isRAW: false,
      errorReason: `Unsupported file extension: ${extension}`,
    };
  }

  const isRAW =
    assetType === "photo" &&
    supportedRAWExtensions.includes(extension as (typeof supportedRAWExtensions)[number]);

  return {
    valid: true,
    assetType,
    extension,
    mimeType,
    isRAW,
  };
};

/**
 * Simple validation function for backward compatibility
 * @param {File} file - The file to validate
 * @returns {boolean} - True if the file is valid
 */
const isValidFileType = (file: File): boolean => {
  const result = validateFile(file);
  return result.valid;
};

/**
 * Returns an array of supported RAW file extensions
 * @returns {string[]} Array of supported RAW file extensions
 */
export const getSupportedRawExtensions = (): string[] => {
  return [
    ".cr2",
    ".cr3",
    ".nef",
    ".arw",
    ".dng",
    ".orf",
    ".rw2",
    ".pef",
    ".raf",
    ".mrw",
    ".srw",
    ".rwl",
    ".x3f",
  ];
};

/**
 * Returns a string of all supported file extensions for accept attribute
 * @returns {string} Comma-separated string of file extensions
 */
export const getSupportedFileExtensionsString = (): string => {
  return acceptFileExtensions.join(",");
};

/**
 * Check if a file is supported based on extension
 */
export const isFileSupported = (filename: string): boolean => {
  const ext = getFileExtension(filename);
  return isSupportedExtension(ext);
};

/**
 * Get user-friendly error message from validation result
 */
export const getValidationErrorMessage = (result: FileValidationResult): string => {
  if (result.valid) {
    return "";
  }

  return (
    result.errorReason || "File validation failed for unknown reason. Please check the file format."
  );
};

// Default export for backward compatibility
export default isValidFileType;
