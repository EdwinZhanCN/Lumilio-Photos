import { useCallback } from "react";
import { useAssetsStore } from "../assets.store";
import { useShallow } from "zustand/react/shallow";
import { AssetActionsResult } from "@/features/assets";
import client from "@/lib/http-commons/client";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useI18n } from "@/lib/i18n.tsx";
import { Asset } from "@/lib/assets/types";

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
  const {
    updateEntity,
    deleteEntity,
    removeAssetFromViews,
    setEntity,
  } = useAssetsStore(
    useShallow((state) => ({
      updateEntity: state.updateEntity,
      deleteEntity: state.deleteEntity,
      removeAssetFromViews: state.removeAssetFromViews,
      setEntity: state.setEntity,
    }))
  );

  const showMessage = useMessage();
  const { t } = useI18n();

  /**
   * Update asset rating with optimistic updates
   */
  const updateRating = useCallback(
    async (assetId: string, rating: number): Promise<void> => {
      // Access current state via getState() to avoid subscription rerenders
      const currentAsset = useAssetsStore.getState().entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      // Optimistic update
      const optimisticUpdate = {
        rating,
      };

      updateEntity(assetId, optimisticUpdate, { isOptimistic: true });

      try {
        // Perform actual API call
        await client.PUT("/api/v1/assets/{id}/rating", {
          params: { path: { id: assetId } },
          body: { rating },
        });

        // Confirm the update (remove optimistic flag)
        updateEntity(assetId, optimisticUpdate, { isOptimistic: false });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        // Revert optimistic update on error
        updateEntity(
          assetId,
          { rating: currentAsset.rating },
          { isOptimistic: false }
        );

        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [updateEntity, showMessage, t],
  );

  /**
   * Toggle asset like status with optimistic updates
   */
  const toggleLike = useCallback(
    async (assetId: string): Promise<void> => {
      const currentAsset = useAssetsStore.getState().entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      const currentLiked = currentAsset.liked || false;
      const newLiked = !currentLiked;

      // Optimistic update
      const optimisticUpdate = {
        liked: newLiked,
      };

      updateEntity(assetId, optimisticUpdate, { isOptimistic: true });

      try {
        // Perform actual API call
        await client.PUT("/api/v1/assets/{id}/like", {
          params: { path: { id: assetId } },
          body: { liked: newLiked },
        });

        // Confirm the update
        updateEntity(assetId, optimisticUpdate, { isOptimistic: false });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        // Revert optimistic update on error
        updateEntity(
          assetId,
          { liked: currentAsset.liked },
          { isOptimistic: false }
        );

        console.error("Failed to toggle like:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [updateEntity, showMessage, t],
  );

  /**
   * Update asset description with optimistic updates
   */
  const updateDescription = useCallback(
    async (assetId: string, description: string): Promise<void> => {
      const currentAsset = useAssetsStore.getState().entities.assets[assetId];
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

      updateEntity(assetId, optimisticUpdate, { isOptimistic: true });

      try {
        // Perform actual API call
        await client.PUT("/api/v1/assets/{id}/description", {
          params: { path: { id: assetId } },
          body: { description },
        });

        // Confirm the update
        updateEntity(assetId, optimisticUpdate, { isOptimistic: false });

        showMessage("success", t("assets.basicInfo.descriptionUpdated"));
      } catch (error) {
        // Revert optimistic update on error
        updateEntity(
          assetId,
          { specific_metadata: currentAsset.specific_metadata },
          { isOptimistic: false }
        );

        console.error("Failed to update description:", error);
        showMessage("error", t("assets.basicInfo.descriptionUpdateError"));
        throw error;
      }
    },
    [updateEntity, showMessage, t],
  );

  /**
   * Delete asset (soft delete, marks as deleted and removes from views)
   */
  const deleteAsset = useCallback(
    async (assetId: string): Promise<void> => {
      const currentAsset = useAssetsStore.getState().entities.assets[assetId];
      if (!currentAsset) {
        throw new Error("Asset not found");
      }

      try {
        // Perform API call first for delete (no optimistic update for safety)
        await client.DELETE("/api/v1/assets/{id}", {
          params: { path: { id: assetId } },
        });

        // Remove from all views
        removeAssetFromViews(assetId);

        // Mark as deleted in entity store (or remove entirely)
        deleteEntity(assetId);

        showMessage("success", t("delete.success"));
      } catch (error) {
        console.error("Failed to delete asset:", error);
        showMessage("error", t("delete.error"));
        throw error;
      }
    },
    [deleteEntity, removeAssetFromViews, showMessage, t],
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
        updateEntity(assetId, assetUpdates, { isOptimistic: true });
      });

      try {
        // Batch API call (if supported, otherwise sequential calls)
        await Promise.all(
          updates.map(async ({ assetId, updates: assetUpdates }) => {
            // Implement specific API calls based on update type
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

        // Confirm all updates
        updates.forEach(({ assetId, updates: assetUpdates }) => {
          updateEntity(assetId, assetUpdates, { isOptimistic: false });
        });

        showMessage(
          "success",
          t("bulk.updateSuccess", { count: updates.length }),
        );
      } catch (error) {
        // Revert all optimistic updates
        updates.forEach(({ assetId }) => {
          const originalAsset = useAssetsStore.getState().entities.assets[assetId];
          if (originalAsset) {
            setEntity(assetId, originalAsset, { isOptimistic: false });
          }
        });

        console.error("Failed to batch update assets:", error);
        showMessage("error", t("bulk.updateError"));
        throw error;
      }
    },
    [updateEntity, setEntity, showMessage, t],
  );

  /**
   * Refresh asset data from server (useful after external changes)
   */
  const refreshAsset = useCallback(
    async (assetId: string): Promise<void> => {
      try {
        const response = await client.GET("/api/v1/assets/{id}", {
          params: { path: { id: assetId } },
        });
        const updatedAsset = response.data?.data as Asset | undefined;

        if (updatedAsset) {
          setEntity(assetId, updatedAsset, { fetchOrigin: "refresh" });
        }
      } catch (error) {
        console.error("Failed to refresh asset:", error);
        showMessage("error", t("refresh.error"));
        throw error;
      }
    },
    [setEntity, showMessage, t],
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
  const updateEntity = useAssetsStore((state) => state.updateEntity);

  const updateRating = useCallback(
    async (assetId: string, rating: number) => {
      try {
        await client.PUT("/api/v1/assets/{id}/rating", {
          params: { path: { id: assetId } },
          body: { rating },
        });

        // Simple update without optimistic handling
        updateEntity(assetId, { rating });

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [updateEntity, showMessage, t],
  );

  return { updateRating };
};
