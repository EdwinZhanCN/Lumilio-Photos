/**
 * HTTP Commons - Central export for API-related types and utilities
 */

// Export generated schema types
export type { paths, components, webhooks } from "./schema";

// Export metadata types
export type {
  PhotoSpecificMetadata,
  VideoSpecificMetadata,
  AudioSpecificMetadata,
  SpecificMetadata,
  SpeciesPredictionMeta,
} from "./metadata-types";

// Export type guards and utilities
export {
  isPhotoMetadata,
  isVideoMetadata,
  isAudioMetadata,
  asPhotoMetadata,
  asVideoMetadata,
  asAudioMetadata,
  getTypedMetadata,
} from "./metadata-types";

// Export extended types
export type {
  AssetDTO,
  Asset,
  UpdateAssetRequest,
} from "./schema-extensions";

// Export helper functions
export {
  toExtendedAsset,
  toExtendedAssets,
  extractAssets,
} from "./schema-extensions";

// Re-export API client
export { default as api } from "./api";
