/**
 * Utility functions for media type detection and classification
 */

import { isPhotoMetadata, type Asset } from "@/lib/http-commons";

/**
 * Determines if an asset is a video based on MIME type or legacy type
 */
export const isVideo = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("video/");
  }
  return asset.type === "VIDEO";
};

/**
 * Determines if an asset is audio based on MIME type or legacy type
 */
export const isAudio = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("audio/");
  }
  return asset.type === "AUDIO";
};

/**
 * Determines if an asset is a photo/image based on MIME type or legacy type
 */
export const isPhoto = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("image/");
  }
  return asset.type === "PHOTO";
};

/**
 * Determines if a photo asset is RAW.
 * Supports both legacy `asset.isRAW` and schema-driven `specific_metadata.is_raw`.
 */
export const isRawPhoto = (asset: Asset): boolean => {
  const legacyRaw = (asset as Asset & { isRAW?: boolean }).isRAW === true;
  const metadataRaw =
    isPhotoMetadata(asset.type, asset.specific_metadata) &&
    asset.specific_metadata?.is_raw === true;

  return legacyRaw || metadataRaw;
};

/**
 * Browser-side export conversion is only available for non-RAW photo assets.
 */
export const isBrowserExportSupported = (asset: Asset): boolean => {
  return isPhoto(asset) && !isRawPhoto(asset);
};

/**
 * Gets the media type category for an asset
 */
export const getMediaType = (
  asset: Asset,
): "video" | "audio" | "photo" | "unknown" => {
  if (isVideo(asset)) return "video";
  if (isAudio(asset)) return "audio";
  if (isPhoto(asset)) return "photo";
  return "unknown";
};

/**
 * Formats duration in seconds to MM:SS format
 */
export const formatDuration = (duration?: number): string => {
  if (!duration || duration <= 0) return "";
  const minutes = Math.floor(duration / 60);
  const seconds = Math.floor(duration % 60);
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
};

/**
 * Gets the appropriate ARIA label for an asset
 */
export const getAssetAriaLabel = (
  asset: Asset,
  includeDuration = true,
): string => {
  const filename = asset.original_filename || "Asset";
  const mediaType = getMediaType(asset);
  const duration = asset.duration;

  let label = `${mediaType.charAt(0).toUpperCase() + mediaType.slice(1)}: ${filename}`;

  if (includeDuration && duration && (isVideo(asset) || isAudio(asset))) {
    label += `, ${formatDuration(duration)} duration`;
  }

  return label;
};

/**
 * Gets human-readable file type description
 */
export const getFileTypeDescription = (asset: Asset): string => {
  const mediaType = getMediaType(asset);

  if (asset.mime_type) {
    // Convert MIME type to readable format
    const [category, subtype] = asset.mime_type.split("/");
    if (subtype) {
      return `${subtype.toUpperCase()} ${category}`;
    }
    return asset.mime_type;
  }

  return asset.type || mediaType;
};
