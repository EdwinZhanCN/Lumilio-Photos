import { $api } from "@/lib/http-commons/queryClient";

/** Read-only preview; Delete rechecks every present file immediately before moving. */
export function useAssetDeleteImpact(assetIds: string[]) {
  return $api.useQuery(
    "post",
    "/api/v1/assets/delete-impact",
    { body: { asset_ids: assetIds } },
    { enabled: assetIds.length > 0, staleTime: 0 },
  );
}
