import { useCallback, useMemo } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import {
  buildApiFilter,
  DEFAULT_ASSET_TYPES,
  DEFAULT_TOP_RESULTS_META,
  normalizeSearchSortBy,
  normalizeTopResultsMeta,
  TOP_RESULTS_LIMIT,
  type SearchTopResultsMeta,
} from "./useAssetsView";
import { useFilterState } from "../state/selectors";
import { selectFilterAsAssetFilter, selectFiltersEnabled } from "../state/slices/filters.slice";
import type { AssetGroup, AssetsViewResult, BrowseGroup, SortByType } from "../types";
import { getViewerTimeZone } from "../utils/assetGroups";
import {
  browseGroupsFromQueryLikePage,
  browseGroupsFromSearchResultsPage,
  browseGroupsFromSearchTop,
  countLoadedBrowseRowsFromPage,
  dedupeBrowseItemsById,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  mergeAdjacentBrowseGroups,
} from "../utils/browseItems";
import { $api } from "@/lib/http-commons/queryClient";
import { client } from "@/lib/http-commons/client";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { Asset } from "@/lib/assets/types";
import { withBodyPaginationOffset } from "../utils/bodyPagination";

type AgentPinDTO = components["schemas"]["dto.AgentPinDTO"];
type AssetFilterDTO = components["schemas"]["dto.AssetFilterDTO"];
type AssetQueryRequestDTO = components["schemas"]["dto.AssetQueryRequestDTO"];
type SearchAssetsRequestDTO = components["schemas"]["dto.SearchAssetsRequestDTO"];

const PAGE_SIZE = 60;

export interface PinAssetsViewOptions {
  sortBy?: SortByType;
  pageSize?: number;
  baseFilter?: AssetFilterDTO;
  viewKey?: string;
  searchQuery?: string;
  searchEnabled?: boolean;
}

export interface PinAssetsViewResult extends AssetsViewResult {
  pin: AgentPinDTO | undefined;
  isExpired: boolean;
  topResults: Asset[];
  resultAssets: Asset[];
  resultGroups: AssetGroup[];
  topResultsBrowseGroups: BrowseGroup[];
  resultBrowseGroups: BrowseGroup[];
  topResultsMeta: SearchTopResultsMeta;
}

const EMPTY_VIEW: PinAssetsViewResult = {
  assets: [],
  groups: undefined,
  browseGroups: [],
  browseItems: [],
  browseAssets: [],
  isLoading: false,
  isLoadingMore: false,
  isFetched: false,
  error: null,
  fetchMore: async () => {},
  refetch: async () => {},
  hasMore: false,
  viewKey: "",
  pageInfo: { page: 1 },
  pin: undefined,
  isExpired: false,
  topResults: [],
  resultAssets: [],
  resultGroups: [],
  topResultsBrowseGroups: [],
  resultBrowseGroups: [],
  topResultsMeta: DEFAULT_TOP_RESULTS_META,
};

/** Pin-driven full gallery view. The board widget preview can keep using the
 * lightweight hydration endpoint; this hook is for `/assets?pin=...` and uses
 * the pin list/search endpoints so sort, filters, search, carousel, and
 * selection all operate over the same BrowseItem-shaped asset set. */
export function usePinAssetsView(
  pinId: string | undefined,
  options: PinAssetsViewOptions = {},
): PinAssetsViewResult {
  const enabled = Boolean(pinId);
  const pageSize = options.pageSize ?? PAGE_SIZE;
  const sortBy = options.sortBy ?? "date_captured";
  const queryText = options.searchEnabled === false ? "" : (options.searchQuery?.trim() ?? "");
  const isSearchActive = queryText.length > 0;
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const filtersState = useFilterState();
  const rawScopedFilter = useMemo(
    () => (selectFiltersEnabled(filtersState) ? selectFilterAsAssetFilter(filtersState) : {}),
    [filtersState],
  );
  const scopedFilter = useMemo(
    () => ({
      ...rawScopedFilter,
      ...options.baseFilter,
    }),
    [options.baseFilter, rawScopedFilter],
  );
  const apiFilter = useMemo(
    () =>
      buildApiFilter(
        {
          types: DEFAULT_ASSET_TYPES,
          filter: scopedFilter,
          sortBy,
          pageSize,
          key: options.viewKey,
          search: isSearchActive ? { query: queryText } : undefined,
        },
        scopedFilter,
      ),
    [isSearchActive, options.viewKey, pageSize, queryText, scopedFilter, sortBy],
  );

  const pinMetaQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}",
    { params: { path: { id: pinId ?? "" } } },
    { enabled, retry: false, staleTime: 60_000 },
  );

  const createListRequest = useCallback(
    (): AssetQueryRequestDTO => ({
      filter: apiFilter,
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      sort_by: normalizeSearchSortBy(sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    }),
    [apiFilter, pageSize, sortBy, viewerTimeZone],
  );

  const listRequest = useMemo(() => createListRequest(), [createListRequest]);
  const listQuery = useInfiniteQuery({
    queryKey: [
      "post",
      "/api/v1/agent/pins/{id}/assets/list",
      { params: { path: { id: pinId ?? "" } }, body: listRequest },
    ],
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await client.POST("/api/v1/agent/pins/{id}/assets/list", {
        params: { path: { id: pinId ?? "" } },
        body: withBodyPaginationOffset(listRequest, Number(pageParam) || 0),
        signal,
      });
      if (error) throw error;
      return data;
    },
    enabled: enabled && !isSearchActive,
    initialPageParam: 0,
    retry: false,
    gcTime: 2 * 60 * 1000,
    getNextPageParam: (lastPage, _allPages, lastPageParam) => {
      const payload = lastPage;
      const total = payload?.total_visible;
      const offset = Number(lastPageParam ?? 0) || 0;
      const loadedCount = countLoadedBrowseRowsFromPage({
        items: payload?.items,
      });
      const hasMore =
        typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;
      return hasMore && loadedCount > 0 ? offset + loadedCount : undefined;
    },
  });

  const createSearchRequest = useCallback(
    (): SearchAssetsRequestDTO => ({
      query: queryText,
      filter: apiFilter,
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      enhancement_mode: "auto",
      top_results_limit: TOP_RESULTS_LIMIT,
      sort_by: normalizeSearchSortBy(sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    }),
    [apiFilter, pageSize, queryText, sortBy, viewerTimeZone],
  );

  const searchRequest = useMemo(() => createSearchRequest(), [createSearchRequest]);
  const searchQuery = useInfiniteQuery({
    queryKey: [
      "post",
      "/api/v1/agent/pins/{id}/assets/search",
      { params: { path: { id: pinId ?? "" } }, body: searchRequest },
    ],
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await client.POST("/api/v1/agent/pins/{id}/assets/search", {
        params: { path: { id: pinId ?? "" } },
        body: withBodyPaginationOffset(searchRequest, Number(pageParam) || 0),
        signal,
      });
      if (error) throw error;
      return data;
    },
    enabled: enabled && isSearchActive,
    initialPageParam: 0,
    retry: false,
    gcTime: 2 * 60 * 1000,
    getNextPageParam: (lastPage, _allPages, lastPageParam) => {
      const payload = lastPage;
      const total = payload?.results_total_visible;
      const offset = Number(lastPageParam ?? 0) || 0;
      const loadedCount = countLoadedBrowseRowsFromPage({
        items: payload?.result_items,
      });
      const hasMore =
        typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;
      return hasMore && loadedCount > 0 ? offset + loadedCount : undefined;
    },
  });

  const pin = pinMetaQuery.data;
  const isExpired = enabled && pinMetaQuery.isError;

  const listPages = useMemo(() => {
    const pages = listQuery.data?.pages ?? [];
    const pageParams = listQuery.data?.pageParams ?? [];
    return pages.map((page, index) => {
      const offset = Number(pageParams[index] ?? 0) || 0;
      const browseGroups = browseGroupsFromQueryLikePage({
        items: page.items,
        sortBy,
      });
      return {
        browseGroups,
        offset,
        total: page.total_visible,
      };
    });
  }, [listQuery.data?.pageParams, listQuery.dataUpdatedAt, sortBy]);

  const listBrowseGroups = useMemo(
    () => mergeAdjacentBrowseGroups(...listPages.map((page) => page.browseGroups)),
    [listPages],
  );
  const listBrowseItems = useMemo(() => flattenBrowseGroups(listBrowseGroups), [listBrowseGroups]);
  const listBrowseAssets = useMemo(
    () => flattenBrowseGroupsToAssets(listBrowseGroups),
    [listBrowseGroups],
  );
  const listGroups = useMemo<AssetGroup[]>(
    () =>
      listBrowseGroups.map((group) => ({
        key: group.key,
        assets: group.items.map(getBrowseItemAsset),
      })),
    [listBrowseGroups],
  );

  const searchPages = useMemo(() => {
    const pages = searchQuery.data?.pages ?? [];
    const pageParams = searchQuery.data?.pageParams ?? [];
    return pages.map((page, index) => ({
      topItems: page.top_items,
      topResultsMeta: normalizeTopResultsMeta(page.top_results_meta),
      resultItems: page.result_items,
      total: page.results_total_visible,
      offset: Number(pageParams[index] ?? 0) || 0,
    }));
  }, [searchQuery.data?.pageParams, searchQuery.dataUpdatedAt]);
  const firstSearchPage = searchPages[0];
  const topResultsBrowseGroups = useMemo(
    () => browseGroupsFromSearchTop({ topItems: firstSearchPage?.topItems }),
    [firstSearchPage?.topItems],
  );
  const resultBrowseGroups = useMemo(
    () =>
      mergeAdjacentBrowseGroups(
        ...searchPages.map((page) =>
          browseGroupsFromSearchResultsPage({
            resultItems: page.resultItems,
          }),
        ),
      ),
    [searchPages],
  );
  const topResults = useMemo(
    () => flattenBrowseGroupsToAssets(topResultsBrowseGroups),
    [topResultsBrowseGroups],
  );
  const resultAssets = useMemo(
    () => flattenBrowseGroupsToAssets(resultBrowseGroups),
    [resultBrowseGroups],
  );
  const resultGroups = useMemo<AssetGroup[]>(
    () => (resultAssets.length > 0 ? [{ key: "pin-search:results", assets: resultAssets }] : []),
    [resultAssets],
  );
  const searchBrowseItems = useMemo(
    () =>
      dedupeBrowseItemsById([
        ...flattenBrowseGroups(topResultsBrowseGroups),
        ...flattenBrowseGroups(resultBrowseGroups),
      ]),
    [resultBrowseGroups, topResultsBrowseGroups],
  );
  const searchBrowseAssets = useMemo(
    () => searchBrowseItems.map(getBrowseItemAsset),
    [searchBrowseItems],
  );
  const searchAssets = useMemo(() => {
    const seen = new Set<string>();
    const merged: Asset[] = [];
    [...topResults, ...resultAssets].forEach((asset) => {
      if (!asset.asset_id || seen.has(asset.asset_id)) {
        return;
      }
      seen.add(asset.asset_id);
      merged.push(asset);
    });
    return merged;
  }, [resultAssets, topResults]);

  const activeQuery = isSearchActive ? searchQuery : listQuery;
  const activeError = activeQuery.error ?? pinMetaQuery.error;
  const errorMessage = useMemo<string | null>(() => {
    if (!enabled || !activeError) return null;
    if (activeError instanceof Error) return activeError.message;
    if (typeof activeError === "string") return activeError;
    return JSON.stringify(activeError) ?? "Unknown error";
  }, [activeError, enabled]);

  const listLastPage = listPages.length > 0 ? listPages[listPages.length - 1] : undefined;
  const searchLastPage = searchPages.length > 0 ? searchPages[searchPages.length - 1] : undefined;
  const pageInfo = useMemo(() => {
    const lastPage = isSearchActive ? searchLastPage : listLastPage;
    return {
      page: lastPage ? Math.floor(lastPage.offset / pageSize) + 1 : 1,
      total: lastPage?.total,
    };
  }, [isSearchActive, listLastPage, pageSize, searchLastPage]);

  if (!enabled) {
    return EMPTY_VIEW;
  }

  return {
    assets: isSearchActive ? searchAssets : listBrowseAssets,
    groups: isSearchActive ? resultGroups : listGroups,
    browseGroups: isSearchActive ? resultBrowseGroups : listBrowseGroups,
    browseItems: isSearchActive ? searchBrowseItems : listBrowseItems,
    browseAssets: isSearchActive ? searchBrowseAssets : listBrowseAssets,
    isLoading: activeQuery.isLoading,
    isLoadingMore: activeQuery.isFetchingNextPage,
    isFetched: activeQuery.isFetched,
    error: errorMessage,
    fetchMore: async () => {
      await activeQuery.fetchNextPage();
    },
    refetch: async () => {
      await Promise.all([pinMetaQuery.refetch(), activeQuery.refetch()]);
    },
    hasMore: activeQuery.hasNextPage ?? false,
    viewKey: `assets:pin:${pinId}:${options.viewKey ?? ""}:${sortBy}:${queryText}`,
    pageInfo,
    pin,
    isExpired,
    topResults,
    resultAssets,
    resultGroups,
    topResultsBrowseGroups,
    resultBrowseGroups,
    topResultsMeta: firstSearchPage?.topResultsMeta ?? DEFAULT_TOP_RESULTS_META,
  };
}
