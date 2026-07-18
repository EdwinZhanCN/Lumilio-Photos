import { useCallback } from "react";
import type { InfiniteData } from "@tanstack/react-query";
import { useQueryClient } from "@tanstack/react-query";
import type { AssetActionsResult } from "../types";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import { Asset } from "@/lib/assets/types";

type BrowseItemDTO = components["schemas"]["dto.BrowseItemDTO"];
type QueryAssetsResponseDTO = components["schemas"]["dto.QueryAssetsResponseDTO"];
type SearchAssetsResponseDTO = components["schemas"]["dto.SearchAssetsResponseDTO"];
type AssetsListPage = QueryAssetsResponseDTO | SearchAssetsResponseDTO;

const ASSET_LIST_QUERY_PATHS = new Set(["/api/v1/assets/list", "/api/v1/assets/search"]);

const isAssetListQuery = (queryKey: unknown): boolean => {
  if (!Array.isArray(queryKey)) return false;
  const path = queryKey[1];
  return typeof path === "string" && ASSET_LIST_QUERY_PATHS.has(path);
};

const patchBrowseItemAsset = (
  item: BrowseItemDTO,
  assetId: string,
  updateFn: (asset: Asset) => Asset,
): BrowseItemDTO => {
  let updated = false;
  let nextItem = item;

  if (item.asset?.asset_id === assetId) {
    nextItem = {
      ...nextItem,
      asset: updateFn(item.asset as Asset),
    };
    updated = true;
  }

  if (item.stack?.cover_asset?.asset_id === assetId) {
    nextItem = {
      ...nextItem,
      stack: {
        ...item.stack,
        cover_asset: updateFn(item.stack.cover_asset as Asset),
      },
    };
    updated = true;
  }

  return updated ? nextItem : item;
};

const isSearchAssetsPage = (page: AssetsListPage): page is SearchAssetsResponseDTO =>
  "result_items" in page || "top_items" in page || "results_total_assets" in page;

const patchAssetsListPage = (
  page: AssetsListPage,
  assetId: string,
  updateFn: (asset: Asset) => Asset,
): AssetsListPage => {
  const patchBrowseItems = (items?: BrowseItemDTO[]) =>
    items?.map((item) => patchBrowseItemAsset(item, assetId, updateFn));

  if (isSearchAssetsPage(page)) {
    return {
      ...page,
      result_items: patchBrowseItems(page.result_items),
      top_items: patchBrowseItems(page.top_items),
    };
  }

  return {
    ...page,
    items: patchBrowseItems(page.items),
  };
};

/**
 * Hook for performing business operations on assets.
 */
export const useAssetActions = (): AssetActionsResult => {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { t } = useI18n();

  const updateRatingMutation = $api.useMutation("put", "/api/v1/assets/{id}/rating");
  const toggleLikeMutation = $api.useMutation("put", "/api/v1/assets/{id}/like");
  const updateDescriptionMutation = $api.useMutation("put", "/api/v1/assets/{id}/description");
  const deleteAssetMutation = $api.useMutation("delete", "/api/v1/assets/{id}");

  const invalidateAssetQueries = useCallback(() => {
    return queryClient.invalidateQueries({
      predicate: (query) => isAssetListQuery(query.queryKey),
    });
  }, [queryClient]);

  const updateAssetInCache = useCallback(
    (assetId: string, updateFn: (asset: Asset) => Asset) => {
      queryClient.setQueriesData<InfiniteData<AssetsListPage>>(
        {
          predicate: (query) => isAssetListQuery(query.queryKey),
        },
        (oldData) => {
          if (!oldData?.pages) return oldData;

          return {
            ...oldData,
            pages: oldData.pages.map((page) => patchAssetsListPage(page, assetId, updateFn)),
          };
        },
      );
    },
    [queryClient],
  );

  const updateRating = useCallback(
    async (assetId: string, rating: number): Promise<void> => {
      try {
        await updateRatingMutation.mutateAsync({
          params: { path: { id: assetId } },
          body: { rating },
        });
        updateAssetInCache(assetId, (asset) => ({ ...asset, rating }));
      } catch (error) {
        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [showMessage, t, updateAssetInCache, updateRatingMutation],
  );

  const toggleLike = useCallback(
    async (assetId: string, isLiked: boolean): Promise<void> => {
      try {
        await toggleLikeMutation.mutateAsync({
          params: { path: { id: assetId } },
          body: { liked: isLiked },
        });
        updateAssetInCache(assetId, (asset) => ({ ...asset, liked: isLiked }));
      } catch (error) {
        console.error("Failed to toggle like:", error);
        showMessage("error", t("rating.updateError"));
        throw error;
      }
    },
    [showMessage, t, toggleLikeMutation, updateAssetInCache],
  );

  const updateDescription = useCallback(
    async (assetId: string, description: string): Promise<void> => {
      try {
        await updateDescriptionMutation.mutateAsync({
          params: { path: { id: assetId } },
          body: { description },
        });
        await invalidateAssetQueries();
        showMessage("success", t("assets.basicInfo.descriptionUpdated"));
      } catch (error) {
        console.error("Failed to update description:", error);
        showMessage("error", t("assets.basicInfo.descriptionUpdateError"));
        throw error;
      }
    },
    [invalidateAssetQueries, showMessage, t, updateDescriptionMutation],
  );

  const deleteAsset = useCallback(
    async (assetId: string): Promise<void> => {
      try {
        await deleteAssetMutation.mutateAsync({
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
    [deleteAssetMutation, invalidateAssetQueries, showMessage, t],
  );

  const batchUpdateAssets = useCallback(
    async (
      updates: Array<{
        assetId: string;
        updates: Partial<Asset>;
      }>,
    ): Promise<void> => {
      try {
        await Promise.all(
          updates.map(async ({ assetId, updates: assetUpdates }) => {
            if (assetUpdates.rating !== undefined) {
              await updateRatingMutation.mutateAsync({
                params: { path: { id: assetId } },
                body: { rating: assetUpdates.rating },
              });
            }
            if (assetUpdates.liked !== undefined) {
              await toggleLikeMutation.mutateAsync({
                params: { path: { id: assetId } },
                body: { liked: assetUpdates.liked },
              });
            }
            if (assetUpdates.specific_metadata?.description !== undefined) {
              await updateDescriptionMutation.mutateAsync({
                params: { path: { id: assetId } },
                body: {
                  description: assetUpdates.specific_metadata.description,
                },
              });
            }
          }),
        );

        await invalidateAssetQueries();
        showMessage("success", t("bulk.updateSuccess", { count: updates.length }));
      } catch (error) {
        console.error("Failed to batch update assets:", error);
        showMessage("error", t("bulk.updateError"));
        throw error;
      }
    },
    [
      invalidateAssetQueries,
      showMessage,
      t,
      toggleLikeMutation,
      updateDescriptionMutation,
      updateRatingMutation,
    ],
  );

  const refreshAsset = useCallback(async (): Promise<void> => {
    await invalidateAssetQueries();
  }, [invalidateAssetQueries]);

  return {
    updateRating,
    toggleLike,
    updateDescription,
    deleteAsset,
    batchUpdateAssets,
    refreshAsset,
  };
};
