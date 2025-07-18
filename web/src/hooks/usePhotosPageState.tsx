import { useState, useEffect, useCallback } from "react";
import { useSearchParams } from "react-router-dom";

export type GroupByType = "date" | "type" | "album";
export type SortOrderType = "asc" | "desc";
export type ViewModeType = "masonry" | "grid";

interface PhotosPageState {
  selectedAssetId: string | null;
  isCarouselOpen: boolean;
  currentIndex: number;
  groupBy: GroupByType;
  sortOrder: SortOrderType;
  viewMode: ViewModeType;
  searchQuery: string;
}

interface PhotosPageActions {
  openCarousel: (assetId: string, index?: number) => void;
  closeCarousel: () => void;
  setGroupBy: (groupBy: GroupByType) => void;
  setSortOrder: (sortOrder: SortOrderType) => void;
  setViewMode: (viewMode: ViewModeType) => void;
  setSearchQuery: (query: string) => void;
  updateCarouselIndex: (index: number) => void;
  navigateToAsset: (assetId: string) => void;
}

const DEFAULT_STATE: PhotosPageState = {
  selectedAssetId: null,
  isCarouselOpen: false,
  currentIndex: 0,
  groupBy: "date",
  sortOrder: "desc",
  viewMode: "masonry",
  searchQuery: "",
};

export const usePhotosPageState = (): PhotosPageState & PhotosPageActions => {
  const [searchParams, setSearchParams] = useSearchParams();

  // Initialize state from URL or defaults
  const [state, setState] = useState<PhotosPageState>(() => {
    const assetId = searchParams.get("asset");
    const carousel = searchParams.get("carousel") === "true";
    const index = parseInt(searchParams.get("index") || "0", 10);
    const groupBy =
      (searchParams.get("groupBy") as GroupByType) || DEFAULT_STATE.groupBy;
    const sortOrder =
      (searchParams.get("sort") as SortOrderType) || DEFAULT_STATE.sortOrder;
    const viewMode =
      (searchParams.get("view") as ViewModeType) || DEFAULT_STATE.viewMode;
    const searchQuery = searchParams.get("q") || DEFAULT_STATE.searchQuery;

    return {
      selectedAssetId: assetId,
      isCarouselOpen: carousel && !!assetId,
      currentIndex: index,
      groupBy,
      sortOrder,
      viewMode,
      searchQuery,
    };
  });

  // Update URL when state changes
  const updateURL = useCallback(
    (newState: Partial<PhotosPageState>) => {
      const params = new URLSearchParams(searchParams);

      // Handle asset and carousel state
      if (newState.selectedAssetId) {
        params.set("asset", newState.selectedAssetId);
      } else {
        params.delete("asset");
      }

      if (newState.isCarouselOpen && newState.selectedAssetId) {
        params.set("carousel", "true");
        if (newState.currentIndex !== undefined && newState.currentIndex > 0) {
          params.set("index", newState.currentIndex.toString());
        } else {
          params.delete("index");
        }
      } else {
        params.delete("carousel");
        params.delete("index");
      }

      // Handle view preferences
      if (newState.groupBy && newState.groupBy !== DEFAULT_STATE.groupBy) {
        params.set("groupBy", newState.groupBy);
      } else if (newState.groupBy === DEFAULT_STATE.groupBy) {
        params.delete("groupBy");
      }

      if (
        newState.sortOrder &&
        newState.sortOrder !== DEFAULT_STATE.sortOrder
      ) {
        params.set("sort", newState.sortOrder);
      } else if (newState.sortOrder === DEFAULT_STATE.sortOrder) {
        params.delete("sort");
      }

      if (newState.viewMode && newState.viewMode !== DEFAULT_STATE.viewMode) {
        params.set("view", newState.viewMode);
      } else if (newState.viewMode === DEFAULT_STATE.viewMode) {
        params.delete("view");
      }

      if (newState.searchQuery && newState.searchQuery.trim()) {
        params.set("q", newState.searchQuery.trim());
      } else {
        params.delete("q");
      }

      setSearchParams(params, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  // Sync state changes with URL
  useEffect(() => {
    updateURL(state);
  }, [state, updateURL]);

  // Actions
  const openCarousel = useCallback((assetId: string, index: number = 0) => {
    setState((prev) => ({
      ...prev,
      selectedAssetId: assetId,
      isCarouselOpen: true,
      currentIndex: index,
    }));
  }, []);

  const closeCarousel = useCallback(() => {
    setState((prev) => ({
      ...prev,
      isCarouselOpen: false,
      selectedAssetId: null,
      currentIndex: 0,
    }));
  }, []);

  const setGroupBy = useCallback((groupBy: GroupByType) => {
    setState((prev) => ({
      ...prev,
      groupBy,
    }));
  }, []);

  const setSortOrder = useCallback((sortOrder: SortOrderType) => {
    setState((prev) => ({
      ...prev,
      sortOrder,
    }));
  }, []);

  const setViewMode = useCallback((viewMode: ViewModeType) => {
    setState((prev) => ({
      ...prev,
      viewMode,
    }));
  }, []);

  const setSearchQuery = useCallback((searchQuery: string) => {
    setState((prev) => ({
      ...prev,
      searchQuery,
    }));
  }, []);

  const updateCarouselIndex = useCallback((index: number) => {
    setState((prev) => ({
      ...prev,
      currentIndex: index,
    }));
  }, []);

  const navigateToAsset = useCallback((assetId: string) => {
    setState((prev) => ({
      ...prev,
      selectedAssetId: assetId,
    }));
  }, []);

  return {
    ...state,
    openCarousel,
    closeCarousel,
    setGroupBy,
    setSortOrder,
    setViewMode,
    setSearchQuery,
    updateCarouselIndex,
    navigateToAsset,
  };
};
