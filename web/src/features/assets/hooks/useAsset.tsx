import { useMemo } from "react";
import { useAssetsContext } from "./useAssetsContext";
import { selectAsset, selectAssetMeta } from "../reducers/entities.reducer";

/**
 * Hook for accessing a single asset from the entity store.
 * Provides reactive access to asset data and metadata.
 *
 * @param assetId The ID of the asset to retrieve
 * @returns Asset object and metadata, or undefined if not found
 *
 * @example
 * ```tsx
 * function AssetCard({ assetId }: { assetId: string }) {
 *   const { asset, meta, isOptimistic } = useAsset(assetId);
 *
 *   if (!asset) {
 *     return <div>Asset not found</div>;
 *   }
 *
 *   return (
 *     <div className={isOptimistic ? 'opacity-50' : ''}>
 *       <img src={asset.thumbnail_url} alt={asset.original_filename} />
 *       <p>Rating: {asset.specific_metadata?.rating || 'Unrated'}</p>
 *       {isOptimistic && <span>Updating...</span>}
 *     </div>
 *   );
 * }
 * ```
 */
export const useAsset = (assetId: string) => {
  const { state } = useAssetsContext();

  const result = useMemo(() => {
    const asset = selectAsset(state.entities, assetId);
    const meta = selectAssetMeta(state.entities, assetId);

    return {
      asset,
      meta,
      exists: !!asset,
      isOptimistic: meta?.isOptimistic ?? false,
      lastUpdated: meta?.lastUpdated,
      fetchOrigin: meta?.fetchOrigin,
    };
  }, [state.entities, assetId]);

  return result;
};

/**
 * Hook for accessing multiple assets at once.
 * More efficient than calling useAsset multiple times.
 *
 * @param assetIds Array of asset IDs to retrieve
 * @returns Object mapping asset IDs to asset data
 *
 * @example
 * ```tsx
 * function AssetGrid({ assetIds }: { assetIds: string[] }) {
 *   const assetsMap = useAssets(assetIds);
 *
 *   return (
 *     <div className="grid">
 *       {assetIds.map(id => {
 *         const { asset, isOptimistic } = assetsMap[id] || {};
 *         if (!asset) return null;
 *
 *         return (
 *           <AssetThumbnail
 *             key={id}
 *             asset={asset}
 *             isOptimistic={isOptimistic}
 *           />
 *         );
 *       })}
 *     </div>
 *   );
 * }
 * ```
 */
export const useAssets = (assetIds: string[]) => {
  const { state } = useAssetsContext();

  const assetsMap = useMemo(() => {
    const result: Record<string, {
      asset?: Asset;
      meta?: any;
      exists: boolean;
      isOptimistic: boolean;
      lastUpdated?: number;
      fetchOrigin?: string;
    }> = {};

    assetIds.forEach(assetId => {
      const asset = selectAsset(state.entities, assetId);
      const meta = selectAssetMeta(state.entities, assetId);

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
  }, [state.entities, assetIds]);

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
  const { state } = useAssetsContext();

  return useMemo(() => {
    return !!selectAsset(state.entities, assetId);
  }, [state.entities, assetId]);
};

/**
 * Hook for accessing asset metadata without the full asset object.
 * Useful for checking update states, optimistic flags, etc.
 *
 * @param assetId The asset ID
 * @returns Asset metadata
 */
export const useAssetMeta = (assetId: string) => {
  const { state } = useAssetsContext();

  return useMemo(() => {
    return selectAssetMeta(state.entities, assetId);
  }, [state.entities, assetId]);
};
