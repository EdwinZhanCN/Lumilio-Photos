import { useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";

const getViewerTimeZone = () =>
  typeof Intl !== "undefined" ? Intl.DateTimeFormat().resolvedOptions().timeZone : "UTC";

export function useRepositoryAssetCount(repositoryId: string) {
  const request = useMemo(
    () => ({
      filter: {
        repository_id: repositoryId,
      },
      pagination: {
        limit: 1,
        offset: 0,
      },
      sort_by: "recently_added" as const,
      viewer_timezone: getViewerTimeZone(),
    }),
    [repositoryId],
  );

  const query = $api.useQuery(
    "post",
    "/api/v1/assets/list",
    {
      body: request,
    },
    {
      enabled: Boolean(repositoryId),
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  );

  return {
    ...query,
    assetCount: query.data?.total_assets ?? 0,
  };
}
