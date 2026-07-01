import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useI18n } from "@/lib/i18n.tsx";

export type AssetTag = components["schemas"]["dto.AssetTagDTO"];

/** asset_tags.source value for tags a user added by hand. */
export const TAG_SOURCE_USER = "user";

/**
 * A tag is user-editable only when it was added by hand (source "user").
 * Everything else (ai, zeroshot, system, bioclip_classify) is read-only.
 */
export const isManualTag = (tag: AssetTag): boolean => tag.source === TAG_SOURCE_USER;

/**
 * Hook for reading and mutating the tags attached to a single asset.
 *
 * Tags come from two sources: user-added "manual" tags (removable) and
 * AI-generated tags such as "zeroshot" (read-only — they are owned by the
 * classifier and reappear on reprocess).
 */
export function useAssetTags(assetId?: string) {
  const queryClient = useQueryClient();
  const showMessage = useMessage();
  const { t } = useI18n();

  const tagsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}/tags",
    { params: { path: { id: assetId ?? "" } } },
    { enabled: Boolean(assetId), staleTime: 30_000 },
  );

  const addMutation = $api.useMutation("post", "/api/v1/assets/{id}/tags");
  const removeMutation = $api.useMutation("delete", "/api/v1/assets/{id}/tags/{tagId}");

  const invalidate = useCallback(
    () =>
      queryClient.invalidateQueries({
        queryKey: ["get", "/api/v1/assets/{id}/tags"],
      }),
    [queryClient],
  );

  const addTag = useCallback(
    async (tagName: string): Promise<void> => {
      const name = tagName.trim();
      if (!assetId || !name) return;
      try {
        await addMutation.mutateAsync({
          params: { path: { id: assetId } },
          body: { tag_name: name },
        });
        await invalidate();
      } catch (error) {
        console.error("Failed to add tag:", error);
        showMessage("error", t("assets.tags.addError"));
        throw error;
      }
    },
    [addMutation, assetId, invalidate, showMessage, t],
  );

  const removeTag = useCallback(
    async (tagId: number): Promise<void> => {
      if (!assetId) return;
      try {
        await removeMutation.mutateAsync({
          params: { path: { id: assetId, tagId } },
        });
        await invalidate();
      } catch (error) {
        console.error("Failed to remove tag:", error);
        showMessage("error", t("assets.tags.removeError"));
        throw error;
      }
    },
    [assetId, invalidate, removeMutation, showMessage, t],
  );

  const tags: AssetTag[] = tagsQuery.data?.tags ?? [];

  return {
    tags,
    isLoading: tagsQuery.isLoading,
    isMutating: addMutation.isPending || removeMutation.isPending,
    addTag,
    removeTag,
  };
}
