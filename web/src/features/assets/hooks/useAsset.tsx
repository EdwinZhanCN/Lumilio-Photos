import { useMemo } from "react";
import { useAssetsStore } from "../assets.store";
import { selectAsset, selectAssetMeta } from "../slices/entities.slice";
import { Asset } from "@/services";

/**
 * Hook for accessing a single asset from the entity store.
 * Provides reactive access to asset data and metadata.
 *
 * @param assetId The ID of the asset to retrieve
 * @returns Asset object and metadata, or undefined if not found
 */
export const useAsset = (assetId: string) => {
  const asset = useAssetsStore((s) => selectAsset(s.entities, assetId));
  const meta = useAssetsStore((s) => selectAssetMeta(s.entities, assetId));

  return useMemo(() => ({
    asset,
    meta,
    exists: !!asset,
    isOptimistic: meta?.isOptimistic ?? false,
    lastUpdated: meta?.lastUpdated,
    fetchOrigin: meta?.fetchOrigin,
  }), [asset, meta]);
};

/**
 * Hook for accessing multiple assets at once.
 * More efficient than calling useAsset multiple times.
 *
 * @param assetIds Array of asset IDs to retrieve
 * @returns Object mapping asset IDs to asset data
 */
export const useAssets = (assetIds: string[]) => {
  const entities = useAssetsStore((s) => s.entities);

  const assetsMap = useMemo(() => {
    const result: Record<
      string,
      {
        asset?: Asset;
        meta?: any;
        exists: boolean;
        isOptimistic: boolean;
        lastUpdated?: number;
        fetchOrigin?: string;
      }
    > = {};

    assetIds.forEach((assetId) => {
      const asset = selectAsset(entities, assetId);
      const meta = selectAssetMeta(entities, assetId);

      result[assetId] = {
        asset,
        meta,
        exists: !!asset,
        isOptimistic: meta?.isOptimistic ?? false,
        lastUpdated: meta?.lastUpdated,
        fetchOrigin: meta?.fetchOrigin,
      };
    });

    return result;
  }, [entities, assetIds]);

  return assetsMap;
};

/**
 * Hook for checking if an asset exists in the entity store.
 * Lightweight alternative to useAsset when you only need existence check.
 *
 * @param assetId The asset ID to check
 * @returns Boolean indicating if asset exists
 */
export const useAssetExists = (assetId: string): boolean => {
  return useAssetsStore((s) => !!selectAsset(s.entities, assetId));
};

/**
 * Hook for accessing asset metadata without the full asset object.
 * Useful for checking update states, optimistic flags, etc.
 *
 * @param assetId The asset ID
 * @returns Asset metadata
 */
export const useAssetMeta = (assetId: string) => {
  return useAssetsStore((s) => selectAssetMeta(s.entities, assetId));
};
