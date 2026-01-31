import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { AssetActionsResult } from "@/features/assets";
import client from "@/lib/http-commons/client";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useI18n } from "@/lib/i18n.tsx";
import { Asset } from "@/lib/assets/types";

/**
 * Hook for performing business operations on assets.
 * Now simplified for React 19 architecture:
 * - No manual cache updates (optimistic UI handles immediate feedback)
 * - No snapshot/restore logic
 * - Just API call + invalidateQueries
 *
 * @returns Object containing asset action functions
 */
export const useAssetActions = (): AssetActionsResult => {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { t } = useI18n();

  // List of API paths that return asset lists (infinite queries)
  // We strictly match these to avoid accidentally updating unrelated queries
  const ASSET_LIST_QUERY_PATHS = new Set([
    "/api/v1/assets/list",
  ]);

  /**
   * Helper to invalidate asset queries
   */
  const invalidateAssetQueries = useCallback(() => {
    return queryClient.invalidateQueries({
      predicate: (query) => {
        const key = query.queryKey;
        // openapi-react-query keys are typically [method, path, params]
        if (Array.isArray(key)) {
          const path = key[1];
          if (typeof path === 'string') {
            return ASSET_LIST_QUERY_PATHS.has(path);
          }
        }
        return false;
      },
    });
  }, [queryClient]);

  /**
   * Helper to manually update asset in cache without invalidation
   */
  const updateAssetInCache = useCallback((assetId: string, updateFn: (asset: Asset) => Asset) => {
    queryClient.setQueriesData(
      {
        predicate: (query) => {
          const key = query.queryKey;
          if (Array.isArray(key)) {
            const path = key[1];
            if (typeof path === 'string') {
              return ASSET_LIST_QUERY_PATHS.has(path);
            }
          }
          return false;
        },
      },
      (oldData: any) => {
        if (!oldData) return oldData;

        // Handle Infinite Query data structure
        if (oldData.pages && Array.isArray(oldData.pages)) {
          return {
            ...oldData,
            pages: oldData.pages.map((page: any) => {
              if (page.data && Array.isArray(page.data.assets)) {
                return {
                  ...page,
                  data: {
                    ...page.data,
                    assets: page.data.assets.map((asset: Asset) => {
                      if (asset.asset_id === assetId) {
                        return updateFn(asset);
                      }
                      return asset;
                    }),
                  },
                };
              }
              return page;
            }),
          };
        }
        return oldData;
      }
    );
  }, [queryClient]);

  /**
   * Update asset rating
   */
  const updateRating = useCallback(
    async (assetId: string, rating: number): Promise<void> => {
      try {
        await client.PUT("/api/v1/assets/{id}/rating", {
          params: { path: { id: assetId } },
          body: { rating },
        });

        // Hybrid Strategy: Manual update for low-risk, high-frequency action
        updateAssetInCache(assetId, (asset) => ({ ...asset, rating }));
      } catch (error) {
        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [updateAssetInCache, showMessage, t],
  );

  /**
   * Toggle asset like status
   */
  const toggleLike = useCallback(
    async (assetId: string, isLiked: boolean): Promise<void> => {
      try {
        await client.PUT("/api/v1/assets/{id}/like", {
          params: { path: { id: assetId } },
          body: { liked: isLiked },
        });

        // Hybrid Strategy: Manual update for low-risk, high-frequency action
        updateAssetInCache(assetId, (asset) => ({ ...asset, liked: isLiked }));
      } catch (error) {
        console.error("Failed to toggle like:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [updateAssetInCache, showMessage, t],
  );

  /**
   * Update asset description
   */
  const updateDescription = useCallback(
    async (assetId: string, description: string): Promise<void> => {
      try {
        await client.PUT("/api/v1/assets/{id}/description", {
          params: { path: { id: assetId } },
          body: { description },
        });

        // Hybrid Strategy: Invalidate for higher-risk/lower-frequency action
        // Ensuring description matches everywhere (e.g. search results) is complex to patch manually
        await invalidateAssetQueries();
        showMessage("success", t("assets.basicInfo.descriptionUpdated"));
      } catch (error) {
        console.error("Failed to update description:", error);
        showMessage("error", t("assets.basicInfo.descriptionUpdateError"));
        throw error;
      }
    },
    [invalidateAssetQueries, showMessage, t],
  );

  /**
   * Delete asset
   */
  const deleteAsset = useCallback(
    async (assetId: string): Promise<void> => {
      try {
        await client.DELETE("/api/v1/assets/{id}", {
          params: { path: { id: assetId } },
        });

        await invalidateAssetQueries();
        showMessage("success", t("delete.success"));
      } catch (error) {
        console.error("Failed to delete asset:", error);
        showMessage("error", t("delete.error"));
        throw error;
      }
    },
    [invalidateAssetQueries, showMessage, t],
  );

  /**
   * Batch update multiple assets
   */
  const batchUpdateAssets = useCallback(
    async (
      updates: Array<{
        assetId: string;
        updates: Partial<Asset>;
      }>,
    ): Promise<void> => {
      try {
        // Batch API call (looping for now as per original logic)
        await Promise.all(
          updates.map(async ({ assetId, updates: assetUpdates }) => {
            if (assetUpdates.rating !== undefined) {
              await client.PUT("/api/v1/assets/{id}/rating", {
                params: { path: { id: assetId } },
                body: { rating: assetUpdates.rating },
              });
            }
            if (assetUpdates.liked !== undefined) {
              await client.PUT("/api/v1/assets/{id}/like", {
                params: { path: { id: assetId } },
                body: { liked: assetUpdates.liked },
              });
            }
            if (assetUpdates.specific_metadata?.description !== undefined) {
              await client.PUT("/api/v1/assets/{id}/description", {
                params: { path: { id: assetId } },
                body: { description: assetUpdates.specific_metadata.description },
              });
            }
          }),
        );

        await invalidateAssetQueries();
        showMessage(
          "success",
          t("bulk.updateSuccess", { count: updates.length }),
        );
      } catch (error) {
        console.error("Failed to batch update assets:", error);
        showMessage("error", t("bulk.updateError"));
        throw error;
      }
    },
    [invalidateAssetQueries, showMessage, t],
  );

  /**
   * Refresh asset data
   */
  const refreshAsset = useCallback(
    async (): Promise<void> => {
      // With the new strategy, refresh usually just means invalidating
      await invalidateAssetQueries();
    },
    [invalidateAssetQueries],
  );

  return {
    updateRating,
    toggleLike,
    updateDescription,
    deleteAsset,
    batchUpdateAssets,
    refreshAsset,
  };
};

/**
 * Hook for asset actions that don't require optimistic updates.
 * (This is essentially the same as above now, potentially can be merged or deprecated)
 */
export const useAssetActionsSimple = useAssetActions;

