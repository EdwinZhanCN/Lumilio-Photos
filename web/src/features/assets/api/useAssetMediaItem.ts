import { $api } from "@/lib/http-commons/queryClient";
import type { MediaItemByAssetResponse } from "@/lib/assets/types";

const isMediaItemResponse = (value: unknown): value is MediaItemByAssetResponse => {
  if (!value || typeof value !== "object") return false;
  const record = value as Record<string, unknown>;
  return typeof record.asset_id === "string" && Boolean(record.media_item);
};

export function useAssetMediaItem(assetId?: string, enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/{id}/media-item",
    {
      params: { path: { id: assetId ?? "" } },
    },
    {
      enabled: enabled && Boolean(assetId),
      retry: false,
      select: (response) => (isMediaItemResponse(response) ? response : undefined),
    },
  );
}
