import { useEffect, useMemo } from "react";
import { type InfiniteData, type UseInfiniteQueryResult } from "@tanstack/react-query";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { $api } from "@/lib/http-commons/queryClient";

type Schemas = components["schemas"];
type AssetMapPointDTO = Schemas["dto.AssetMapPointDTO"];
type AssetMapPointListResponse = Schemas["dto.AssetMapPointListResponseDTO"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};
export type MapPhotoPoint = AssetMapPointDTO;

const PAGE_SIZE = 1000;

export type UseMapPhotoAssetsOptions = {
  repositoryId?: string;
};

export function useMapPhotoAssets(options: UseMapPhotoAssetsOptions = {}) {
  const { repositoryId } = options;

  const query = $api.useInfiniteQuery(
    "get",
    "/api/v1/assets/map-points",
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
        const payload = (lastPage as ApiResult<AssetMapPointListResponse>)?.data;
        const pagePoints = payload?.points ?? [];
        const total = payload?.total;
        const offset = Number(lastPageParam ?? 0) || 0;

        if (typeof total === "number") {
          return offset + pagePoints.length < total
            ? offset + PAGE_SIZE
            : undefined;
        }

        return pagePoints.length >= PAGE_SIZE ? offset + PAGE_SIZE : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<ApiResult<AssetMapPointListResponse>>, unknown>;

  useEffect(() => {
    if (
      !query.hasNextPage ||
      query.isFetchingNextPage ||
      query.isLoading ||
      query.isError
    ) {
      return;
    }

    void query.fetchNextPage();
  }, [
    query.fetchNextPage,
    query.hasNextPage,
    query.isError,
    query.isFetchingNextPage,
    query.isLoading,
  ]);

  const allPoints = useMemo(() => {
    const pages = query.data?.pages ?? [];
    const merged: MapPhotoPoint[] = [];
    const seen = new Set<string>();

    for (const page of pages) {
      const points = page.data?.points ?? [];
      for (const point of points) {
        const assetId = point.asset_id;
        if (!assetId || seen.has(assetId)) {
          continue;
        }
        seen.add(assetId);
        merged.push(point);
      }
    }

    return merged;
  }, [query.data?.pages]);

  const totalPhotos = query.data?.pages?.reduce((maxTotal, page) => {
    const total = page.data?.total;
    if (typeof total !== "number") {
      return maxTotal;
    }
    return Math.max(maxTotal, total);
  }, 0);

  return {
    points: allPoints,
    loadedPhotos: allPoints.length,
    totalPhotos: totalPhotos && totalPhotos > 0 ? totalPhotos : undefined,
    ...query,
  };
}
