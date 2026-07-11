import { useEffect, useMemo } from "react";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { $api } from "@/lib/http-commons/queryClient";
import type { MapViewport } from "@/components/MapComponent";

type Schemas = components["schemas"];
type AssetMapPointDTO = Schemas["dto.AssetMapPointDTO"];
export type MapPhotoPoint = AssetMapPointDTO;

const DEFAULT_PAGE_SIZE = 1000;

export type UseMapPhotoAssetsOptions = {
  repositoryId?: string;
  viewport?: MapViewport | null;
  enabled?: boolean;
  autoFetchAll?: boolean;
  pageSize?: number;
};

export function useMapPhotoAssets(options: UseMapPhotoAssetsOptions = {}) {
  const {
    repositoryId,
    viewport,
    enabled = true,
    autoFetchAll = false,
    pageSize = DEFAULT_PAGE_SIZE,
  } = options;
  const viewportQuery = viewport
    ? {
        west: viewport.bbox[0],
        south: viewport.bbox[1],
        east: viewport.bbox[2],
        north: viewport.bbox[3],
      }
    : undefined;

  const query = $api.useInfiniteQuery(
    "get",
    "/api/v1/assets/map-points",
    {
      params: {
        query: {
          limit: pageSize,
          repository_id: repositoryId,
          ...viewportQuery,
        },
      },
    },
    {
      pageParamName: "offset",
      initialPageParam: 0,
      enabled,
      staleTime: 2 * 60 * 1000,
      gcTime: 2 * 60 * 1000,
      retry: 1,
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const pagePoints = lastPage?.points ?? [];
        const total = lastPage?.total;
        const offset = Number(lastPageParam ?? 0) || 0;
        if (pagePoints.length === 0) return undefined;

        if (typeof total === "number") {
          return offset + pagePoints.length < total ? offset + pagePoints.length : undefined;
        }

        return pagePoints.length >= pageSize ? offset + pagePoints.length : undefined;
      },
    },
  );

  useEffect(() => {
    if (
      !autoFetchAll ||
      !query.hasNextPage ||
      query.isFetchingNextPage ||
      query.isLoading ||
      query.isError
    ) {
      return;
    }

    void query.fetchNextPage();
  }, [
    autoFetchAll,
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
      const points = page.points ?? [];
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
    const total = page.total;
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
