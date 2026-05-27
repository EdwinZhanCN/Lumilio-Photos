import { useCallback, useMemo } from "react";
import type {
  InfiniteData,
  UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useAssetsStore } from "@/features/assets/assets.store";
import { useSortBy } from "@/features/assets/selectors";
import {
  AssetsViewResult,
  SortByType,
  ViewDefinitionOptions,
} from "@/features/assets/types/assets.type";
import { generateViewKey } from "@/features/assets/utils/viewKey";
import {
  selectFilterAsAssetFilter,
  selectFiltersEnabled,
} from "@/features/assets/slices/filters.slice";
import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import { useWorkingRepository } from "@/features/settings";
import {
  flattenAssetGroups,
  getViewerTimeZone,
} from "@/features/assets/utils/assetGroups";
import {
  browseGroupsFromQueryLikePage,
  countLoadedBrowseRowsFromPage,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  mergeAdjacentBrowseGroups,
} from "@/features/assets/utils/browseItems";

type AssetQueryRequest = components["schemas"]["dto.AssetQueryRequestDTO"];
type AssetFilter = components["schemas"]["dto.AssetFilterDTO"];
type QueryAssetsResponseDTO =
  components["schemas"]["dto.QueryAssetsResponseDTO"];

type PersonAssetsApiEnvelope = Omit<
  paths["/api/v1/people/{id}/assets/list"]["post"]["responses"][200]["content"]["application/json"],
  "data"
> & {
  data?: QueryAssetsResponseDTO;
};

const normalizeSearchSortBy = (
  sortBy?: SortByType,
): "recently_added" | "date_captured" => {
  switch (sortBy) {
    case "recently_added":
      return sortBy;
    default:
      return "date_captured";
  }
};

export function usePersonAssetsView(
  personId: number,
  options: ViewDefinitionOptions & {
    sortBy?: SortByType;
    pageSize?: number;
  } = {},
): AssetsViewResult {
  const { scopedRepositoryId } = useWorkingRepository();
  const filtersState = useAssetsStore((state) => state.filters);
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const uiSortBy = useSortBy();
  const { autoFetch = true, disabled = false, withGroups = false } = options;
  const pageSize = options.pageSize ?? 50;
  const sortBy = options.sortBy ?? uiSortBy;
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);

  const effectiveFilter = useMemo(() => {
    const globalFilter = selectFiltersEnabled({ filters: filtersState } as any)
      ? selectFilterAsAssetFilter({ filters: filtersState } as any)
      : {};

    const filter: AssetFilter = {
      ...globalFilter,
    };

    if (filter.repository_id === undefined && scopedRepositoryId) {
      filter.repository_id = scopedRepositoryId;
    }

    return filter;
  }, [filtersState, scopedRepositoryId]);

  const requestBody = useMemo<AssetQueryRequest>(() => {
    const request: AssetQueryRequest = {
      filter: effectiveFilter,
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      sort_by: normalizeSearchSortBy(sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    };

    if (searchQuery.trim()) {
      request.query = searchQuery.trim();
    }

    return request;
  }, [effectiveFilter, pageSize, searchQuery, sortBy, viewerTimeZone]);

  const viewKey = useMemo(
    () =>
      `person:${personId}:${generateViewKey({
        filter: effectiveFilter,
        sortBy,
        search: searchQuery.trim() ? { query: searchQuery.trim() } : undefined,
        pageSize,
      })}`,
    [effectiveFilter, pageSize, personId, searchQuery, sortBy],
  );

  const query = $api.useInfiniteQuery(
    "post",
    "/api/v1/people/{id}/assets/list",
    {
      params: {
        path: {
          id: personId,
        },
      },
      body: requestBody,
    },
    {
      enabled: autoFetch && !disabled && personId > 0,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = (lastPage as PersonAssetsApiEnvelope | undefined)?.data;
        const total = responseData?.total_visible;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countLoadedBrowseRowsFromPage({
          items: responseData?.items,
        });
        const hasMore =
          typeof total === "number"
            ? offset + loadedCount < total
            : loadedCount >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<PersonAssetsApiEnvelope>, unknown>;

  const pages = useMemo(() => {
    const responsePages = (query.data?.pages ?? []) as PersonAssetsApiEnvelope[];
    const pageParams = query.data?.pageParams ?? [];

    return responsePages.map((page, index) => {
      const offset = Number(pageParams[index] ?? 0) || 0;
      const responseData = page?.data;
      const browseGroups = browseGroupsFromQueryLikePage({
        items: responseData?.items,
        sortBy,
      });
      const total = responseData?.total_visible;
      const loadedCount = countLoadedBrowseRowsFromPage({
        items: responseData?.items,
      });
      const hasMore =
        typeof total === "number"
          ? offset + loadedCount < total
          : loadedCount >= pageSize;

      return { browseGroups, offset, total, hasMore };
    });
  }, [pageSize, query.data?.pageParams, query.dataUpdatedAt, sortBy]);

  const browseGroups = useMemo(
    () => mergeAdjacentBrowseGroups(...pages.map((page) => page.browseGroups)),
    [pages],
  );
  const groups = useMemo(
    () =>
      browseGroups.map((bg) => ({
        key: bg.key,
        assets: bg.items.map(getBrowseItemAsset),
      })),
    [browseGroups],
  );
  const assets = useMemo(() => flattenAssetGroups(groups), [groups]);
  const browseItems = useMemo(
    () => flattenBrowseGroups(browseGroups),
    [browseGroups],
  );
  const browseAssets = useMemo(
    () => flattenBrowseGroupsToAssets(browseGroups),
    [browseGroups],
  );
  const lastPage = pages.length > 0 ? pages[pages.length - 1] : undefined;
  const pageInfo = useMemo(
    () => ({
      cursor: undefined,
      page: lastPage ? Math.floor(lastPage.offset / pageSize) + 1 : 1,
      total: lastPage?.total,
    }),
    [lastPage, pageSize],
  );

  const error =
    query.error instanceof Error
      ? query.error.message
      : query.error
        ? String(query.error)
        : null;

  const fetchMore = useCallback(async () => {
    await query.fetchNextPage();
  }, [query]);

  const refetch = useCallback(async () => {
    await query.refetch();
  }, [query]);

  return {
    assets,
    groups: withGroups ? groups : undefined,
    browseGroups,
    browseItems,
    browseAssets,
    isLoading: query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    error,
    fetchMore,
    refetch,
    hasMore: query.hasNextPage ?? true,
    isFetched: query.isFetched,
    viewKey,
    pageInfo,
  };
}
