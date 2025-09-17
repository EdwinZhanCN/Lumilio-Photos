import { useState, useMemo, useCallback } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import {
  assetService,
  ListAssetsParams,
  FilterAssetsParams,
  SearchAssetsParams,
  AssetFilter,
} from "@/services/assetsService";
import { AssetsState, AssetsActions } from "@/features/assets";

// Defines the shape of the object returned by our custom hook.
interface FetchProcessValue {
  state: AssetsState;
  actions: AssetsActions;
}

const DEFAULT_FILTERS: ListAssetsParams = {
  limit: 20,
  offset: 0,
  type: "PHOTO",
};

// State to track the current fetch mode
type FetchMode = "list" | "filter" | "search";

interface FetchState {
  mode: FetchMode;
  searchParams?: SearchAssetsParams;
  filterParams?: FilterAssetsParams;
  listParams: ListAssetsParams;
}

/**
 * @hook useFetchProcess
 * @description Core hook for managing the state and actions related to fetching assets.
 * It handles filtering, pagination, and data transformation with support for advanced filtering and search.
 * @returns {FetchProcessValue} An object containing the current state and action handlers.
 */
export function useFetchProcess(): FetchProcessValue {
  const [fetchState, setFetchState] = useState<FetchState>({
    mode: "list",
    listParams: DEFAULT_FILTERS,
  });

  // --- DATA FETCHING (using React Query's useInfiniteQuery) ---
  const {
    data,
    error,
    isFetching,
    isFetchingNextPage,
    fetchNextPage,
    hasNextPage,
    // refetch,
  } = useInfiniteQuery({
    queryKey: ["assets", "infinite", fetchState],
    queryFn: async ({ pageParam = 0 }) => {
      let response;

      switch (fetchState.mode) {
        case "search": {
          if (fetchState.searchParams) {
            const searchParams = {
              ...fetchState.searchParams,
              offset: pageParam,
            };
            response = await assetService.searchAssets(searchParams);
          } else {
            throw new Error("Search parameters not provided");
          }
          break;
        }
        case "filter": {
          if (fetchState.filterParams) {
            const filterParams = {
              ...fetchState.filterParams,
              offset: pageParam,
            };
            response = await assetService.filterAssets(filterParams);
          } else {
            throw new Error("Filter parameters not provided");
          }
          break;
        }
        case "list":
        default: {
          const listParams = {
            ...fetchState.listParams,
            offset: pageParam,
          };
          response = await assetService.listAssets(listParams);
          break;
        }
      }

      return response.data.data;
    },
    initialPageParam: 0,
    refetchOnWindowFocus: false,
    getNextPageParam: (lastPage) => {
      const assets = Array.isArray(lastPage?.assets) ? lastPage.assets : [];
      if (!lastPage || assets.length === 0) {
        return undefined; // No more pages
      }
      const currentOffset =
        typeof lastPage?.offset === "number" ? lastPage.offset : 0;
      return currentOffset + assets.length;
    },
  });

  // --- ACTIONS (wrapped in useCallback for stable function references) ---

  const applyFilter = useCallback((key: keyof ListAssetsParams, value: any) => {
    setFetchState((prev) => ({
      mode: "list",
      listParams: { ...prev.listParams, [key]: value, offset: 0 },
    }));
  }, []);

  const setSearchQuery = useCallback((query: string) => {
    if (!query.trim()) {
      // If empty query, switch back to list mode
      setFetchState((prev) => ({
        mode: "list",
        listParams: { ...prev.listParams, q: undefined, offset: 0 },
      }));
    } else {
      // Use simple filename search via list endpoint for backward compatibility
      setFetchState((prev) => ({
        mode: "list",
        listParams: { ...prev.listParams, q: query, offset: 0 },
      }));
    }
  }, []);

  const performAdvancedSearch = useCallback((params: SearchAssetsParams) => {
    setFetchState((prev) => ({
      ...prev,
      mode: "search",
      // 保留现有 listParams（如 type 等上下文），仅更新搜索参数
      searchParams: { ...params, offset: 0 },
    }));
  }, []);

  const applyAdvancedFilter = useCallback((filter: AssetFilter) => {
    setFetchState((prev) => ({
      ...prev,
      mode: "filter",
      // 进入过滤模式时保留之前的 searchParams 或 listParams，便于清除时恢复
      filterParams: {
        filter,
        offset: 0,
        limit: prev.filterParams?.limit ?? 20,
      },
    }));
  }, []);

  const resetFilters = useCallback(() => {
    setFetchState((prev) => {
      // 如果当前处于过滤模式，恢复到之前的搜索或列表状态
      if (prev.mode === "filter") {
        if (prev.searchParams) {
          // 恢复到搜索模式
          return {
            ...prev,
            mode: "search",
            filterParams: undefined,
            searchParams: { ...prev.searchParams, offset: 0 },
          };
        }
        // 恢复到列表模式，保留用户之前的列表参数（如 type / q）
        return {
          ...prev,
          mode: "list",
          filterParams: undefined,
          listParams: { ...prev.listParams, offset: 0 },
        };
      }
      // 如果本来就在搜索模式，仅重置偏移
      if (prev.mode === "search") {
        return {
          ...prev,
          searchParams: prev.searchParams
            ? { ...prev.searchParams, offset: 0 }
            : prev.searchParams,
        };
      }
      // 列表模式：保留当前列表参数（避免清空用户已选择的 type 等）
      return {
        ...prev,
        mode: "list",
        listParams: { ...prev.listParams, offset: 0 },
      };
    });
  }, []);

  const fetchAssets = useCallback(async (params: ListAssetsParams) => {
    setFetchState({
      mode: "list",
      listParams: params,
    });
  }, []);

  const handleFetchNextPage = useCallback(async () => {
    if (hasNextPage && !isFetchingNextPage) {
      await fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  // --- STATE TRANSFORMATION (using useMemo for performance) ---

  // Flattens the paginated data from React Query into a single array for UI rendering
  const allAssets = useMemo(
    () =>
      data?.pages.flatMap((page) =>
        Array.isArray(page.assets) ? page.assets : [],
      ) ?? [],
    [data],
  );

  // Current filters for compatibility - extract from current fetch state
  const currentFilters = useMemo(() => {
    switch (fetchState.mode) {
      case "search":
        return { ...DEFAULT_FILTERS, q: fetchState.searchParams?.query || "" };
      case "filter":
        return DEFAULT_FILTERS; // Filter mode doesn't use ListAssetsParams format
      case "list":
      default:
        return fetchState.listParams;
    }
  }, [fetchState]);

  // --- RETURN VALUE ---
  // We assemble the state and actions objects that our context will provide.
  const state: AssetsState = {
    assets: allAssets,
    filters: currentFilters,
    isLoading: isFetching && !isFetchingNextPage,
    isLoadingNextPage: isFetchingNextPage,
    error: error ? error.message : null,
    hasMore: hasNextPage,
  };

  // Clear search (used when search UI toggle is turned off)
  const clearSearch = useCallback(() => {
    setFetchState((prev) => {
      // 若当前是过滤模式，只移除搜索上下文即可，不影响过滤结果
      if (prev.mode === "filter") {
        return { ...prev, searchParams: undefined };
      }
      // 搜索或列表模式：清空 q 并回到列表模式
      return {
        ...prev,
        mode: "list",
        searchParams: undefined,
        listParams: { ...prev.listParams, q: undefined, offset: 0 },
      };
    });
  }, []);

  const actions: AssetsActions = {
    fetchAssets,
    fetchNextPage: handleFetchNextPage,
    applyFilter,
    setSearchQuery,
    resetFilters,
    // Extended actions for new API features
    performAdvancedSearch,
    applyAdvancedFilter,
  } as AssetsActions & { clearSearch: () => void };

  // 暴露给外部（类型文件尚未添加时通过断言兼容）
  (actions as any).clearSearch = clearSearch;

  return { state, actions };
}
