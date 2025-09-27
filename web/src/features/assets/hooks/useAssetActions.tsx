import { useCallback } from "react";
import { useAssetsContext } from "./useAssetsContext";
import { AssetActionsResult } from "../types";
import { assetService } from "@/services/assetsService";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useI18n } from "@/lib/i18n.tsx";

/**
 * Hook for performing business operations on assets.
 * Handles optimistic updates and error recovery automatically.
 *
 * @returns Object containing asset action functions
 *
 * @example
 * ```tsx
 * function AssetRating({ assetId }: { assetId: string }) {
 *   const { updateRating } = useAssetActions();
 *   const { asset } = useAsset(assetId);
 *
 *   const handleRatingChange = async (newRating: number) => {
 *     await updateRating(assetId, newRating);
 *   };
 *
 *   return (
 *     <StarRating
 *       rating={asset?.specific_metadata?.rating || 0}
 *       onChange={handleRatingChange}
 *     />
 *   );
 * }
 * ```
 */
export const useAssetActions = (): AssetActionsResult => {
  const { state, dispatch } = useAssetsContext();
  const showMessage = useMessage();
  const { t } = useI18n();

  /**
   * Update asset rating with optimistic updates
   */
  const updateRating = useCallback(
    async (assetId: string, rating: number): Promise<void> => {
      const currentAsset = state.entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      // Optimistic update
      const optimisticUpdate = {
        specific_metadata: {
          ...currentAsset.specific_metadata,
          rating,
        },
      };

      dispatch({
        type: "UPDATE_ENTITY",
        payload: {
          assetId,
          updates: optimisticUpdate,
          meta: { isOptimistic: true },
        },
      });

      try {
        // Perform actual API call
        await assetService.updateAssetRating(assetId, rating);

        // Confirm the update (remove optimistic flag)
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: optimisticUpdate,
            meta: { isOptimistic: false },
          },
        });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        // Revert optimistic update on error
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: {
              specific_metadata: currentAsset.specific_metadata,
            },
            meta: { isOptimistic: false },
          },
        });

        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [state.entities.assets, dispatch, showMessage, t],
  );

  /**
   * Toggle asset like status with optimistic updates
   */
  const toggleLike = useCallback(
    async (assetId: string): Promise<void> => {
      const currentAsset = state.entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      const currentLiked = currentAsset.specific_metadata?.liked || false;
      const newLiked = !currentLiked;

      // Optimistic update
      const optimisticUpdate = {
        specific_metadata: {
          ...currentAsset.specific_metadata,
          liked: newLiked,
        },
      };

      dispatch({
        type: "UPDATE_ENTITY",
        payload: {
          assetId,
          updates: optimisticUpdate,
          meta: { isOptimistic: true },
        },
      });

      try {
        // Perform actual API call
        await assetService.updateAssetLike(assetId, newLiked);

        // Confirm the update
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: optimisticUpdate,
            meta: { isOptimistic: false },
          },
        });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        // Revert optimistic update on error
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: {
              specific_metadata: currentAsset.specific_metadata,
            },
            meta: { isOptimistic: false },
          },
        });

        console.error("Failed to toggle like:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [state.entities.assets, dispatch, showMessage, t],
  );

  /**
   * Update asset description with optimistic updates
   */
  const updateDescription = useCallback(
    async (assetId: string, description: string): Promise<void> => {
      const currentAsset = state.entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      // Optimistic update
      const optimisticUpdate = {
        specific_metadata: {
          ...currentAsset.specific_metadata,
          description,
        },
      };

      dispatch({
        type: "UPDATE_ENTITY",
        payload: {
          assetId,
          updates: optimisticUpdate,
          meta: { isOptimistic: true },
        },
      });

      try {
        // Perform actual API call
        await assetService.updateAssetDescription(assetId, description);

        // Confirm the update
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: optimisticUpdate,
            meta: { isOptimistic: false },
          },
        });

        showMessage("success", t("assets.basicInfo.descriptionUpdated"));
      } catch (error) {
        // Revert optimistic update on error
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: {
              specific_metadata: currentAsset.specific_metadata,
            },
            meta: { isOptimistic: false },
          },
        });

        console.error("Failed to update description:", error);
        showMessage("error", t("assets.basicInfo.descriptionUpdateError"));
        throw error;
      }
    },
    [state.entities.assets, dispatch, showMessage, t],
  );

  /**
   * Delete asset (soft delete, marks as deleted and removes from views)
   */
  const deleteAsset = useCallback(
    async (assetId: string): Promise<void> => {
      const currentAsset = state.entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      try {
        // Perform API call first for delete (no optimistic update for safety)
        await assetService.deleteAsset(assetId);

        // Remove from all views
        dispatch({
          type: "REMOVE_ASSET_FROM_VIEWS",
          payload: { assetId },
        });

        // Mark as deleted in entity store (or remove entirely)
        dispatch({
          type: "DELETE_ENTITY",
          payload: { assetId },
        });

        showMessage("success", t("delete.success"));
      } catch (error) {
        console.error("Failed to delete asset:", error);
        showMessage("error", t("delete.error"));
        throw error;
      }
    },
    [dispatch, showMessage, t],
  );

  /**
   * Batch update multiple assets (useful for bulk operations)
   */
  const batchUpdateAssets = useCallback(
    async (
      updates: Array<{
        assetId: string;
        updates: Partial<Asset>;
      }>,
    ): Promise<void> => {
      // Apply optimistic updates
      updates.forEach(({ assetId, updates: assetUpdates }) => {
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: assetUpdates,
            meta: { isOptimistic: true },
          },
        });
      });

      try {
        // Batch API call (if supported, otherwise sequential calls)
        await Promise.all(
          updates.map(async ({ assetId, updates: assetUpdates }) => {
            // Implement specific API calls based on update type
            if (assetUpdates.specific_metadata?.rating !== undefined) {
              await assetService.updateAssetRating(
                assetId,
                assetUpdates.specific_metadata.rating,
              );
            }
            if (assetUpdates.specific_metadata?.liked !== undefined) {
              await assetService.updateAssetLike(
                assetId,
                assetUpdates.specific_metadata.liked,
              );
            }
            if (assetUpdates.specific_metadata?.description !== undefined) {
              await assetService.updateAssetDescription(
                assetId,
                assetUpdates.specific_metadata.description,
              );
            }
          }),
        );

        // Confirm all updates
        updates.forEach(({ assetId, updates: assetUpdates }) => {
          dispatch({
            type: "UPDATE_ENTITY",
            payload: {
              assetId,
              updates: assetUpdates,
              meta: { isOptimistic: false },
            },
          });
        });

        showMessage(
          "success",
          t("bulk.updateSuccess", { count: updates.length }),
        );
      } catch (error) {
        // Revert all optimistic updates
        updates.forEach(({ assetId }) => {
          const originalAsset = state.entities.assets[assetId];
          if (originalAsset) {
            dispatch({
              type: "SET_ENTITY",
              payload: {
                assetId,
                asset: originalAsset,
                meta: { isOptimistic: false },
              },
            });
          }
        });

        console.error("Failed to batch update assets:", error);
        showMessage("error", t("bulk.updateError"));
        throw error;
      }
    },
    [state.entities.assets, dispatch, showMessage, t],
  );

  /**
   * Refresh asset data from server (useful after external changes)
   */
  const refreshAsset = useCallback(
    async (assetId: string): Promise<void> => {
      try {
        const response = await assetService.getAssetById(assetId);
        const updatedAsset = response.data?.data;

        if (updatedAsset) {
          dispatch({
            type: "SET_ENTITY",
            payload: {
              assetId,
              asset: updatedAsset,
              meta: { fetchOrigin: "refresh" },
            },
          });
        }
      } catch (error) {
        console.error("Failed to refresh asset:", error);
        showMessage("error", t("refresh.error"));
        throw error;
      }
    },
    [dispatch, showMessage, t],
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
 * Useful for operations where immediate feedback isn't critical.
 */
export const useAssetActionsSimple = () => {
  const showMessage = useMessage();
  const { t } = useI18n();
  const { dispatch } = useAssetsContext();

  const updateRating = useCallback(
    async (assetId: string, rating: number) => {
      try {
        await assetService.updateAssetRating(assetId, rating);

        // Simple update without optimistic handling
        dispatch({
          type: "UPDATE_ENTITY",
          payload: {
            assetId,
            updates: {
              specific_metadata: { rating },
            },
          },
        });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [dispatch, showMessage, t],
  );

  return { updateRating };
};
