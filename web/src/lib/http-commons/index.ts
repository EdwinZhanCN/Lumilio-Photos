/**
 * HTTP Commons - Central export for API-related types and utilities
 */

// Export generated schema types
export type { paths, components, webhooks } from "./schema";
import type { components } from "./schema";

// Export commonly used types from generated schema
export type Asset = components["schemas"]["dto.AssetDTO"];
export type AssetDTO = components["schemas"]["dto.AssetDTO"];

// Export metadata types from generated schema
export type PhotoSpecificMetadata =
  components["schemas"]["dbtypes.PhotoSpecificMetadata"];
export type VideoSpecificMetadata =
  components["schemas"]["dbtypes.VideoSpecificMetadata"];
export type AudioSpecificMetadata =
  components["schemas"]["dbtypes.AudioSpecificMetadata"];

// Union type for all specific metadata types
export type SpecificMetadata =
  | PhotoSpecificMetadata
  | VideoSpecificMetadata
  | AudioSpecificMetadata;

// Type guards for metadata
export function isPhotoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is PhotoSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "PHOTO";
}

export function isVideoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is VideoSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "VIDEO";
}

export function isAudioMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): metadata is AudioSpecificMetadata {
  if (!type && !metadata) return false;
  return type === "AUDIO";
}

// Safe cast functions
export function asPhotoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): PhotoSpecificMetadata | undefined {
  return isPhotoMetadata(type, metadata) ? metadata : undefined;
}

export function asVideoMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): VideoSpecificMetadata | undefined {
  return isVideoMetadata(type, metadata) ? metadata : undefined;
}

export function asAudioMetadata(
  type: string | undefined,
  metadata: SpecificMetadata | undefined,
): AudioSpecificMetadata | undefined {
  return isAudioMetadata(type, metadata) ? metadata : undefined;
}

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

// Re-export token management functions and client
export { getToken, getRefreshToken, saveToken, removeToken } from "./api";
export { default as client } from "./client";
