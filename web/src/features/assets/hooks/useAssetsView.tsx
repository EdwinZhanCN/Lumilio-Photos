import { useEffect, useCallback, useMemo, useRef } from "react";
import { useAssetsContext } from "./useAssetsContext";
import {
  AssetViewDefinition,
  AssetsViewResult,
  ViewDefinitionOptions,
  TabType,
} from "../types";
import {
  generateViewKey,
  selectView,
  selectViewAssetIds,
} from "../reducers/views.reducer";
import { selectAssets } from "../reducers/entities.reducer";
import {
  selectFilterAsAssetFilter,
  selectFiltersEnabled,
} from "../reducers/filters.reducer";
import {
  assetService,
  ListAssetsParams,
  SearchAssetsParams,
} from "@/services/assetsService";
import { groupAssets } from "@/lib/utils/assetGrouping";

/**
 * Primary hook for accessing asset data through view definitions.
 * Handles fetching, caching, pagination, and filtering automatically.
 *
 * @param definition View definition specifying what assets to fetch
 * @param options Additional options for behavior control
 * @returns AssetsViewResult with assets, loading states, and actions
 *
 * @example
 * ```tsx
 * // Basic photo view
 * const photoView = useAssetsView({
 *   types: ['photos'],
 *   groupBy: 'date',
 *   pageSize: 50
 * });
 *
 * // Filtered view with search
 * const searchView = useAssetsView({
 *   types: ['photos'],
 *   search: { query: 'sunset', mode: 'semantic' },
 *   filter: { rating: 5 }
 * });
 *
 * // High-rated assets for collection
 * const collectionView = useAssetsView({
 *   filter: { rating: 4 },
 *   inheritGlobalFilter: false,
 *   sort: { field: 'rating', direction: 'desc' }
 * });
 * ```
 */
export const useAssetsView = (
  definition: AssetViewDefinition,
  options: ViewDefinitionOptions = {},
): AssetsViewResult => {
  const { state, dispatch } = useAssetsContext();
  const { autoFetch = true, disabled = false, withGroups = false } = options;

  // Generate stable view key
  const viewKey = useMemo(() => {
    return generateViewKey(definition);
  }, [definition]);

  // Get view state
  const viewState = selectView(state.views, viewKey);
  const assetIds = selectViewAssetIds(state.views, viewKey);

  // Get actual asset objects from entity store
  const assets = useMemo(() => {
    return selectAssets(state.entities, assetIds);
  }, [state.entities, assetIds]);

  // Merge filters if inheritGlobalFilter is enabled
  const effectiveFilter = useMemo(() => {
    const globalFilter = selectFiltersEnabled(state.filters)
      ? selectFilterAsAssetFilter(state.filters)
      : {};

    const baseFilter =
      definition.inheritGlobalFilter !== false ? globalFilter : {};

    return {
      ...baseFilter,
      ...definition.filter,
    };
  }, [definition.filter, definition.inheritGlobalFilter, state.filters]);

  // Whether there is any effective filter to apply
  const hasEffectiveFilter = useMemo(() => {
    return Object.keys(effectiveFilter || {}).length > 0;
  }, [effectiveFilter]);
  // Stable hash to compare filter changes without causing effect dependency churn
  const effectiveFilterHash = useMemo(
    () => JSON.stringify(effectiveFilter || {}),
    [effectiveFilter],
  );

  // Convert tab types to API mime types
  const getApiMimeTypes = useCallback(
    (tabTypes: TabType[]): ("PHOTO" | "VIDEO" | "AUDIO")[] => {
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
    },
    [],
  );

  // Create parameters for listing assets (non-search, no filters)
  const createListParams = useCallback(
    (page?: number): ListAssetsParams => {
      const params: ListAssetsParams = {
        limit: definition.pageSize || 50,
      };

      // Add type filtering
      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          // Single type: use 'type' parameter
          params.type = mimeTypes[0];
        } else if (mimeTypes.length > 1) {
          // Multiple types: use 'types' parameter with comma-separated values
          params.types = mimeTypes.join(",");
        }
      }

      // Add pagination (using offset-based pagination)
      if (page && page > 1) {
        params.offset = (page - 1) * (definition.pageSize || 50);
      }

      // Add sorting
      if (definition.sort) {
        params.sort_order = definition.sort.direction;
      }

      return params;
    },
    [definition, getApiMimeTypes],
  );

  // Create parameters for filtering assets (non-search, with filters)
  const createFilterParams = useCallback(
    (page?: number) => {
      const payload: {
        filter: any;
        limit: number;
        offset?: number;
      } = {
        filter: { ...effectiveFilter },
        limit: definition.pageSize || 50,
      };

      // Pagination
      if (page && page > 1) {
        payload.offset = (page - 1) * (definition.pageSize || 50);
      }

      // Single type only for filter API
      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          payload.filter.type = mimeTypes[0];
        }
      }

      return payload;
    },
    [definition, effectiveFilter, getApiMimeTypes],
  );

  // Create parameters for searching assets
  const createSearchParams = useCallback(
    (page?: number): SearchAssetsParams => {
      const params: SearchAssetsParams = {
        query: definition.search!.query,
        search_type:
          definition.search!.mode === "semantic" ? "semantic" : "filename",
        limit: definition.pageSize || 50,
      };

      // Add pagination
      if (page && page > 1) {
        params.offset = (page - 1) * (definition.pageSize || 50);
      }

      // Add filter object for search endpoint
      const searchFilter = { ...effectiveFilter };

      // Add type filtering to filter object
      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          // Search endpoint only supports single type in filter
          searchFilter.type = mimeTypes[0];
        }
        // Note: SearchAssetsParams.filter.type only supports single type,
        // for multiple types in search, you might need to make multiple requests
        // or the backend needs to support multiple types in search filter
      }

      params.filter = searchFilter;

      return params;
    },
    [definition, effectiveFilter, getApiMimeTypes],
  );

  // Check if this is a search operation
  const isSearchOperation = useMemo(() => {
    return !!definition.search?.query;
  }, [definition.search]);

  // Track if we're currently fetching to prevent duplicate requests
  const fetchingRef = useRef(false);

  // Initial fetch function
  const fetchAssets = useCallback(
    async (replace: boolean = true) => {
      if (disabled || fetchingRef.current) return;

      try {
        fetchingRef.current = true;

        dispatch({
          type: "SET_VIEW_LOADING",
          payload: { viewKey, loading: true },
        });

        let result;

        if (isSearchOperation) {
          const searchParams = createSearchParams();
          result = await assetService.searchAssets(searchParams);
        } else if (hasEffectiveFilter) {
          const filterParams = createFilterParams();
          result = await assetService.filterAssets(filterParams);
        } else {
          const listParams = createListParams();
          result = await assetService.listAssets(listParams);
        }

        // Extract data from API response
        const responseData = result.data?.data;
        const assets = responseData?.assets || [];

        // Store entities in normalized format
        if (assets && assets.length > 0) {
          dispatch({
            type: "BATCH_SET_ENTITIES",
            payload: { assets },
          });
        }

        // Update view with asset IDs
        const newAssetIds = assets
          .map((asset: Asset) => asset.asset_id!)
          .filter(Boolean);

        dispatch({
          type: "SET_VIEW_ASSETS",
          payload: {
            viewKey,
            assetIds: newAssetIds,
            hasMore:
              (responseData?.assets?.length || 0) >=
              (definition.pageSize || 50),
            pageInfo: {
              cursor: undefined,
              page: responseData?.offset
                ? Math.floor(
                    responseData.offset / (definition.pageSize || 50),
                  ) + 1
                : 1,
              total: undefined,
            },
            replace,
          },
        });
      } catch (error) {
        console.error("Failed to fetch assets:", error);
        dispatch({
          type: "SET_VIEW_ERROR",
          payload: {
            viewKey,
            error:
              error instanceof Error ? error.message : "Failed to fetch assets",
          },
        });
      } finally {
        fetchingRef.current = false;
      }
    },
    [
      disabled,
      viewKey,
      isSearchOperation,
      hasEffectiveFilter,
      createSearchParams,
      createFilterParams,
      createListParams,
      dispatch,
      definition.pageSize,
    ],
  );

  // Keep a stable ref to fetchAssets to use inside effects without re-subscribing
  const fetchAssetsRef = useRef(fetchAssets);
  useEffect(() => {
    fetchAssetsRef.current = fetchAssets;
  }, [fetchAssets]);

  // Fetch more (pagination)
  const fetchMore = useCallback(async () => {
    if (
      !viewState ||
      disabled ||
      fetchingRef.current ||
      viewState.isLoadingMore ||
      !viewState.hasMore
    ) {
      return;
    }

    try {
      fetchingRef.current = true;

      dispatch({
        type: "SET_VIEW_LOADING_MORE",
        payload: { viewKey, loading: true },
      });

      const nextPage = viewState.pageInfo.page
        ? viewState.pageInfo.page + 1
        : 2;
      let result;

      if (isSearchOperation) {
        const searchParams = createSearchParams(nextPage);
        result = await assetService.searchAssets(searchParams);
      } else if (hasEffectiveFilter) {
        const filterParams = createFilterParams(nextPage);
        result = await assetService.filterAssets(filterParams);
      } else {
        const listParams = createListParams(nextPage);
        result = await assetService.listAssets(listParams);
      }

      // Extract data from API response
      const responseData = result.data?.data;
      const assets = responseData?.assets || [];

      // Store new entities
      if (assets && assets.length > 0) {
        dispatch({
          type: "BATCH_SET_ENTITIES",
          payload: { assets },
        });
      }

      // Append to view
      const newAssetIds = assets
        .map((asset: Asset) => asset.asset_id!)
        .filter(Boolean);

      dispatch({
        type: "APPEND_VIEW_ASSETS",
        payload: {
          viewKey,
          assetIds: newAssetIds,
          hasMore:
            (responseData?.assets?.length || 0) >= (definition.pageSize || 50),
          pageInfo: {
            cursor: undefined,
            page: nextPage,
            total: undefined,
          },
        },
      });
    } catch (error) {
      console.error("Failed to fetch more assets:", error);
      dispatch({
        type: "SET_VIEW_ERROR",
        payload: {
          viewKey,
          error:
            error instanceof Error
              ? error.message
              : "Failed to fetch more assets",
        },
      });
    } finally {
      fetchingRef.current = false;
    }
  }, [
    viewState,
    disabled,
    viewKey,
    isSearchOperation,
    hasEffectiveFilter,
    createSearchParams,
    createFilterParams,
    createListParams,
    dispatch,
    definition.pageSize,
  ]);

  // Refetch function (clears and refetches)
  const refetch = useCallback(async () => {
    await fetchAssets(true);
  }, [fetchAssets]);

  // Create/ensure view exists
  useEffect(() => {
    if (!disabled) {
      dispatch({
        type: "CREATE_VIEW",
        payload: { viewKey, definition },
      });
    }
  }, [viewKey, definition, disabled, dispatch]);

  // Auto-fetch on view creation or definition changes
  useEffect(() => {
    if (
      autoFetch &&
      !disabled &&
      viewState &&
      !viewState.isLoading &&
      viewState.assetIds.length === 0 &&
      !viewState.error &&
      viewState.lastFetchAt === 0
    ) {
      fetchAssets();
    }
  }, [autoFetch, disabled, viewState, fetchAssets]);

  // Track last applied filter hash to avoid initial refetch and loops
  const lastAppliedFilterHashRef = useRef(effectiveFilterHash);

  // Refetch when effective filters change after initial load
  useEffect(() => {
    if (disabled) return;
    // Only refetch when the filter hash actually changes after mount
    if (lastAppliedFilterHashRef.current === effectiveFilterHash) return;
    lastAppliedFilterHashRef.current = effectiveFilterHash;
    fetchAssetsRef.current(true);
  }, [disabled, effectiveFilterHash]);

  // Generate groups if requested
  const groups = useMemo(() => {
    if (!withGroups || !definition.groupBy || definition.groupBy === "flat") {
      return undefined;
    }
    return groupAssets(assets, definition.groupBy);
  }, [withGroups, definition.groupBy, assets]);

  // Return view result
  return {
    assets,
    groups,
    isLoading: viewState?.isLoading ?? false,
    isLoadingMore: viewState?.isLoadingMore ?? false,
    error: viewState?.error ?? null,
    fetchMore,
    refetch,
    hasMore: viewState?.hasMore ?? true,
    viewKey,
    pageInfo: viewState?.pageInfo ?? {
      cursor: undefined,
      page: 1,
      total: undefined,
    },
  };
};

/**
 * Hook for getting assets of the current active tab.
 * Convenience wrapper around useAssetsView for the main page view.
 *
 * @param options View options
 * @returns Assets view result for current tab
 */
export const useCurrentTabAssets = (
  options: ViewDefinitionOptions & {
    groupBy?: string;
    pageSize?: number;
  } = {},
): AssetsViewResult => {
  const { state } = useAssetsContext();
  const { groupBy, pageSize, ...viewOptions } = options;

  const definition: AssetViewDefinition = useMemo(
    () => ({
      types: [state.ui.currentTab],
      groupBy: (groupBy as any) || state.ui.groupBy,
      pageSize: pageSize || 50,
      sort: { field: "taken_time", direction: "desc" },
      search: state.ui.searchQuery.trim()
        ? {
            query: state.ui.searchQuery.trim(),
            mode:
              state.ui.currentTab === "photos"
                ? state.ui.searchMode
                : "filename",
          }
        : undefined,
    }),
    [
      state.ui.currentTab,
      state.ui.groupBy,
      state.ui.searchQuery,
      state.ui.searchMode,
      groupBy,
      pageSize,
    ],
  );

  return useAssetsView(definition, {
    ...viewOptions,
    withGroups: true, // Main page always needs groups
  });
};
