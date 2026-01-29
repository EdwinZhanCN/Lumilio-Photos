import { useEffect, useCallback, useMemo } from "react";
import { useAssetsStore } from "../assets.store";
import { useShallow } from "zustand/react/shallow";
import {
  AssetViewDefinition,
  AssetsViewResult,
  ViewDefinitionOptions,
  TabType,
} from "@/features/assets";
import {
  generateViewKey,
  selectView,
  selectViewAssetIds,
} from "../slices/views.slice";
import { selectAssets } from "../slices/entities.slice";
import {
  selectFilterAsAssetFilter,
  selectFiltersEnabled,
} from "../slices/filters.slice";
import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import { groupAssets } from "@/lib/utils/assetGrouping";
import { Asset } from "@/lib/assets/types";

type ListAssetsParams = NonNullable<paths["/api/v1/assets"]["get"]["parameters"]["query"]>;
type SearchAssetsParams = components["schemas"]["dto.SearchAssetsRequestDTO"];

type StoreSyncResult = {
  assets: Asset[];
  viewState:
    | {
        isLoading: boolean;
        isLoadingMore: boolean;
        hasMore: boolean;
        error: string | null;
        pageInfo: { cursor?: string; page?: number; total?: number };
      }
    | undefined;
};

type AssetsViewQueryResult = {
  assets: Asset[];
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

/**
 * Converts tab types to their corresponding API MIME type strings.
 * 
 * @param tabTypes - Array of tab types to convert
 * @returns Array of MIME type strings compatible with the API
 * 
 * @example
 * ```ts
 * const mimeTypes = getApiMimeTypes(['photos', 'videos']);
 * // Returns: ['PHOTO', 'VIDEO']
 * ```
 */
const getApiMimeTypes = (tabTypes: TabType[]): ("PHOTO" | "VIDEO" | "AUDIO")[] => {
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

/**
 * React Query hook for fetching assets with view-aware filtering and pagination.
 * 
 * This hook handles different API endpoints based on the view definition:
 * - Regular asset listing for simple queries
 * - Album-specific filtering when album_id is present
 * - Search endpoint when search query is provided
 * - Filter endpoint for complex filtering
 * 
 * @param definition - The asset view definition containing filters, sorting, and pagination settings
 * @param options - Additional options for the view behavior
 * @param options.autoFetch - Whether to automatically fetch data (default: true)
 * @param options.disabled - Whether the view is disabled (default: false)
 * 
 * @returns AssetsViewQueryResult containing assets, loading states, and pagination controls
 * 
 * @example
 * ```ts
 * const result = useAssetsViewQuery({
 *   types: ['photos'],
 *   filter: { album_id: '123' },
 *   sort: { field: 'taken_time', direction: 'desc' },
 *   pageSize: 50
 * });
 * 
 * const { assets, isLoading, fetchMore, hasMore } = result;
 * ```
 */
export const useAssetsViewQuery = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewQueryResult => {
  const { autoFetch = true, disabled = false } = options;
  const viewKey = useMemo(() => generateViewKey(definition), [definition]);

  const filtersState = useAssetsStore((state) => state.filters);

  const effectiveFilter = useMemo(() => {
    const globalFilter = selectFiltersEnabled({ filters: filtersState } as any)
      ? selectFilterAsAssetFilter({ filters: filtersState } as any)
      : {};

    const baseFilter =
      definition.inheritGlobalFilter !== false ? globalFilter : {};

    return {
      ...baseFilter,
      ...definition.filter,
    };
  }, [definition.filter, definition.inheritGlobalFilter, filtersState]);

  const hasEffectiveFilter = useMemo(() => {
    return Object.keys(effectiveFilter || {}).length > 0;
  }, [effectiveFilter]);

  const isSearchOperation = useMemo(() => {
    return !!definition.search?.query;
  }, [definition.search]);

  const pageSize = definition.pageSize || 50;

  /**
   * Creates API parameters for listing assets with filtering, sorting, and pagination.
   * 
   * @param offset - The number of assets to skip for pagination (default: 0)
   * @returns ListAssetsParams object containing query parameters for the API
   * 
   * @example
   * ```ts
   * const params = createListParams(50);
   * // Returns: { limit: 50, offset: 50, type: "PHOTO", sort_order: "desc" }
   * ```
   */
  const createListParams = useCallback(
    (offset: number = 0): ListAssetsParams => {
      const params: ListAssetsParams = {
        limit: pageSize,
        offset,
      };

      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          params.type = mimeTypes[0];
        } else if (mimeTypes.length > 1) {
          params.types = mimeTypes.join(",");
        }
      }

      if (definition.sort) {
        params.sort_order = definition.sort.direction;
      }

      return params;
    },
    [definition, pageSize],
  );

  /**
   * Creates request body parameters for filtered asset queries.
   * Combines effective filters with asset type restrictions and pagination.
   * 
   * @returns Request payload object for filter-based API endpoints
   * 
   * @example
   * ```ts
   * const payload = createFilterParams();
   * // Returns: { filter: { type: "PHOTO", album_id: "123" }, limit: 50 }
   * ```
   */
  const createFilterParams = useCallback(
    () => {
      const payload: any = {
        filter: { ...effectiveFilter },
        limit: pageSize,
      };

      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          payload.filter.type = mimeTypes[0];
        }
      }

      return payload;
    },
    [definition, effectiveFilter, pageSize],
  );

  /**
   * Creates request body parameters for asset search queries.
   * Supports both semantic and filename-based search with filtering.
   * 
   * @returns SearchAssetsParams object for the search API endpoint
   * 
   * @example
   * ```ts
   * const params = createSearchParams();
   * // Returns: { query: "beach", search_type: "semantic", filter: { type: "PHOTO" }, limit: 50 }
   * ```
   */
  const createSearchParams = useCallback(
    (): SearchAssetsParams => {
      const params: SearchAssetsParams = {
        query: definition.search!.query,
        search_type:
          definition.search!.mode === "semantic" ? "semantic" : "filename",
        limit: pageSize,
      };

      const searchFilter = { ...effectiveFilter };

      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          searchFilter.type = mimeTypes[0];
        }
      }

      params.filter = searchFilter;
      return params;
    },
    [definition, effectiveFilter, pageSize],
  );

  const requestConfig = useMemo(() => {
    const albumId = effectiveFilter.album_id;

    if (albumId) {
      return {
        method: "post",
        path: "/api/v1/albums/{id}/filter",
        init: {
          params: { path: { id: albumId } },
          body: createFilterParams(),
        },
        pageParamName: "offset",
      } as const;
    }

    if (isSearchOperation) {
      return {
        method: "post",
        path: "/api/v1/assets/search",
        init: {
          body: createSearchParams(),
        },
        pageParamName: "offset",
      } as const;
    }

    if (hasEffectiveFilter) {
      return {
        method: "post",
        path: "/api/v1/assets/filter",
        init: {
          body: createFilterParams(),
        },
        pageParamName: "offset",
      } as const;
    }

    return {
      method: "get",
      path: "/api/v1/assets",
      init: {
        params: { query: createListParams(0) },
      },
      pageParamName: "offset",
    } as const;
  }, [
    effectiveFilter.album_id,
    isSearchOperation,
    hasEffectiveFilter,
    createFilterParams,
    createSearchParams,
    createListParams,
  ]);

  const query = $api.useInfiniteQuery(
    requestConfig.method as any,
    requestConfig.path as any,
    requestConfig.init as any,
    {
      enabled: autoFetch && !disabled,
      initialPageParam: 0,
      pageParamName: requestConfig.pageParamName,
      getNextPageParam: (lastPage, _allPages, lastPageParam) => {
        const responseData = (lastPage as any)?.data;
        const assets = responseData?.assets || [];
        const total = responseData?.total;
        const offset = Number(lastPageParam ?? 0) || 0;
        const hasMore =
          typeof total === "number"
            ? offset + assets.length < total
            : assets.length >= pageSize;

        return hasMore ? offset + pageSize : undefined;
      },
    },
  );

  const pages = query.data?.pages ?? [];
  const pageParams = query.data?.pageParams ?? [];
  const assetsPages = pages.map((page, index) => {
    const offset = Number(pageParams[index] ?? 0) || 0;
    const responseData = (page as any)?.data;
    const assets = responseData?.assets || [];
    const total = responseData?.total;
    const hasMore =
      typeof total === "number"
        ? offset + assets.length < total
        : assets.length >= pageSize;

    return { assets, offset, total, hasMore };
  });

  const assets = useMemo(
    () => assetsPages.flatMap((page) => page.assets),
    [assetsPages],
  );

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

/**
 * Synchronizes React Query results with the Zustand assets store for legacy compatibility.
 * 
 * This hook maintains backward compatibility by keeping the zustand view state
 * synchronized with the React Query lifecycle. It handles:
 * - View creation and initialization
 * - Loading state synchronization
 * - Error state propagation
 * - Asset entity updates in the store
 * - Pagination state management
 * 
 * @param definition - The asset view definition
 * @param options - View options including disabled flag
 * @param queryResult - The query result from useAssetsViewQuery
 * 
 * @returns StoreSyncResult containing synchronized assets and view state
 * 
 * @example
 * ```ts
 * const queryResult = useAssetsViewQuery(definition, options);
 * const { assets, viewState } = useAssetsViewStoreSync(definition, options, queryResult);
 * ```
 */
const useAssetsViewStoreSync = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions,
  queryResult: AssetsViewQueryResult,
): StoreSyncResult => {
  const { disabled = false } = options;
  const {
    assets: queryAssets,
    isLoading,
    isLoadingMore,
    hasMore,
    isFetched,
    error,
    viewKey,
    pageInfo,
  } = queryResult;

  const {
    createView,
    setViewLoading,
    setViewAssets,
    setViewError,
    setViewLoadingMore,
    batchSetEntities,
  } = useAssetsStore(
    useShallow((state) => ({
      createView: state.createView,
      setViewLoading: state.setViewLoading,
      setViewAssets: state.setViewAssets,
      setViewError: state.setViewError,
      setViewLoadingMore: state.setViewLoadingMore,
      batchSetEntities: state.batchSetEntities,
    })),
  );

  const viewState = useAssetsStore((state) => selectView(state, viewKey));
  const assetIds = useAssetsStore((state) => selectViewAssetIds(state, viewKey));
  const assets = useAssetsStore(
    useShallow((state) => selectAssets(state.entities, assetIds)),
  );

  useEffect(() => {
    if (!disabled) {
      createView(viewKey, definition);
    }
  }, [viewKey, disabled, createView, definition]);

  useEffect(() => {
    if (disabled) return;
    setViewLoading(viewKey, isLoading);
  }, [disabled, viewKey, isLoading, setViewLoading]);

  useEffect(() => {
    if (disabled) return;
    setViewLoadingMore(viewKey, isLoadingMore);
  }, [disabled, viewKey, isLoadingMore, setViewLoadingMore]);

  useEffect(() => {
    if (disabled) return;
    setViewError(viewKey, error);
  }, [disabled, viewKey, error, setViewError]);

  useEffect(() => {
    if (disabled) return;
    if (!isFetched) return;

    batchSetEntities(queryAssets);

    const newAssetIds = queryAssets
      .map((asset) => asset.asset_id)
      .filter((id): id is string => Boolean(id));

    setViewAssets(viewKey, newAssetIds, hasMore, pageInfo, true);
  }, [
    disabled,
    queryAssets,
    viewKey,
    hasMore,
    pageInfo,
    isFetched,
    batchSetEntities,
    setViewAssets,
  ]);

  return {
    assets,
    viewState: viewState
      ? {
          isLoading: viewState.isLoading,
          isLoadingMore: viewState.isLoadingMore,
          hasMore: viewState.hasMore,
          error: viewState.error,
          pageInfo: viewState.pageInfo,
        }
      : undefined,
  };
};

/**
 * Groups assets based on the specified grouping criteria.
 * 
 * This hook applies asset grouping logic when enabled, organizing assets
 * into logical groups (e.g., by date, location, or custom criteria).
 * 
 * @param assets - The array of assets to group
 * @param definition - The view definition containing grouping configuration
 * @param withGroups - Whether grouping should be applied
 * 
 * @returns Grouped assets structure or undefined if grouping is disabled
 * 
 * @example
 * ```ts
 * const groups = useAssetsViewGrouping(assets, { groupBy: 'date' }, true);
 * // Returns: { '2024-01-01': [asset1, asset2], '2024-01-02': [asset3] }
 * ```
 */
const useAssetsViewGrouping = (
  assets: Asset[],
  definition: AssetViewDefinition,
  withGroups: boolean,
) => {
  return useMemo(() => {
    if (!withGroups || !definition.groupBy || definition.groupBy === "flat") {
      return undefined;
    }
    return groupAssets(assets, definition.groupBy);
  }, [withGroups, definition.groupBy, assets]);
};

/**
 * Primary hook for accessing and managing asset data through view definitions.
 * 
 * This is the main entry point for working with assets in the application.
 * It combines data fetching, caching, pagination, filtering, and optional grouping
 * into a single, easy-to-use interface.
 * 
 * Features:
 * - Automatic data fetching with React Query
 * - Pagination with infinite scroll support
 * - View-aware filtering and sorting
 * - Optional asset grouping
 * - Error handling and loading states
 * - Store synchronization for legacy compatibility
 * 
 * @param definition - The asset view definition specifying what data to fetch
 * @param options - Additional configuration options
 * @param options.withGroups - Whether to return grouped assets (default: false)
 * @param options.autoFetch - Whether to automatically fetch data (default: true)
 * @param options.disabled - Whether the view is disabled (default: false)
 * 
 * @returns AssetsViewResult containing assets, groups, loading states, and controls
 * 
 * @example
 * ```ts
 * // Basic usage for photos
 * const { assets, isLoading, fetchMore } = useAssetsView({
 *   types: ['photos'],
 *   pageSize: 50
 * });
 * 
 * // Advanced usage with filtering and grouping
 * const { assets, groups, hasMore } = useAssetsView({
 *   types: ['photos', 'videos'],
 *   filter: { album_id: '123' },
 *   sort: { field: 'taken_time', direction: 'desc' },
 *   groupBy: 'date'
 * }, { withGroups: true });
 * ```
 */
export const useAssetsView = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewResult => {
  const { withGroups = false } = options;
  const queryResult = useAssetsViewQuery(definition, options);
  const { assets, viewState } = useAssetsViewStoreSync(
    definition,
    options,
    queryResult,
  );
  const groups = useAssetsViewGrouping(assets, definition, withGroups);

  return {
    assets,
    groups,
    isLoading: viewState?.isLoading ?? false,
    isLoadingMore: viewState?.isLoadingMore ?? false,
    error: viewState?.error ?? null,
    fetchMore: queryResult.fetchMore,
    refetch: queryResult.refetch,
    hasMore: viewState?.hasMore ?? true,
    viewKey: queryResult.viewKey,
    pageInfo:
      viewState?.pageInfo ?? { cursor: undefined, page: 1, total: undefined },
  };
};

/**
 * Hook for accessing assets from the currently active tab in the UI.
 * 
 * This hook automatically reads the current tab state from the store
 * and creates an appropriate view definition. It's commonly used in
 * components that need to display assets based on the user's current
 * tab selection (photos, videos, audios).
 * 
 * Features:
 * - Automatic tab type detection
 * - Search query integration
 * - UI grouping preferences
 * - Default sorting by taken time
 * - Automatic grouping enabled
 * 
 * @param options - Configuration options for the current tab view
 * @param options.groupBy - Override the UI grouping preference
 * @param options.pageSize - Custom page size (default: 50)
 * @param options.withGroups - Whether to return grouped assets (default: true)
 * @param options.autoFetch - Whether to automatically fetch data (default: true)
 * @param options.disabled - Whether the view is disabled (default: false)
 * 
 * @returns AssetsViewResult for the current tab's assets
 * 
 * @example
 * ```ts
 * // Simple usage - gets current tab assets
 * const { assets, isLoading } = useCurrentTabAssets();
 * 
 * // Custom page size and grouping
 * const { assets, groups } = useCurrentTabAssets({
 *   pageSize: 100,
 *   groupBy: 'location'
 * });
 * ```
 */
export const useCurrentTabAssets = (
  options: ViewDefinitionOptions & {
    groupBy?: string;
    pageSize?: number;
  } = {},
): AssetsViewResult => {
  const currentTab = useAssetsStore((state) => state.ui.currentTab);
  const uiGroupBy = useAssetsStore((state) => state.ui.groupBy);
  const searchQuery = useAssetsStore((state) => state.ui.searchQuery);
  const searchMode = useAssetsStore((state) => state.ui.searchMode);

  const { groupBy, pageSize, ...viewOptions } = options;

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: [currentTab],
      groupBy: (groupBy as any) || uiGroupBy,
      pageSize: pageSize || 50,
      sort: { field: "taken_time", direction: "desc" },
      search: searchQuery.trim()
        ? {
          query: searchQuery.trim(),
          mode: currentTab === "photos" ? searchMode : "filename",
        }
        : undefined,
    }),
    [currentTab, uiGroupBy, searchQuery, searchMode, groupBy, pageSize],
  );

  return useAssetsView(definition, {
    ...viewOptions,
    withGroups: true,
  });
};
