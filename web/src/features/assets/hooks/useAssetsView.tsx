import { useEffect, useCallback, useMemo, useRef } from "react";
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
import client from "@/lib/http-commons/client";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";

type ListAssetsParams = NonNullable<paths["/assets"]["get"]["parameters"]["query"]>;
type SearchAssetsParams = components["schemas"]["dto.SearchAssetsRequestDTO"];
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
  const { autoFetch = true, disabled = false, withGroups = false } = options;

  // Generate stable view key
  const viewKey = useMemo(() => {
    return generateViewKey(definition);
  }, [definition]);

  // Actions
  const {
    createView,
    setViewLoading,
    setViewAssets,
    appendViewAssets,
    setViewError,
    setViewLoadingMore,
    batchSetEntities,
  } = useAssetsStore(
    useShallow((state) => ({
      createView: state.createView,
      setViewLoading: state.setViewLoading,
      setViewAssets: state.setViewAssets,
      appendViewAssets: state.appendViewAssets,
      setViewError: state.setViewError,
      setViewLoadingMore: state.setViewLoadingMore,
      batchSetEntities: state.batchSetEntities,
    })),
  );

  // Get view state
  const viewState = useAssetsStore((state) => selectView(state, viewKey));
  const assetIds = useAssetsStore((state) => selectViewAssetIds(state, viewKey));

  // Get actual asset objects from entity store - subscribe only to the assets in this view
  const assets = useAssetsStore(
    useShallow((state) => selectAssets(state.entities, assetIds))
  );

  // Get filters state for calculating effective filter
  // We need to subscribe to filter changes to re-calculate effective filter
  const filtersState = useAssetsStore(
    useShallow((state) => ({
      // We subscribe to the whole filter object to ensure we catch any updates
      // This is acceptable as filter changes usually mean we need to refetch anyway
      ...state.filters
    }))
  );

  // Merge filters if inheritGlobalFilter is enabled
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

        setViewLoading(viewKey, true);

        let result;
        const albumId = effectiveFilter.album_id;

        if (albumId) {
          result = await client.POST("/albums/{id}/filter", {
            params: { path: { id: albumId } },
            body: createFilterParams(),
          });
        } else if (isSearchOperation) {
          result = await client.POST("/assets/search", {
            body: createSearchParams(),
          });
        } else if (hasEffectiveFilter) {
          result = await client.POST("/assets/filter", {
            body: createFilterParams(),
          });
        } else {
          result = await client.GET("/assets", {
            params: { query: createListParams() },
          });
        }

        const responseData = result.data?.data;
        const assets = responseData?.assets || [];

        if (assets && assets.length > 0) {
          batchSetEntities(assets);
        }

        const newAssetIds = assets
          .map((asset: Asset) => asset.asset_id)
          .filter((id: any): id is string => Boolean(id));

        setViewAssets(
          viewKey,
          newAssetIds,
          (assets.length || 0) >= (definition.pageSize || 50),
          {
            cursor: undefined,
            page: responseData?.offset ? Math.floor(responseData.offset / (definition.pageSize || 50)) + 1 : 1,
            total: undefined,
          },
          replace
        );
      } catch (error) {
        console.error("Failed to fetch assets:", error);
        setViewError(
          viewKey,
          error instanceof Error ? error.message : "Failed to fetch assets"
        );
      } finally {
        fetchingRef.current = false;
      }
    },
    [disabled, viewKey, isSearchOperation, hasEffectiveFilter, effectiveFilter.album_id, createSearchParams, createFilterParams, createListParams, definition.pageSize, setViewLoading, setViewAssets, batchSetEntities, setViewError],
  );

  const fetchMore = useCallback(async () => {
    if (!viewState || disabled || fetchingRef.current || viewState.isLoadingMore || !viewState.hasMore) {
      return;
    }

    try {
      fetchingRef.current = true;
      setViewLoadingMore(viewKey, true);

      const nextPage = viewState.pageInfo.page ? viewState.pageInfo.page + 1 : 2;
      let result;
      const albumId = effectiveFilter.album_id;

      if (albumId) {
        result = await client.POST("/albums/{id}/filter", {
          params: { path: { id: albumId } },
          body: createFilterParams(nextPage),
        });
      } else if (isSearchOperation) {
        result = await client.POST("/assets/search", {
          body: createSearchParams(nextPage),
        });
      } else if (hasEffectiveFilter) {
        result = await client.POST("/assets/filter", {
          body: createFilterParams(nextPage),
        });
      } else {
        result = await client.GET("/assets", {
          params: { query: createListParams(nextPage) },
        });
      }

      const responseData = result.data?.data;
      const assets = responseData?.assets || [];

      if (assets && assets.length > 0) {
        batchSetEntities(assets);
      }

      const newAssetIds = assets.map((asset: Asset) => asset.asset_id).filter((id: any): id is string => Boolean(id));

      appendViewAssets(
        viewKey,
        newAssetIds,
        (assets.length || 0) >= (definition.pageSize || 50),
        { cursor: undefined, page: nextPage, total: undefined }
      );
    } catch (error) {
      console.error("Failed to fetch more assets:", error);
      setViewError(
        viewKey,
        error instanceof Error ? error.message : "Failed to fetch more assets"
      );
    } finally {
      fetchingRef.current = false;
      setViewLoadingMore(viewKey, false);
    }
  }, [viewState, disabled, viewKey, isSearchOperation, hasEffectiveFilter, effectiveFilter.album_id, createSearchParams, createFilterParams, createListParams, definition.pageSize, setViewLoadingMore, batchSetEntities, appendViewAssets, setViewError]);

  const refetch = useCallback(async () => {
    await fetchAssets(true);
  }, [fetchAssets]);

  // 1. Ensure view exists
  useEffect(() => {
    if (!disabled) {
      createView(viewKey, definition);
    }
  }, [viewKey, disabled, createView, definition]);

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

