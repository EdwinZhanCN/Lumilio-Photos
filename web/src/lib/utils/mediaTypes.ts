/**
 * Utility functions for media type detection and classification
 */

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
