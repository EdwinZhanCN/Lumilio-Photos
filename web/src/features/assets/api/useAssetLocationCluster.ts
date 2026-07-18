import { useMemo } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import { encodeGeohash } from "@/lib/geo/geohash";

type UseAssetLocationClusterOptions = {
  latitude?: number;
  longitude?: number;
  repositoryId?: string;
};

export function useAssetLocationCluster({
  latitude,
  longitude,
  repositoryId,
}: UseAssetLocationClusterOptions) {
  const geohash = useMemo(() => {
    if (typeof latitude !== "number" || typeof longitude !== "number") {
      return null;
    }
    return encodeGeohash(latitude, longitude, 7);
  }, [latitude, longitude]);

  const query = $api.useQuery(
    "get",
    "/api/v1/locations/clusters",
    {
      params: {
        query: {
          geohash: geohash ?? undefined,
          limit: 1,
          repository_id: repositoryId,
        },
      },
    },
    {
      enabled: Boolean(geohash),
      staleTime: 5 * 60 * 1000,
      gcTime: 30 * 60 * 1000,
      retry: 1,
    },
  );

  const cluster = query.data?.clusters?.[0];

  return {
    cluster,
    geohash,
    ...query,
  };
}
