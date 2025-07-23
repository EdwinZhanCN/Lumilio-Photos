import { useState, useMemo, useCallback } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { getAssetService, ListAssetsParams } from "@/services/getAssetsService";
import { AssetsState, AssetsActions } from "@/contexts/FetchContext";

// Defines the shape of the object returned by our custom hook.
interface FetchProcessValue {
  state: AssetsState;
  actions: AssetsActions;
}

const DEFAULT_FILTERS: ListAssetsParams = {
  limit: 20,
  offset: 0,
  type: "PHOTO", // Default filter example
};

/**
 * @hook useFetchProcess
 * @description Core hook for managing the state and actions related to fetching assets.
 * It handles filtering, pagination, and data transformation.
 * @returns {FetchProcessValue} An object containing the current state and action handlers.
 */
export function useFetchProcess(): FetchProcessValue {
  const [filters, setFilters] = useState<ListAssetsParams>(DEFAULT_FILTERS);

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
    queryKey: ["assets", "infinite", filters],
    queryFn: async ({ pageParam = 0 }) => {
      const params: ListAssetsParams = {
        ...filters,
        offset: pageParam,
      };
      // Fetches the data using the provided service
      const response = await getAssetService.listAssets(params);
      // The service returns the full axios response, so we extract the data
      return response.data.data;
    },
    initialPageParam: 0,
    // Determines the offset for the next page fetch
    getNextPageParam: (lastPage) => {
      if (!lastPage || lastPage.assets.length === 0) {
        return undefined; // No more pages
      }
      // The next page's offset is the current offset plus the number of items fetched
      return lastPage.offset + lastPage.assets.length;
    },
  });

  // --- ACTIONS (wrapped in useCallback for stable function references) ---

  const applyFilter = useCallback((key: keyof ListAssetsParams, value: any) => {
    setFilters((prev) => ({ ...prev, [key]: value, offset: 0 }));
  }, []);

  const setSearchQuery = useCallback((query: string) => {
    setFilters((prev) => ({ ...prev, q: query, offset: 0 }));
  }, []);

  const resetFilters = useCallback(() => {
    setFilters(DEFAULT_FILTERS);
  }, []);

  const fetchAssets = useCallback(async (params: ListAssetsParams) => {
    setFilters(params);
    // refetch() can be used here if you want to force a fetch with the new params
    // but changing the state `filters` which is part of the queryKey already triggers it.
  }, []);

  const handleFetchNextPage = useCallback(async () => {
    if (hasNextPage && !isFetchingNextPage) {
      await fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  // --- STATE TRANSFORMATION (using useMemo for performance) ---

  // Flattens the paginated data from React Query into a single array for UI rendering
  const allAssets = useMemo(
    () => data?.pages.flatMap((page) => page.assets) ?? [],
    [data],
  );

  // --- RETURN VALUE ---
  // We assemble the state and actions objects that our context will provide.
  const state: AssetsState = {
    assets: allAssets,
    filters,
    isLoading: isFetching && !isFetchingNextPage,
    isLoadingNextPage: isFetchingNextPage,
    error: error ? error.message : null,
    hasMore: hasNextPage,
  };

  const actions: AssetsActions = {
    fetchAssets,
    fetchNextPage: handleFetchNextPage,
    applyFilter,
    setSearchQuery,
    resetFilters,
  };

  return { state, actions };
}
