/**
 * Type extensions for auto-generated schema.d.ts
 * This file provides proper types for fields that OpenAPI generator cannot infer correctly
 */

import type { components } from "./schema";
import type { SpecificMetadata } from "./metadata-types";

/**
 * Extended AssetDTO with properly typed specific_metadata
 * Use this type instead of components["schemas"]["handler.AssetDTO"] when you need to access metadata
 */
export interface AssetDTO extends Omit<components["schemas"]["handler.AssetDTO"], "specific_metadata"> {
  specific_metadata?: SpecificMetadata;
}

/**
 * Extended UpdateAssetRequest with properly typed specific_metadata
 */
export interface UpdateAssetRequest extends Omit<components["schemas"]["handler.UpdateAssetRequest"], "specific_metadata"> {
  specific_metadata?: SpecificMetadata;
}

/**
 * Type alias for Asset (commonly used in the codebase)
 */
export type Asset = AssetDTO;

/**
 * Helper function to safely cast schema AssetDTO to extended AssetDTO
 * This is useful when you get data from API calls
 */
export function toExtendedAsset(
  asset: components["schemas"]["handler.AssetDTO"],
): AssetDTO {
  return asset as AssetDTO;
}

/**
 * Helper function to safely cast an array of schema AssetDTO to extended AssetDTO
 */
export function toExtendedAssets(
  assets: components["schemas"]["handler.AssetDTO"][],
): AssetDTO[] {
  return assets as AssetDTO[];
}

/**
 * Extract properly typed assets from API response
 */
export function extractAssets(
  response: components["schemas"]["handler.AssetListResponse"],
): AssetDTO[] {
  return (response.assets || []) as AssetDTO[];
}
