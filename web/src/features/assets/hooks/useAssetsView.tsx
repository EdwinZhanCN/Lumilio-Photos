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
import { albumService } from "@/services/albumService";
import { groupAssets } from "@/lib/utils/assetGrouping";
import { Asset } from "@/services";

/**
 * Primary hook for accessing asset data through view definitions.
 * Handles fetching, caching, pagination, and filtering automatically.
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

  const hasEffectiveFilter = useMemo(() => {
    return Object.keys(effectiveFilter || {}).length > 0;
  }, [effectiveFilter]);

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

  // Create parameters for listing assets
  const createListParams = useCallback(
    (page?: number): ListAssetsParams => {
      const params: ListAssetsParams = {
        limit: definition.pageSize || 50,
      };

      if (definition.types && definition.types.length > 0) {
        const mimeTypes = getApiMimeTypes(definition.types);
        if (mimeTypes.length === 1) {
          params.type = mimeTypes[0];
        } else if (mimeTypes.length > 1) {
          params.types = mimeTypes.join(",");
        }
      }

      if (page && page > 1) {
        params.offset = (page - 1) * (definition.pageSize || 50);
      } else {
        params.offset = 0;
      }

      if (definition.sort) {
        params.sort_order = definition.sort.direction;
      }

      return params;
    },
    [definition, getApiMimeTypes],
  );

  // Create parameters for filtering assets
  const createFilterParams = useCallback(
    (page?: number) => {
      const payload: any = {
        filter: { ...effectiveFilter },
        limit: definition.pageSize || 50,
      };

      if (page && page > 1) {
        payload.offset = (page - 1) * (definition.pageSize || 50);
      } else {
        payload.offset = 0;
      }

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

      if (page && page > 1) {
        params.offset = (page - 1) * (definition.pageSize || 50);
      } else {
        params.offset = 0;
      }

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
    [definition, effectiveFilter, getApiMimeTypes],
  );

  const isSearchOperation = useMemo(() => {
    return !!definition.search?.query;
  }, [definition.search]);

  const fetchingRef = useRef(false);

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
        const albumId = effectiveFilter.album_id;

        if (albumId) {
          result = await albumService.filterAlbumAssets(albumId, createFilterParams());
        } else if (isSearchOperation) {
          result = await assetService.searchAssets(createSearchParams());
        } else if (hasEffectiveFilter) {
          result = await assetService.filterAssets(createFilterParams());
        } else {
          result = await assetService.listAssets(createListParams());
        }

        const responseData = result.data?.data;
        const assets = responseData?.assets || [];

        if (assets && assets.length > 0) {
          dispatch({
            type: "BATCH_SET_ENTITIES",
            payload: { assets },
          });
        }

        const newAssetIds = assets
          .map((asset: Asset) => asset.asset_id)
          .filter((id: any): id is string => Boolean(id));

        dispatch({
          type: "SET_VIEW_ASSETS",
          payload: {
            viewKey,
            assetIds: newAssetIds,
            hasMore: (responseData?.assets?.length || 0) >= (definition.pageSize || 50),
            pageInfo: {
              cursor: undefined,
              page: responseData?.offset ? Math.floor(responseData.offset / (definition.pageSize || 50)) + 1 : 1,
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
            error: error instanceof Error ? error.message : "Failed to fetch assets",
          },
        });
      } finally {
        fetchingRef.current = false;
      }
    },
    [disabled, viewKey, isSearchOperation, hasEffectiveFilter, effectiveFilter.album_id, createSearchParams, createFilterParams, createListParams, dispatch, definition.pageSize],
  );

  const fetchMore = useCallback(async () => {
    if (!viewState || disabled || fetchingRef.current || viewState.isLoadingMore || !viewState.hasMore) {
      return;
    }

    try {
      fetchingRef.current = true;
      dispatch({ type: "SET_VIEW_LOADING_MORE", payload: { viewKey, loading: true } });

      const nextPage = viewState.pageInfo.page ? viewState.pageInfo.page + 1 : 2;
      let result;
      const albumId = effectiveFilter.album_id;

      if (albumId) {
        result = await albumService.filterAlbumAssets(albumId, createFilterParams(nextPage));
      } else if (isSearchOperation) {
        result = await assetService.searchAssets(createSearchParams(nextPage));
      } else if (hasEffectiveFilter) {
        result = await assetService.filterAssets(createFilterParams(nextPage));
      } else {
        result = await assetService.listAssets(createListParams(nextPage));
      }

      const responseData = result.data?.data;
      const assets = responseData?.assets || [];

      if (assets && assets.length > 0) {
        dispatch({ type: "BATCH_SET_ENTITIES", payload: { assets } });
      }

      const newAssetIds = assets.map((asset: Asset) => asset.asset_id).filter((id: any): id is string => Boolean(id));

      dispatch({
        type: "APPEND_VIEW_ASSETS",
        payload: {
          viewKey,
          assetIds: newAssetIds,
          hasMore: (responseData?.assets?.length || 0) >= (definition.pageSize || 50),
          pageInfo: { cursor: undefined, page: nextPage, total: undefined },
        },
      });
    } catch (error) {
      console.error("Failed to fetch more assets:", error);
      dispatch({
        type: "SET_VIEW_ERROR",
        payload: {
          viewKey,
          error: error instanceof Error ? error.message : "Failed to fetch more assets",
        },
      });
    } finally {
      fetchingRef.current = false;
    }
  }, [viewState, disabled, viewKey, isSearchOperation, hasEffectiveFilter, effectiveFilter.album_id, createSearchParams, createFilterParams, createListParams, dispatch, definition.pageSize]);

  const refetch = useCallback(async () => {
    await fetchAssets(true);
  }, [fetchAssets]);

  // 1. Ensure view exists
  useEffect(() => {
    if (!disabled) {
      dispatch({ type: "CREATE_VIEW", payload: { viewKey, definition } });
    }
  }, [viewKey, disabled, dispatch]);

  // 2. Auto-fetch on mount or view creation
  useEffect(() => {
    if (autoFetch && !disabled && viewState && !viewState.isLoading && viewState.assetIds.length === 0 && !viewState.error && viewState.lastFetchAt === 0) {
      fetchAssets();
    }
  }, [autoFetch, disabled, viewState?.isLoading, viewState?.assetIds.length, viewState?.error, viewState?.lastFetchAt, fetchAssets]);

  // 3. Refetch on filter changes
  const lastFilterHash = useRef(effectiveFilterHash);
  useEffect(() => {
    if (disabled) return;
    if (lastFilterHash.current !== effectiveFilterHash) {
      lastFilterHash.current = effectiveFilterHash;
      fetchAssets(true);
    }
  }, [disabled, effectiveFilterHash, fetchAssets]);

  const groups = useMemo(() => {
    if (!withGroups || !definition.groupBy || definition.groupBy === "flat") {
      return undefined;
    }
    return groupAssets(assets, definition.groupBy);
  }, [withGroups, definition.groupBy, assets]);

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
    pageInfo: viewState?.pageInfo ?? { cursor: undefined, page: 1, total: undefined },
  };
};

/**
 * Hook for getting assets of the current active tab.
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
            mode: state.ui.currentTab === "photos" ? state.ui.searchMode : "filename",
          }
        : undefined,
    }),
    [state.ui.currentTab, state.ui.groupBy, state.ui.searchQuery, state.ui.searchMode, groupBy, pageSize],
  );

  return useAssetsView(definition, {
    ...viewOptions,
    withGroups: true,
  });
};
