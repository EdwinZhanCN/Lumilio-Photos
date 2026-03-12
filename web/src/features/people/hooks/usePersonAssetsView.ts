import { useCallback, useMemo } from "react";
import type {
  InfiniteData,
  UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { keepPreviousData } from "@tanstack/react-query";
import { useAssetsStore } from "@/features/assets/assets.store";
import { useGroupBy } from "@/features/assets/selectors";
import {
  AssetGroup,
  AssetsViewResult,
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
  getViewerTimeZone,
  mergeAdjacentAssetGroups,
  normalizeAssetGroups,
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

const countGroupedAssets = (groups: AssetGroup[]) => flattenAssetGroups(groups).length;

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

const normalizeSearchGroupBy = (groupBy?: string): "date" | "type" | "flat" => {
  switch (groupBy) {
    case "type":
    case "flat":
      return groupBy;
    default:
      return "date";
  }
};

export function usePersonAssetsView(
  personId: number,
  options: ViewDefinitionOptions & {
    groupBy?: string;
    pageSize?: number;
  } = {},
): AssetsViewResult {
  const { scopedRepositoryId } = useWorkingRepository();
  const filtersState = useAssetsStore((state) => state.filters);
  const currentTab = useAssetsStore((state) => state.ui.currentTab);
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const uiGroupBy = useGroupBy();
  const { autoFetch = true, disabled = false, withGroups = false } = options;
  const pageSize = options.pageSize ?? 50;
  const groupBy = options.groupBy ?? uiGroupBy;
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
      group_by: normalizeSearchGroupBy(groupBy),
      viewer_timezone: viewerTimeZone,
    };

    if (searchQuery.trim()) {
      request.query = searchQuery.trim();
    }

    return request;
  }, [effectiveFilter, groupBy, pageSize, searchQuery, viewerTimeZone]);

  const viewKey = useMemo(
    () =>
      `person:${personId}:${generateViewKey({
        types: [currentTab],
        filter: effectiveFilter,
        groupBy: groupBy as any,
        search: searchQuery.trim() ? { query: searchQuery.trim() } : undefined,
        pageSize,
      })}`,
    [currentTab, effectiveFilter, groupBy, pageSize, personId, searchQuery],
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
      placeholderData: keepPreviousData,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = (lastPage as PersonAssetsApiResult | undefined)?.data;
        const pageGroups = normalizeAssetGroups(responseData?.groups);
        const total = responseData?.total;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countGroupedAssets(pageGroups);
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
      const groups = normalizeAssetGroups(responseData?.groups);
      const total = responseData?.total;
      const loadedCount = countGroupedAssets(groups);
      const hasMore =
        typeof total === "number"
          ? offset + loadedCount < total
          : loadedCount >= pageSize;

      return { groups, offset, total, hasMore };
    });
  }, [pageSize, query.data?.pageParams, query.dataUpdatedAt]);

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
