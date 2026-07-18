import { $api } from "@/lib/http-commons/queryClient";

/** Shared small album list for pickers, asset actions, and mention sources. */
export function useAlbumOptions(enabled = true) {
  return $api.useQuery(
    "get",
    "/api/v1/albums",
    { params: { query: { limit: 100, offset: 0 } } },
    { enabled, staleTime: 60_000, gcTime: 5 * 60_000 },
  );
}
