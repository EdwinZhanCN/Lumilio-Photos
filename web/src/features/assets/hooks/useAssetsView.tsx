import { useCallback, useMemo } from "react";
import { useAssetsStore } from "../assets.store";
import { useFilterState, useSortBy } from "../selectors";
import {
  AssetMediaType,
  BrowseGroup,
  BrowseItem,
  AssetGroup,
  AssetViewDefinition,
  AssetsViewResult,
  SortByType,
  ViewDefinitionOptions,
} from "@/features/assets/types/assets.type";
import { generateViewKey } from "../utils/viewKey";
import { selectFilterAsAssetFilter, selectFiltersEnabled } from "../slices/filters.slice";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { Asset } from "@/lib/assets/types";
import { useBrowseScope } from "@/features/settings";
import { flattenAssetGroups, getViewerTimeZone } from "../utils/assetGroups";
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

type AssetQueryRequest = components["schemas"]["dto.AssetQueryRequestDTO"];
type AssetFilter = components["schemas"]["dto.AssetFilterDTO"];
type SearchAssetsRequestDTO = components["schemas"]["dto.SearchAssetsRequestDTO"];
type SearchAssetsResponseDTO = components["schemas"]["dto.SearchAssetsResponseDTO"];

export type SearchTopResultsMeta = {
  enabled: boolean;
  degraded: boolean;
  reason?: string;
  source_types: string[];
};

type AssetsViewQueryResult = {
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

type PhotoSearchViewResult = AssetsViewResult & {
  topResults: Asset[];
  resultAssets: Asset[];
  resultGroups: AssetGroup[];
  topResultsBrowseGroups: BrowseGroup[];
  resultBrowseGroups: BrowseGroup[];
  topResultsMeta: SearchTopResultsMeta;
};

export const DEFAULT_TOP_RESULTS_META: SearchTopResultsMeta = {
  enabled: false,
  degraded: false,
  source_types: [],
};

const EMPTY_ASSETS_VIEW_RESULT: AssetsViewResult = {
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

const EMPTY_PHOTO_SEARCH_VIEW_RESULT: PhotoSearchViewResult = {
  ...EMPTY_ASSETS_VIEW_RESULT,
  topResults: [],
  resultAssets: [],
  resultGroups: [],
  topResultsBrowseGroups: [],
  resultBrowseGroups: [],
  topResultsMeta: DEFAULT_TOP_RESULTS_META,
};

// Apple Photos-style two-tier search: Top Results is a small, high-precision
// showcase (aggregate relevance order); the Results tier below carries the
// full relevance set sorted by capture time.
export const TOP_RESULTS_LIMIT = 9;
export const DEFAULT_ASSET_TYPES: AssetMediaType[] = ["photos", "videos"];

const getApiMimeTypes = (mediaTypes: AssetMediaType[]): ("PHOTO" | "VIDEO" | "AUDIO")[] => {
  const mimeTypes: ("PHOTO" | "VIDEO" | "AUDIO")[] = [];
  mediaTypes.forEach((type) => {
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

export const normalizeTopResultsMeta = (
  meta?: SearchAssetsResponseDTO["top_results_meta"],
): SearchTopResultsMeta => ({
  enabled: Boolean(meta?.enabled),
  degraded: Boolean(meta?.degraded),
  reason: meta?.reason,
  source_types: meta?.source_types ?? [],
});

export const normalizeSearchSortBy = (
  sortBy?: AssetViewDefinition["sortBy"],
): SearchAssetsRequestDTO["sort_by"] => {
  switch (sortBy) {
    case "recently_added":
    case "date_captured":
      return sortBy;
    default:
      return "date_captured";
  }
};

const useDefinitionFilter = (definition: AssetViewDefinition): AssetFilter => {
  const { scopedRepositoryId } = useBrowseScope();

  return useMemo(() => {
    const mergedFilter: AssetFilter = {
      ...definition.filter,
    };

    if (mergedFilter.repository_id === undefined && scopedRepositoryId) {
      mergedFilter.repository_id = scopedRepositoryId;
    }

    return mergedFilter;
  }, [definition.filter, scopedRepositoryId]);
};

export const buildApiFilter = (
  definition: AssetViewDefinition,
  effectiveFilter: AssetFilter,
): AssetFilter => {
  const filter: AssetFilter = { ...effectiveFilter };

  if (
    filter.type === undefined &&
    filter.types === undefined &&
    definition.types &&
    definition.types.length > 0
  ) {
    const mimeTypes = getApiMimeTypes(definition.types);
    if (mimeTypes.length === 1) {
      filter.type = mimeTypes[0];
    } else if (mimeTypes.length > 1) {
      filter.types = mimeTypes;
    }
  }

  return filter;
};

export const useAssetsViewQuery = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewQueryResult => {
  const { autoFetch = true, disabled = false } = options;
  const effectiveFilter = useDefinitionFilter(definition);
  const apiFilter = useMemo(
    () => buildApiFilter(definition, effectiveFilter),
    [definition, effectiveFilter],
  );
  const viewKey = useMemo(
    () =>
      generateViewKey({
        ...definition,
        filter: apiFilter,
      }),
    [apiFilter, definition],
  );
  const pageSize = definition.pageSize || 50;
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const sortBy = definition.sortBy ?? "date_captured";

  const createUnifiedRequest = useCallback((): AssetQueryRequest => {
    const request: AssetQueryRequest = {
      filter: apiFilter,
      pagination: {
        limit: pageSize,
        offset: 0,
      },
      sort_by: normalizeSearchSortBy(definition.sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    };

    if (definition.search?.query) {
      request.query = definition.search.query;
    }

    return request;
  }, [apiFilter, definition, pageSize, viewerTimeZone]);

  const query = $api.useInfiniteQuery(
    "post",
    "/api/v1/assets/list",
    {
      body: createUnifiedRequest(),
    },
    {
      enabled: autoFetch && !disabled,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = lastPage;
        const total = responseData?.total_visible;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countLoadedBrowseRowsFromPage({
          items: responseData?.items,
        });
        const hasMore =
          typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  );

  const assetsPages = useMemo(() => {
    const pages = query.data?.pages ?? [];
    const pageParams = query.data?.pageParams ?? [];

    return pages.map((page, index) => {
      const offset = Number(pageParams[index] ?? 0) || 0;
      const responseData = page;
      const browseGroups = browseGroupsFromQueryLikePage({
        items: responseData?.items,
        sortBy,
      });
      const visibleTotal = responseData?.total_visible;
      const loadedCount = countLoadedBrowseRowsFromPage({
        items: responseData?.items,
      });
      const hasMore =
        typeof visibleTotal === "number"
          ? offset + loadedCount < visibleTotal
          : loadedCount >= pageSize;

      return { browseGroups, offset, total: visibleTotal, hasMore };
    });
  }, [query.data?.pageParams, query.dataUpdatedAt, pageSize, sortBy]);

  const browseGroups = useMemo(
    () => mergeAdjacentBrowseGroups(...assetsPages.map((page) => page.browseGroups)),
    [assetsPages],
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
  const browseProjection = useMemo(
    () => ({
      browseGroups,
      browseItems: flattenBrowseGroups(browseGroups),
      browseAssets: flattenBrowseGroupsToAssets(browseGroups),
    }),
    [browseGroups],
  );

  const lastPage = assetsPages.length > 0 ? assetsPages[assetsPages.length - 1] : undefined;
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
        ? (JSON.stringify(query.error) ?? "Unknown error")
        : null;

  return {
    assets,
    groups,
    browseGroups: browseProjection.browseGroups,
    browseItems: browseProjection.browseItems,
    browseAssets: browseProjection.browseAssets,
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
  const assets = queryResult.assets;

  return {
    assets,
    groups: withGroups ? queryResult.groups : undefined,
    browseGroups: queryResult.browseGroups,
    browseItems: queryResult.browseItems,
    browseAssets: queryResult.browseAssets,
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
  const effectiveFilter = useDefinitionFilter(definition);
  const apiFilter = useMemo(
    () => buildApiFilter(definition, effectiveFilter),
    [definition, effectiveFilter],
  );
  const pageSize = definition.pageSize || 50;
  const queryText = definition.search?.query?.trim() ?? "";
  const viewerTimeZone = useMemo(() => getViewerTimeZone(), []);
  const viewKey = useMemo(
    () =>
      `${generateViewKey({
        ...definition,
        filter: apiFilter,
      })}:photo-search`,
    [apiFilter, definition],
  );

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
      sort_by: normalizeSearchSortBy(definition.sortBy),
      stack_mode: "collapsed",
      viewer_timezone: viewerTimeZone,
    }),
    [apiFilter, definition.sortBy, pageSize, queryText, viewerTimeZone],
  );

  const query = $api.useInfiniteQuery(
    "post",
    "/api/v1/assets/search",
    {
      body: createSearchRequest(),
    },
    {
      enabled: autoFetch && !disabled && queryText.length > 0,
      initialPageParam: 0,
      pageParamName: "offset",
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = lastPage;
        const total = responseData?.results_total_visible;
        const offset = Number(lastPageParam ?? 0) || 0;
        const loadedCount = countLoadedBrowseRowsFromPage({
          items: responseData?.result_items,
        });
        const hasMore =
          typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  );

  const searchPages = useMemo(() => {
    const pages = query.data?.pages ?? [];
    const pageParams = query.data?.pageParams ?? [];

    return pages.map((page, index) => {
      const responseData = page;
      const offset = Number(pageParams[index] ?? 0) || 0;
      const total = responseData?.results_total_visible;
      const loadedCount = countLoadedBrowseRowsFromPage({
        items: responseData?.result_items,
      });
      const hasMore =
        typeof total === "number" ? offset + loadedCount < total : loadedCount >= pageSize;

      return {
        topItems: responseData?.top_items,
        topResultsMeta: normalizeTopResultsMeta(responseData?.top_results_meta),
        resultItems: responseData?.result_items,
        total,
        offset,
        hasMore,
      };
    });
  }, [pageSize, query.data?.pageParams, query.dataUpdatedAt]);

  const firstPage = searchPages[0];
  const topResultsBrowseGroups = useMemo(
    () =>
      browseGroupsFromSearchTop({
        topItems: firstPage?.topItems,
      }),
    [firstPage?.topItems],
  );
  const resultBrowseGroups = useMemo(() => {
    const perPage = searchPages.map((page) =>
      browseGroupsFromSearchResultsPage({
        resultItems: page.resultItems,
      }),
    );
    return mergeAdjacentBrowseGroups(...perPage);
  }, [searchPages]);

  const topResults = useMemo(
    () => flattenBrowseGroupsToAssets(topResultsBrowseGroups),
    [topResultsBrowseGroups],
  );
  const resultAssets = useMemo(
    () => flattenBrowseGroupsToAssets(resultBrowseGroups),
    [resultBrowseGroups],
  );
  const resultGroups = useMemo(
    () => (resultAssets.length > 0 ? [{ key: "search:results", assets: resultAssets }] : []),
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

  const lastPage = searchPages.length > 0 ? searchPages[searchPages.length - 1] : undefined;
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
        ? (JSON.stringify(query.error) ?? "Unknown error")
        : null;

  if (queryText.length === 0) {
    return {
      ...EMPTY_PHOTO_SEARCH_VIEW_RESULT,
      viewKey,
    };
  }

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

export const useCurrentAssetsSearchView = (
  options: ViewDefinitionOptions & {
    sortBy?: SortByType;
    pageSize?: number;
  } = {},
): PhotoSearchViewResult => {
  const uiSortBy = useSortBy();
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const filtersState = useFilterState();
  const { sortBy, pageSize, baseFilter, viewKey, ...viewOptions } = options;
  const rawScopedFilter = useMemo(
    () => (selectFiltersEnabled(filtersState) ? selectFilterAsAssetFilter(filtersState) : {}),
    [filtersState],
  );
  const scopedFilter = useMemo(
    () => ({
      ...rawScopedFilter,
      ...baseFilter,
    }),
    [baseFilter, rawScopedFilter],
  );

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: DEFAULT_ASSET_TYPES,
      filter: scopedFilter,
      sortBy: sortBy || uiSortBy,
      pageSize: pageSize || 50,
      key: viewKey,
      search: searchQuery.trim()
        ? {
            query: searchQuery.trim(),
          }
        : undefined,
    }),
    [scopedFilter, uiSortBy, searchQuery, sortBy, pageSize, viewKey],
  );

  const enabled = searchQuery.trim().length > 0;
  return usePhotoSearchView(definition, {
    ...viewOptions,
    withGroups: viewOptions.withGroups ?? true,
    disabled: viewOptions.disabled || !enabled,
  });
};

export const useCurrentAssetsView = (
  options: ViewDefinitionOptions & {
    sortBy?: SortByType;
    pageSize?: number;
  } = {},
): AssetsViewResult => {
  const uiSortBy = useSortBy();
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const filtersState = useFilterState();

  const { sortBy, pageSize, baseFilter, viewKey, ...viewOptions } = options;
  const rawScopedFilter = useMemo(
    () => (selectFiltersEnabled(filtersState) ? selectFilterAsAssetFilter(filtersState) : {}),
    [filtersState],
  );
  const scopedFilter = useMemo(
    () => ({
      ...rawScopedFilter,
      ...baseFilter,
    }),
    [baseFilter, rawScopedFilter],
  );

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: DEFAULT_ASSET_TYPES,
      filter: scopedFilter,
      sortBy: sortBy || uiSortBy,
      pageSize: pageSize || 50,
      key: viewKey,
      search: searchQuery.trim()
        ? {
            query: searchQuery.trim(),
          }
        : undefined,
    }),
    [scopedFilter, uiSortBy, searchQuery, sortBy, pageSize, viewKey],
  );

  const standardView = useAssetsView(definition, {
    ...viewOptions,
    withGroups: true,
  });
  const photoSearchView = useCurrentAssetsSearchView(options);

  if (searchQuery.trim()) {
    return photoSearchView;
  }

  return standardView;
};
