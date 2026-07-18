import { useMemo } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import type { Asset } from "@/lib/assets/types";
import { client } from "@/lib/http-commons/client";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type {
  AssetGroup,
  AssetsViewResult,
  AssetViewDefinition,
  BrowseGroup,
  BrowseItem,
  ViewDefinitionOptions,
} from "../types";
import { flattenAssetGroups, getViewerTimeZone } from "../model/assetGroups";
import { withBodyPaginationOffset } from "./bodyPagination";
import {
  browseGroupsFromQueryLikePage,
  countLoadedBrowseRowsFromPage,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  mergeAdjacentBrowseGroups,
} from "../model/browseItems";
import { generateViewKey } from "../model/viewKey";
import { buildAssetApiFilter, normalizeAssetSort } from "./assetViewModel";

type AssetQueryRequest = components["schemas"]["dto.AssetQueryRequestDTO"];

type AssetsListQueryResult = {
  assets: Asset[];
  groups: AssetGroup[];
  browseGroups: BrowseGroup[];
  browseItems: BrowseItem[];
  browseAssets: Asset[];
  isLoading: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  fetchMore: () => Promise<void>;
  refetch: () => Promise<void>;
  isFetched: boolean;
  error: string | null;
  viewKey: string;
  pageInfo: { cursor?: string; page: number; total?: number };
};

function useAssetsListQuery(
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions,
): AssetsListQueryResult {
  const { autoFetch = true, disabled = false } = options;
  const apiFilter = useMemo(
    () => buildAssetApiFilter(definition, definition.filter ?? {}),
    [definition],
  );
  const viewKey = useMemo(
    () => generateViewKey({ ...definition, filter: apiFilter, search: undefined }),
    [apiFilter, definition],
  );
  const pageSize = definition.pageSize ?? 50;
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const sortBy = definition.sortBy ?? "date_captured";
  const request = useMemo<AssetQueryRequest>(
    () => ({
      filter: apiFilter,
      pagination: { limit: pageSize, offset: 0 },
      sort_by: normalizeAssetSort(definition.sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    }),
    [apiFilter, definition.sortBy, pageSize, viewerTimeZone],
  );
  const query = useInfiniteQuery({
    queryKey: ["post", "/api/v1/assets/list", { body: request }],
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await client.POST("/api/v1/assets/list", {
        body: withBodyPaginationOffset(request, Number(pageParam) || 0),
        signal,
      });
      if (error) throw error;
      return data;
    },
    enabled: autoFetch && !disabled,
    gcTime: 2 * 60 * 1000,
    initialPageParam: 0,
    getNextPageParam: (lastPage, _pages, lastPageParam) => {
      const total = lastPage?.total_visible;
      const offset = Number(lastPageParam ?? 0) || 0;
      const loadedCount = countLoadedBrowseRowsFromPage({ items: lastPage?.items });
      const hasMore =
        typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;
      return hasMore && loadedCount > 0 ? offset + loadedCount : undefined;
    },
  });
  const pages = useMemo(() => {
    const pageParams = query.data?.pageParams ?? [];
    return (query.data?.pages ?? []).map((page, index) => {
      const offset = Number(pageParams[index] ?? 0) || 0;
      const total = page?.total_visible;
      return {
        browseGroups: browseGroupsFromQueryLikePage({ items: page?.items, sortBy }),
        offset,
        total,
      };
    });
  }, [query.data?.pageParams, query.dataUpdatedAt, sortBy]);
  const browseGroups = useMemo(
    () => mergeAdjacentBrowseGroups(...pages.map((page) => page.browseGroups)),
    [pages],
  );
  const groups = useMemo(
    () =>
      browseGroups.map((group) => ({
        key: group.key,
        assets: group.items.map(getBrowseItemAsset),
      })),
    [browseGroups],
  );
  const assets = useMemo(() => flattenAssetGroups(groups), [groups]);
  const browseItems = useMemo(() => flattenBrowseGroups(browseGroups), [browseGroups]);
  const browseAssets = useMemo(() => flattenBrowseGroupsToAssets(browseGroups), [browseGroups]);
  const lastPage = pages.at(-1);
  const error =
    query.error instanceof Error
      ? query.error.message
      : query.error
        ? (JSON.stringify(query.error) ?? "Unknown error")
        : null;

  return {
    assets,
    groups,
    browseGroups,
    browseItems,
    browseAssets,
    isLoading: query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    hasMore: query.hasNextPage ?? true,
    fetchMore: async () => void (await query.fetchNextPage()),
    refetch: async () => void (await query.refetch()),
    isFetched: query.isFetched,
    error,
    viewKey,
    pageInfo: {
      page: lastPage ? Math.floor(lastPage.offset / pageSize) + 1 : 1,
      total: lastPage?.total,
    },
  };
}

export function useAssetsList(
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewResult {
  const result = useAssetsListQuery(definition, options);
  return {
    ...result,
    groups: options.withGroups ? result.groups : undefined,
  };
}
