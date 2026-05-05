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
  TabType,
  ViewDefinitionOptions,
} from "@/features/assets/types/assets.type";
import { generateViewKey } from "@/features/assets/utils/viewKey";
import {
  selectFilterAsAssetFilter,
  selectFiltersEnabled,
} from "@/features/assets/slices/filters.slice";
import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import type { Asset } from "@/lib/assets/types";
import { useWorkingRepository } from "@/features/settings";
import {
  flattenAssetGroups,
  groupAssetsBySort,
  getViewerTimeZone,
  mergeAdjacentAssetGroups,
} from "@/features/assets/utils/assetGroups";

type AssetQueryRequest = components["schemas"]["dto.AssetQueryRequestDTO"];
type AssetFilter = components["schemas"]["dto.AssetFilterDTO"];
type QueryAssetsResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];
type PersonAssetsApiResult = Omit<
  paths["/api/v1/people/{id}/assets/list"]["post"]["responses"][200]["content"]["application/json"],
  "data"
> & {
  data?: QueryAssetsResponse;
};

const countAssets = (assets: Asset[]) => assets.length;

const getApiMimeTypes = (
  tabTypes: TabType[],
): ("PHOTO" | "VIDEO" | "AUDIO")[] => {
  const mimeTypes: ("PHOTO" | "VIDEO" | "AUDIO")[] = [];
  tabTypes.forEach((type) => {
    switch (type) {
      case "photos":
        mimeTypes.push("PHOTO");
        break;
      case "videos":
        mimeTypes.push("VIDEO");
        break;
      case "audios":
        mimeTypes.push("AUDIO");
        break;
    }
  });
  return mimeTypes;
};

const normalizeVisibleAssets = (assets: Asset[]): Asset[] =>
  assets.filter((asset) => !asset.is_deleted && !asset.deleted_at);

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
  const currentTab = useAssetsStore((state) => state.ui.currentTab);
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

    const mimeTypes = getApiMimeTypes([currentTab]);
    if (mimeTypes.length === 1) {
      filter.type = mimeTypes[0];
    } else if (mimeTypes.length > 1) {
      filter.types = mimeTypes;
    }

    return filter;
  }, [currentTab, filtersState, scopedRepositoryId]);

  const requestBody = useMemo<AssetQueryRequest>(() => {
    const request: AssetQueryRequest = {
      filter: effectiveFilter,
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      sort_by: normalizeSearchSortBy(sortBy),
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
        types: [currentTab],
        filter: effectiveFilter,
        sortBy,
        search: searchQuery.trim() ? { query: searchQuery.trim() } : undefined,
        pageSize,
      })}`,
    [currentTab, effectiveFilter, pageSize, personId, searchQuery, sortBy],
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
        const responseData = (lastPage as PersonAssetsApiResult | undefined)?.data;
        const pageAssets = normalizeVisibleAssets(responseData?.assets ?? []);
        const total = responseData?.total;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countAssets(pageAssets);
        const hasMore =
          typeof total === "number"
            ? offset + loadedCount < total
            : loadedCount >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<PersonAssetsApiResult>, unknown>;

  const pages = useMemo(() => {
    const responsePages = (query.data?.pages ?? []) as PersonAssetsApiResult[];
    const pageParams = query.data?.pageParams ?? [];

    return responsePages.map((page, index) => {
      const offset = Number(pageParams[index] ?? 0) || 0;
      const responseData = page?.data;
      const assets = normalizeVisibleAssets(responseData?.assets ?? []);
      const groups = groupAssetsBySort(assets, sortBy);
      const total = responseData?.total;
      const loadedCount = countAssets(assets);
      const hasMore =
        typeof total === "number"
          ? offset + loadedCount < total
          : loadedCount >= pageSize;

      return { assets, groups, offset, total, hasMore };
    });
  }, [pageSize, query.data?.pageParams, query.dataUpdatedAt, sortBy]);

  const groups = useMemo(
    () => mergeAdjacentAssetGroups(...pages.map((page) => page.groups)),
    [pages],
  );
  const assets = useMemo(() => flattenAssetGroups(groups), [groups]);
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
    assets: normalizeVisibleAssets(assets),
    groups: withGroups ? groups : undefined,
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
