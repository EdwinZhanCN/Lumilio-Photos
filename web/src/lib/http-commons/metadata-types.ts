/**
 * Frontend type definitions for asset-specific metadata
 * These types mirror the Go backend types but are designed for TypeScript
 */

/**
 * Species prediction metadata for photos
 */
export interface SpeciesPredictionMeta {
  label: string;
  score: number;
}

/**
 * Photo-specific metadata
 * Contains EXIF data and other photo-related information
 */
export interface PhotoSpecificMetadata {
  taken_time?: string;
  camera_model?: string;
  lens_model?: string;
  exposure_time?: string;
  f_number?: number;
  focal_length?: number;
  iso_speed?: number;
  exposure?: number;
  dimensions?: string;
  resolution?: string;
  gps_latitude?: number;
  gps_longitude?: number;
  description?: string;
  species_prediction?: SpeciesPredictionMeta[];
  is_raw?: boolean;
}

/**
 * Video-specific metadata
 * Contains codec, bitrate, and other video-related information
 */
export interface VideoSpecificMetadata {
  codec?: string;
  bitrate?: number;
  frame_rate?: number;
  recorded_time?: string;
  camera_model?: string;
  gps_latitude?: number;
  gps_longitude?: number;
  description?: string;
}

/**
 * Audio-specific metadata
 * Contains codec, bitrate, and music metadata
 */
export interface AudioSpecificMetadata {
  codec?: string;
  bitrate?: number;
  sample_rate?: number;
  channels?: number;
  artist?: string;
  album?: string;
  title?: string;
  genre?: string;
  year?: number;
  description?: string;
}

/**
 * Union type for all specific metadata types
 */
export type SpecificMetadata =
  | PhotoSpecificMetadata
  | VideoSpecificMetadata
  | AudioSpecificMetadata
  | Record<string, never>; // For unknown or empty metadata

/**
 * Type guard to check if metadata is PhotoSpecificMetadata
 */
export function isPhotoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is PhotoSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "PHOTO";
}

/**
 * Type guard to check if metadata is VideoSpecificMetadata
 */
export function isVideoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is VideoSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "VIDEO";
}

/**
 * Type guard to check if metadata is AudioSpecificMetadata
 */
export function isAudioMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is AudioSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "AUDIO";
}

/**
 * Safely cast metadata to PhotoSpecificMetadata
 * Returns undefined if the metadata is not photo metadata
 */
export function asPhotoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): PhotoSpecificMetadata | undefined {
  return isPhotoMetadata(type, metadata) ? metadata : undefined;
}

/**
 * Safely cast metadata to VideoSpecificMetadata
 * Returns undefined if the metadata is not video metadata
 */
export function asVideoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): VideoSpecificMetadata | undefined {
  return isVideoMetadata(type, metadata) ? metadata : undefined;
}

/**
 * Safely cast metadata to AudioSpecificMetadata
 * Returns undefined if the metadata is not audio metadata
 */
export function asAudioMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): AudioSpecificMetadata | undefined {
  return isAudioMetadata(type, metadata) ? metadata : undefined;
}

/**
 * Get metadata by asset type
 * This provides type-safe access to metadata based on the asset type string
 */
export function getTypedMetadata(
  assetType: string | undefined,
  metadata: SpecificMetadata | undefined,
):
  | PhotoSpecificMetadata
  | VideoSpecificMetadata
  | AudioSpecificMetadata
  | undefined {
  if (!metadata || !assetType) return undefined;

  const type = assetType.toUpperCase();
  switch (type) {
    case "PHOTO":
      return asPhotoMetadata(assetType, metadata);
    case "VIDEO":
      return asVideoMetadata(assetType, metadata);
    case "AUDIO":
      return asAudioMetadata(assetType, metadata);
    default:
      return undefined;
  }
}
