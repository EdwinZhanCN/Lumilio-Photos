import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import type { Asset } from "@/lib/assets/types";
import { $api } from "@/lib/http-commons/queryClient";
import { useAssetActions } from "../../../api/useAssetActions";
import { downloadAssets } from "../../export/downloadAssets";
import { useAssetSelection } from "../selection/useAssetSelection";

export function useBulkAssetActions(resolvedAssetIds?: string[]) {
  const selection = useAssetSelection();
  const queryClient = useQueryClient();
  const { deleteAsset, batchUpdateAssets } = useAssetActions();
  const { mutateAsync: addAssetToAlbum } = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/assets/{assetId}",
  );
  const { mutateAsync: addAssetTag } = $api.useMutation("post", "/api/v1/assets/{id}/tags");
  const targetIds = useCallback(
    () => resolvedAssetIds ?? Array.from(selection.selectedIds),
    [resolvedAssetIds, selection.selectedIds],
  );

  const bulkUpdateRating = useCallback(
    async (rating: number) => {
      await batchUpdateAssets(targetIds().map((assetId) => ({ assetId, updates: { rating } })));
    },
    [batchUpdateAssets, targetIds],
  );

  const bulkSetLike = useCallback(
    async (liked: boolean) => {
      await batchUpdateAssets(targetIds().map((assetId) => ({ assetId, updates: { liked } })));
    },
    [batchUpdateAssets, targetIds],
  );

  const bulkDelete = useCallback(async () => {
    await Promise.all(targetIds().map(deleteAsset));
    selection.clear();
  }, [deleteAsset, selection, targetIds]);

  const bulkDownload = useCallback(
    (assets?: Asset[]) => downloadAssets(targetIds(), assets),
    [targetIds],
  );

  const bulkAddToAlbum = useCallback(
    async (albumId: number) => {
      await Promise.all(
        targetIds().map((assetId) =>
          addAssetToAlbum({
            params: { path: { id: albumId, assetId } },
            body: {},
          }),
        ),
      );
    },
    [addAssetToAlbum, targetIds],
  );

  const bulkAddTags = useCallback(
    async (tagNames: string[]) => {
      const names = [...new Set(tagNames.map((name) => name.trim()).filter(Boolean))];
      if (names.length === 0) return;

      await Promise.all(
        targetIds().flatMap((assetId) =>
          names.map((tagName) =>
            addAssetTag({
              params: { path: { id: assetId } },
              body: { tag_name: tagName },
            }),
          ),
        ),
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/{id}/tags"] }),
        queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/tags"] }),
        queryClient.invalidateQueries({
          predicate: (query) => {
            const path = query.queryKey[1];
            return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
          },
        }),
      ]);
    },
    [addAssetTag, queryClient, targetIds],
  );

  return {
    bulkUpdateRating,
    bulkSetLike,
    bulkDelete,
    bulkDownload,
    bulkAddToAlbum,
    bulkAddTags,
    selectedCount: selection.selectedCount,
    hasSelection: selection.hasSelection,
  };
}
