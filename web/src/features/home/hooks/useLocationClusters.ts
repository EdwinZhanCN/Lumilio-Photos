import { useMemo } from "react";
import { type InfiniteData, type UseInfiniteQueryResult } from "@tanstack/react-query";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { $api } from "@/lib/http-commons/queryClient";

type Schemas = components["schemas"];
type LocationClusterDTO = Schemas["dto.LocationClusterDTO"];
type LocationClusterListResponse = Schemas["dto.LocationClusterListResponseDTO"];

export type LocationCluster = LocationClusterDTO;

const PAGE_SIZE = 100;

export type UseLocationClustersOptions = {
  repositoryId?: string;
};

export function useLocationClusters(options: UseLocationClustersOptions = {}) {
  const { repositoryId } = options;

  const query = $api.useInfiniteQuery(
    "get",
    "/api/v1/locations/clusters",
    {
      params: {
        query: {
          limit: PAGE_SIZE,
          repository_id: repositoryId,
        },
      },
    },
    {
      pageParamName: "offset",
      initialPageParam: 0,
      staleTime: 2 * 60 * 1000,
      gcTime: 15 * 60 * 1000,
      retry: 1,
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const payload = lastPage as LocationClusterListResponse | undefined;
        const pageClusters = payload?.clusters ?? [];
        const total = payload?.total;
        const offset = Number(lastPageParam ?? 0) || 0;

        if (typeof total === "number") {
          return offset + pageClusters.length < total
            ? offset + PAGE_SIZE
            : undefined;
        }

        return pageClusters.length >= PAGE_SIZE ? offset + PAGE_SIZE : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<LocationClusterListResponse>, unknown>;

  const clusters = useMemo(() => {
    const pages = query.data?.pages ?? [];
    const merged: LocationCluster[] = [];
    const seen = new Set<string>();

    for (const page of pages) {
      const pageClusters = page.clusters ?? [];
      for (const cluster of pageClusters) {
        const clusterId = cluster.cluster_id;
        if (!clusterId || seen.has(clusterId)) {
          continue;
        }
        seen.add(clusterId);
        merged.push(cluster);
      }
    }

    return merged;
  }, [query.data?.pages]);

  const totalClusters = query.data?.pages?.reduce((maxTotal, page) => {
    const total = page.total;
    if (typeof total !== "number") {
      return maxTotal;
    }
    return Math.max(maxTotal, total);
  }, 0);

  return {
    clusters,
    loadedClusters: clusters.length,
    totalClusters: totalClusters && totalClusters > 0 ? totalClusters : undefined,
    ...query,
  };
}
