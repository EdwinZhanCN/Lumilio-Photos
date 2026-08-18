import { useMemo } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { authenticatedFetch, client } from "@/lib/http-commons/client";
import { normalizeProblem, readProblemResponse } from "@/lib/http-commons/problem";
import type { components } from "@/lib/http-commons/schema.d.ts";
import type { AssetViewDefinition, AssetsViewResult, ViewDefinitionOptions } from "../types";
import { getViewerTimeZone } from "../model/assetGroups";
import { fileQueryIdentity, throwSearchError } from "../model/visualSearch";
import { withBodyPaginationOffset } from "./bodyPagination";
import {
  browseGroupsFromSearchResultsPage,
  browseGroupsFromSearchTop,
  countLoadedBrowseRowsFromPage,
  dedupeBrowseItemsById,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  mergeAdjacentBrowseGroups,
} from "../model/browseItems";
import { generateViewKey } from "../model/viewKey";
import {
  buildAssetApiFilter,
  DEFAULT_TOP_RESULTS_META,
  mergeUniqueAssets,
  normalizeAssetSort,
  normalizeTopResultsMeta,
  TOP_RESULTS_LIMIT,
  type AssetBrowserViewResult,
} from "./assetViewModel";

type SearchAssetsRequest = components["schemas"]["dto.SearchAssetsRequestDTO"];
type SearchAssetsResponse = components["schemas"]["dto.SearchAssetsResponseDTO"];

const EMPTY_VIEW: AssetsViewResult = {
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
};

const EMPTY_SEARCH_VIEW: AssetBrowserViewResult = {
  ...EMPTY_VIEW,
  topResults: [],
  resultAssets: [],
  resultGroups: [],
  topResultsBrowseGroups: [],
  resultBrowseGroups: [],
  topResultsMeta: DEFAULT_TOP_RESULTS_META,
};

export function useAssetsSearch(
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetBrowserViewResult {
  const { autoFetch = true, disabled = false, withGroups = false, fileQuery = null } = options;
  const apiFilter = useMemo(
    () => buildAssetApiFilter(definition, definition.filter ?? {}),
    [definition],
  );
  const pageSize = definition.pageSize ?? 50;
  const queryText = definition.search?.query?.trim() ?? "";
  const similarAssetId = definition.search?.similarAssetId?.trim() ?? "";
  const fileIdentity = fileQueryIdentity(fileQuery);
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const viewKey = useMemo(
    () => `${generateViewKey({ ...definition, filter: apiFilter })}:asset-search`,
    [apiFilter, definition],
  );
  const searchActive = queryText.length > 0 || similarAssetId.length > 0 || Boolean(fileQuery);
  const request = useMemo<SearchAssetsRequest>(() => {
    const base: SearchAssetsRequest = {
      filter: apiFilter,
      pagination: { limit: pageSize, offset: 0 },
      top_results_limit: TOP_RESULTS_LIMIT,
      sort_by: normalizeAssetSort(definition.sortBy),
      viewer_timezone: viewerTimeZone,
    };
    if (similarAssetId) {
      return { ...base, similar_to_asset_id: similarAssetId };
    }
    return { ...base, query: queryText, enhancement_mode: "auto" };
  }, [apiFilter, definition.sortBy, pageSize, queryText, similarAssetId, viewerTimeZone]);
  const query = useInfiniteQuery({
    queryKey: fileQuery
      ? ["post", "/api/v1/assets/search/by-image", { filter: apiFilter, file: fileIdentity }]
      : ["post", "/api/v1/assets/search", { body: request }],
    queryFn: async ({ pageParam, signal }) => {
      const offset = Number(pageParam) || 0;
      if (fileQuery) {
        return searchByImage({
          file: fileQuery,
          filter: apiFilter,
          limit: pageSize,
          offset,
          viewerTimeZone,
          signal,
        });
      }
      const { data, error } = await client.POST("/api/v1/assets/search", {
        body: withBodyPaginationOffset(request, offset),
        signal,
      });
      if (error) throwSearchError(error);
      return data;
    },
    enabled: autoFetch && !disabled && searchActive,
    gcTime: 2 * 60 * 1000,
    initialPageParam: 0,
    getNextPageParam: (lastPage, _pages, lastPageParam) => {
      const total = lastPage?.results_total_visible;
      const offset = Number(lastPageParam ?? 0) || 0;
      const loadedCount = countLoadedBrowseRowsFromPage({ items: lastPage?.result_items });
      const hasMore =
        typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;
      return hasMore && loadedCount > 0 ? offset + loadedCount : undefined;
    },
  });
  const pages = useMemo(() => {
    const pageParams = query.data?.pageParams ?? [];
    return (query.data?.pages ?? []).map((page, index) => ({
      topItems: page?.top_items,
      topResultsMeta: normalizeTopResultsMeta(page?.top_results_meta),
      resultItems: page?.result_items,
      total: page?.results_total_visible,
      offset: Number(pageParams[index] ?? 0) || 0,
    }));
  }, [query.data?.pageParams, query.dataUpdatedAt]);
  const firstPage = pages[0];
  const topResultsBrowseGroups = useMemo(
    () => browseGroupsFromSearchTop({ topItems: firstPage?.topItems }),
    [firstPage?.topItems],
  );
  const resultBrowseGroups = useMemo(
    () =>
      mergeAdjacentBrowseGroups(
        ...pages.map((page) =>
          browseGroupsFromSearchResultsPage({ resultItems: page.resultItems }),
        ),
      ),
    [pages],
  );
  const topResults = useMemo(
    () => flattenBrowseGroupsToAssets(topResultsBrowseGroups),
    [topResultsBrowseGroups],
  );
  const resultAssets = useMemo(
    () => flattenBrowseGroupsToAssets(resultBrowseGroups),
    [resultBrowseGroups],
  );
  const resultGroups = useMemo(
    () => (resultAssets.length ? [{ key: "search:results", assets: resultAssets }] : []),
    [resultAssets],
  );
  const browseItems = useMemo(
    () =>
      dedupeBrowseItemsById([
        ...flattenBrowseGroups(topResultsBrowseGroups),
        ...flattenBrowseGroups(resultBrowseGroups),
      ]),
    [resultBrowseGroups, topResultsBrowseGroups],
  );
  const browseAssets = useMemo(() => browseItems.map(getBrowseItemAsset), [browseItems]);
  const assets = useMemo(
    () => mergeUniqueAssets(topResults, resultAssets),
    [resultAssets, topResults],
  );
  const lastPage = pages.at(-1);
  const error = query.error ?? null;

  if (!searchActive) return { ...EMPTY_SEARCH_VIEW, viewKey };
  return {
    assets,
    browseGroups: resultBrowseGroups,
    browseItems,
    browseAssets,
    topResults,
    resultAssets,
    resultGroups,
    topResultsBrowseGroups,
    resultBrowseGroups,
    topResultsMeta: firstPage?.topResultsMeta ?? DEFAULT_TOP_RESULTS_META,
    groups: withGroups ? resultGroups : undefined,
    isLoading: query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    error,
    fetchMore: async () => void (await query.fetchNextPage()),
    refetch: async () => void (await query.refetch()),
    hasMore: query.hasNextPage ?? false,
    isFetched: query.isFetched,
    viewKey,
    pageInfo: {
      page: lastPage ? Math.floor(lastPage.offset / pageSize) + 1 : 1,
      total: lastPage?.total,
    },
  };
}

async function searchByImage({
  file,
  filter,
  limit,
  offset,
  viewerTimeZone,
  signal,
}: {
  file: File;
  filter: SearchAssetsRequest["filter"];
  limit: number;
  offset: number;
  viewerTimeZone: string;
  signal?: AbortSignal;
}): Promise<SearchAssetsResponse> {
  const body = new FormData();
  body.append("file", file);
  if (filter && Object.keys(filter).length > 0) {
    body.append("filter", JSON.stringify(filter));
  }
  body.append("limit", String(limit));
  body.append("offset", String(offset));
  body.append("top_results_limit", String(TOP_RESULTS_LIMIT));
  body.append("viewer_timezone", viewerTimeZone);
  const response = await authenticatedFetch("/api/v1/assets/search/by-image", {
    method: "POST",
    body,
    signal,
  });
  if (!response.ok) throw await readProblemResponse(response);
  const payload: unknown = await response.json().catch(() => undefined);
  if (!isSearchAssetsResponse(payload)) throw normalizeProblem(undefined);
  return payload;
}

function isSearchAssetsResponse(value: unknown): value is SearchAssetsResponse {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}
