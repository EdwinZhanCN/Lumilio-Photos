import { $api } from "@/lib/http-commons/queryClient";

export const ASSET_FILTER_OPTIONS_QUERY_KEY = ["get", "/api/v1/assets/filter-options"] as const;

/** Shared TanStack Query owner for camera/lens filter metadata. */
export function useAssetFilterOptions(enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/filter-options",
    {},
    {
      enabled,
      staleTime: 5 * 60 * 1000,
      gcTime: 5 * 60 * 1000,
    },
  );
}
