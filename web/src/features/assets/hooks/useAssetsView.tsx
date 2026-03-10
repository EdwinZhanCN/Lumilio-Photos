import { useCallback, useMemo } from "react";
import type {
  InfiniteData,
  UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { keepPreviousData } from "@tanstack/react-query";
import { useAssetsStore } from "../assets.store";
import { useGroupBy } from "../selectors";
import {
  AssetGroup,
  AssetViewDefinition,
  AssetsViewResult,
  TabType,
  ViewDefinitionOptions,
} from "@/features/assets/types/assets.type";
import { generateViewKey } from "../utils/viewKey";
import {
  selectFilterAsAssetFilter,
  selectFiltersEnabled,
} from "../slices/filters.slice";
import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import { Asset } from "@/lib/assets/types";
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
type SearchAssetsRequest = components["schemas"]["dto.SearchAssetsRequestDTO"];
type SearchAssetsResponse =
  components["schemas"]["dto.SearchAssetsResponseDTO"];
type QueryAssetsApiResult = Omit<
  paths["/api/v1/assets/list"]["post"]["responses"][200]["content"]["application/json"],
  "data"
> & {
  data?: QueryAssetsResponse;
};
type SearchAssetsApiResult = Omit<
  paths["/api/v1/assets/search"]["post"]["responses"][200]["content"]["application/json"],
  "data"
> & {
  data?: SearchAssetsResponse;
};

type SearchTopResultsMeta = {
  enabled: boolean;
  degraded: boolean;
  reason?: string;
  source_types: string[];
};

type AssetsViewQueryResult = {
  assets: Asset[];
  groups: AssetGroup[];
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

type PhotoSearchViewResult = AssetsViewResult & {
  topResults: Asset[];
  resultAssets: Asset[];
  resultGroups: AssetGroup[];
  topResultsMeta: SearchTopResultsMeta;
};

const DEFAULT_TOP_RESULTS_META: SearchTopResultsMeta = {
  enabled: false,
  degraded: false,
  source_types: [],
};

const EMPTY_ASSETS_VIEW_RESULT: AssetsViewResult = {
  assets: [],
  groups: undefined,
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

const EMPTY_PHOTO_SEARCH_VIEW_RESULT: PhotoSearchViewResult = {
  ...EMPTY_ASSETS_VIEW_RESULT,
  topResults: [],
  resultAssets: [],
  resultGroups: [],
  topResultsMeta: DEFAULT_TOP_RESULTS_META,
};

const TOP_RESULTS_LIMIT = 12;

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

const mergeUniqueAssets = (...assetCollections: Asset[][]): Asset[] => {
  const seen = new Set<string>();
  const merged: Asset[] = [];

  assetCollections.forEach((collection) => {
    collection.forEach((asset) => {
      const key = asset.asset_id;
      if (!key || seen.has(key)) {
        return;
      }
      seen.add(key);
      merged.push(asset);
    });
  });

  return merged;
};

const normalizeTopResultsMeta = (
  meta?: SearchAssetsResponse["top_results_meta"],
): SearchTopResultsMeta => ({
  enabled: Boolean(meta?.enabled),
  degraded: Boolean(meta?.degraded),
  reason: meta?.reason,
  source_types: meta?.source_types ?? [],
});

const normalizeSearchGroupBy = (
  groupBy?: AssetViewDefinition["groupBy"],
): SearchAssetsRequest["group_by"] => {
  switch (groupBy) {
    case "date":
    case "type":
    case "flat":
      return groupBy;
    default:
      return "flat";
  }
};

const useEffectiveFilter = (definition: AssetViewDefinition): AssetFilter => {
  const filtersState = useAssetsStore((state) => state.filters);
  const { scopedRepositoryId } = useWorkingRepository();

  return useMemo(() => {
    const globalFilter = selectFiltersEnabled({ filters: filtersState } as any)
      ? selectFilterAsAssetFilter({ filters: filtersState } as any)
      : {};

    const baseFilter =
      definition.inheritGlobalFilter !== false ? globalFilter : {};

    const mergedFilter: AssetFilter = {
      ...baseFilter,
      ...definition.filter,
    };

    if (mergedFilter.repository_id === undefined && scopedRepositoryId) {
      mergedFilter.repository_id = scopedRepositoryId;
    }

    return mergedFilter;
  }, [
    definition.filter,
    definition.inheritGlobalFilter,
    filtersState,
    scopedRepositoryId,
  ]);
};

const buildApiFilter = (
  definition: AssetViewDefinition,
  effectiveFilter: AssetFilter,
): AssetFilter => {
  const filter: AssetFilter = { ...effectiveFilter };

  if (definition.types && definition.types.length > 0) {
    const mimeTypes = getApiMimeTypes(definition.types);
    if (mimeTypes.length === 1) {
      filter.type = mimeTypes[0];
    } else if (mimeTypes.length > 1) {
      filter.types = mimeTypes;
    }
  }

  return filter;
};

const countGroupedAssets = (groups: AssetGroup[]) => flattenAssetGroups(groups).length;

export const useAssetsViewQuery = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewQueryResult => {
  const { autoFetch = true, disabled = false } = options;
  const viewKey = useMemo(() => generateViewKey(definition), [definition]);
  const effectiveFilter = useEffectiveFilter(definition);
  const pageSize = definition.pageSize || 50;
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);

  const createUnifiedRequest = useCallback((): AssetQueryRequest => {
    const request: AssetQueryRequest = {
      filter: buildApiFilter(definition, effectiveFilter),
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      group_by: normalizeSearchGroupBy(definition.groupBy),
      viewer_timezone: viewerTimeZone,
    };

    if (definition.search?.query) {
      request.query = definition.search.query;
    }

    return request;
  }, [definition, effectiveFilter, pageSize, viewerTimeZone]);

  const query = $api.useInfiniteQuery(
    "post",
    "/api/v1/assets/list",
    {
      body: createUnifiedRequest(),
    },
    {
      enabled: autoFetch && !disabled,
      placeholderData: keepPreviousData,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = (lastPage as QueryAssetsApiResult | undefined)?.data;
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
  ) as UseInfiniteQueryResult<InfiniteData<QueryAssetsApiResult>, unknown>;

  const assetsPages = useMemo(() => {
    const pages = (query.data?.pages ?? []) as QueryAssetsApiResult[];
    const pageParams = query.data?.pageParams ?? [];

    return pages.map((page, index) => {
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
  }, [query.data?.pageParams, query.dataUpdatedAt, pageSize]);

  const groups = useMemo(
    () => mergeAdjacentAssetGroups(...assetsPages.map((page) => page.groups)),
    [assetsPages],
  );
  const assets = useMemo(() => flattenAssetGroups(groups), [groups]);

  const lastPage =
    assetsPages.length > 0 ? assetsPages[assetsPages.length - 1] : undefined;
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

  return {
    assets,
    groups,
    isLoading: query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    hasMore: query.hasNextPage ?? true,
    fetchMore: async () => {
      await query.fetchNextPage();
    },
    refetch: async () => {
      await query.refetch();
    },
    isFetched: query.isFetched,
    error,
    viewKey,
    pageInfo,
  };
};

export const useAssetsView = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewResult => {
  const { withGroups = false } = options;
  const queryResult = useAssetsViewQuery(definition, options);
  const assets = useMemo(
    () => normalizeVisibleAssets(queryResult.assets),
    [queryResult.assets],
  );

  return {
    assets,
    groups: withGroups ? queryResult.groups : undefined,
    isLoading: queryResult.isLoading,
    isLoadingMore: queryResult.isLoadingMore,
    error: queryResult.error,
    fetchMore: queryResult.fetchMore,
    refetch: queryResult.refetch,
    hasMore: queryResult.hasMore,
    isFetched: queryResult.isFetched,
    viewKey: queryResult.viewKey,
    pageInfo: queryResult.pageInfo,
  };
};

export const usePhotoSearchView = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): PhotoSearchViewResult => {
  const { autoFetch = true, disabled = false, withGroups = false } = options;
  const effectiveFilter = useEffectiveFilter(definition);
  const pageSize = definition.pageSize || 50;
  const queryText = definition.search?.query?.trim() ?? "";
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const viewKey = useMemo(
    () => `${generateViewKey(definition)}:photo-search`,
    [definition],
  );

  const createSearchRequest = useCallback(
    (): SearchAssetsRequest => ({
      query: queryText,
      filter: buildApiFilter(definition, effectiveFilter),
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      enhancement_mode: "auto",
      top_results_limit: TOP_RESULTS_LIMIT,
      group_by: normalizeSearchGroupBy(definition.groupBy),
      viewer_timezone: viewerTimeZone,
    }),
    [definition, effectiveFilter, pageSize, queryText, viewerTimeZone],
  );

  const query = $api.useInfiniteQuery(
    "post",
    "/api/v1/assets/search",
    {
      body: createSearchRequest(),
    },
    {
      enabled: autoFetch && !disabled && queryText.length > 0,
      placeholderData: keepPreviousData,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = (lastPage as SearchAssetsApiResult | undefined)?.data;
        const pageGroups = normalizeAssetGroups(responseData?.result_groups);
        const total = responseData?.results_total;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countGroupedAssets(pageGroups);
        const hasMore =
          typeof total === "number"
            ? offset + loadedCount < total
            : loadedCount >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  ) as UseInfiniteQueryResult<InfiniteData<SearchAssetsApiResult>, unknown>;

  const searchPages = useMemo(() => {
    const pages = (query.data?.pages ?? []) as SearchAssetsApiResult[];
    const pageParams = query.data?.pageParams ?? [];

    return pages.map((page, index) => {
      const responseData = page?.data;
      const offset = Number(pageParams[index] ?? 0) || 0;
      const resultGroups = normalizeAssetGroups(responseData?.result_groups);
      const total = responseData?.results_total;
      const loadedCount = countGroupedAssets(resultGroups);
      const hasMore =
        typeof total === "number"
          ? offset + loadedCount < total
          : loadedCount >= pageSize;

      return {
        topResults: responseData?.top_results ?? [],
        topResultsMeta: normalizeTopResultsMeta(responseData?.top_results_meta),
        resultGroups,
        total,
        offset,
        hasMore,
      };
    });
  }, [pageSize, query.data?.pageParams, query.dataUpdatedAt]);

  const firstPage = searchPages[0];
  const topResults = useMemo(
    () => normalizeVisibleAssets(firstPage?.topResults ?? []),
    [firstPage?.topResults],
  );
  const resultGroups = useMemo(
    () =>
      mergeAdjacentAssetGroups(...searchPages.map((page) => page.resultGroups)),
    [searchPages],
  );
  const resultAssets = useMemo(
    () => flattenAssetGroups(resultGroups),
    [resultGroups],
  );
  const assets = useMemo(
    () => mergeUniqueAssets(topResults, resultAssets),
    [resultAssets, topResults],
  );

  const lastPage =
    searchPages.length > 0 ? searchPages[searchPages.length - 1] : undefined;
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

  if (queryText.length === 0) {
    return {
      ...EMPTY_PHOTO_SEARCH_VIEW_RESULT,
      viewKey,
    };
  }

  return {
    assets,
    topResults,
    resultAssets,
    resultGroups,
    topResultsMeta: firstPage?.topResultsMeta ?? DEFAULT_TOP_RESULTS_META,
    groups: withGroups ? resultGroups : undefined,
    isLoading: query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    error,
    fetchMore: async () => {
      await query.fetchNextPage();
    },
    refetch: async () => {
      await query.refetch();
    },
    hasMore: query.hasNextPage ?? false,
    isFetched: query.isFetched,
    viewKey,
    pageInfo,
  };
};

export const useCurrentTabPhotoSearchView = (
  options: ViewDefinitionOptions & {
    groupBy?: string;
    pageSize?: number;
  } = {},
): PhotoSearchViewResult => {
  const currentTab = useAssetsStore((state) => state.ui.currentTab);
  const uiGroupBy = useGroupBy();
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const { groupBy, pageSize, ...viewOptions } = options;

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: [currentTab],
      groupBy: (groupBy as any) || uiGroupBy,
      pageSize: pageSize || 50,
      sort: { direction: "desc" },
      search: searchQuery.trim()
        ? {
            query: searchQuery.trim(),
          }
        : undefined,
    }),
    [currentTab, uiGroupBy, searchQuery, groupBy, pageSize],
  );

  const enabled = currentTab === "photos" && searchQuery.trim().length > 0;
  return usePhotoSearchView(definition, {
    ...viewOptions,
    withGroups: viewOptions.withGroups ?? true,
    disabled: viewOptions.disabled || !enabled,
  });
};

export const useCurrentTabAssets = (
  options: ViewDefinitionOptions & {
    groupBy?: string;
    pageSize?: number;
  } = {},
): AssetsViewResult => {
  const currentTab = useAssetsStore((state) => state.ui.currentTab);
  const uiGroupBy = useGroupBy();
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);

  const { groupBy, pageSize, ...viewOptions } = options;

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: [currentTab],
      groupBy: (groupBy as any) || uiGroupBy,
      pageSize: pageSize || 50,
      sort: { direction: "desc" },
      search: searchQuery.trim()
        ? {
            query: searchQuery.trim(),
          }
        : undefined,
    }),
    [currentTab, uiGroupBy, searchQuery, groupBy, pageSize],
  );

  const standardView = useAssetsView(definition, {
    ...viewOptions,
    withGroups: true,
  });
  const photoSearchView = useCurrentTabPhotoSearchView(options);

  if (currentTab === "photos" && searchQuery.trim()) {
    return photoSearchView;
  }

  return standardView;
};
