import { $api } from "@/lib/http-commons/queryClient";
import type { StackByAssetResponse } from "@/lib/assets/types";

const isStackByAssetResponse = (value: unknown): value is StackByAssetResponse => {
  if (!value || typeof value !== "object") {
    return false;
  }

  const record = value as Record<string, unknown>;
  return (
    typeof record.asset_id === "string" && Boolean(record.stack) && typeof record.stack === "object"
  );
};

const unwrapStackByAssetResponse = (response: unknown): StackByAssetResponse | undefined => {
  if (isStackByAssetResponse(response)) {
    return response;
  }

  return undefined;
};

export function useAssetStackDetails(assetId?: string, enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/{id}/stack",
    {
      params: {
        path: {
          id: assetId ?? "",
        },
      },
    },
    {
      enabled: enabled && Boolean(assetId),
      retry: false,
      select: (response) => unwrapStackByAssetResponse(response),
    },
  );
}
