/**
 * File validation utilities
 * Matches backend validation logic from server/internal/utils/file/validator.go
 */

import {
  acceptFileExtensions,
  getFileExtension,
  isSupportedExtension,
  getAssetTypeFromExtension,
} from "./accept-file-extensions";

/**
 * Supported MIME types for photos
 */
const supportedPhotoMimeTypes = [
  "image/jpeg",
  "image/jpg",
  "image/png",
  "image/webp",
  "image/gif",
  "image/bmp",
  "image/tiff",
  "image/heic",
  "image/heif",
  "image/x-canon-cr2",
  "image/x-canon-cr3",
  "image/x-nikon-nef",
  "image/x-sony-arw",
  "image/x-adobe-dng",
  "image/x-olympus-orf",
  "image/x-panasonic-rw2",
  "image/x-fuji-raf",
] as const;

/**
 * Supported MIME types for videos
 */
const supportedVideoMimeTypes = [
  "video/mp4",
  "video/quicktime",
  "video/x-msvideo",
  "video/x-matroska",
  "video/webm",
  "video/x-flv",
  "video/x-ms-wmv",
  "video/mpeg",
  "video/3gpp",
  "video/ogg",
] as const;

/**
 * Supported MIME types for audio
 */
const supportedAudioMimeTypes = [
  "audio/mpeg",
  "audio/mp3",
  "audio/aac",
  "audio/mp4",
  "audio/x-m4a",
  "audio/flac",
  "audio/wav",
  "audio/x-wav",
  "audio/ogg",
  "audio/x-aiff",
  "audio/x-ms-wma",
  "audio/opus",
] as const;

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
 * Check if MIME type is valid for given asset type
 */
const isValidMimeType = (
  mimeType: string,
  assetType: "photo" | "video" | "audio" | "unknown",
): boolean => {
  const mime = mimeType.toLowerCase().trim();

  // Check exact match
  if (assetType === "photo" && supportedPhotoMimeTypes.includes(mime as any)) {
    return true;
  }
  if (assetType === "video" && supportedVideoMimeTypes.includes(mime as any)) {
    return true;
  }
  if (assetType === "audio" && supportedAudioMimeTypes.includes(mime as any)) {
    return true;
  }

  // Check prefix match
  if (assetType === "photo" && mime.startsWith("image/")) {
    return true;
  }
  if (assetType === "video" && mime.startsWith("video/")) {
    return true;
  }
  if (assetType === "audio" && mime.startsWith("audio/")) {
    return true;
  }

  return false;
};

/**
 * Validate a file based on filename and MIME type
 * Matches backend validation logic
 */
export const validateFile = (file: File): FileValidationResult => {
  const extension = getFileExtension(file.name);
  const mimeType = file.type.toLowerCase().trim();

  // Check if extension is empty
  if (!extension) {
    return {
      valid: false,
      assetType: "unknown",
      extension: "",
      mimeType,
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

  // Check if it's a RAW format
  const isRAW =
    assetType === "photo" &&
    [
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
    ].includes(extension);

  // Validate MIME type if provided and not the generic octet-stream
  // This matches backend logic: skip MIME validation for application/octet-stream
  if (mimeType && mimeType !== "application/octet-stream") {
    if (!isValidMimeType(mimeType, assetType)) {
      return {
        valid: false,
        assetType,
        extension,
        mimeType,
        isRAW,
        errorReason: `MIME type '${mimeType}' does not match file extension '${extension}'`,
      };
    }
  }

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
export const getValidationErrorMessage = (
  result: FileValidationResult,
): string => {
  if (result.valid) {
    return "";
  }

  return (
    result.errorReason ||
    "File validation failed for unknown reason. Please check the file format."
  );
};

// Default export for backward compatibility
export default isValidFileType;
