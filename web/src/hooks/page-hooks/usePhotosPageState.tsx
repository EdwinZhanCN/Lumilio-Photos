import { useState, useEffect, useCallback } from "react";
import { useSearchParams, useNavigate, useParams } from "react-router-dom";

export type GroupByType = "date" | "type" | "album";
export type SortOrderType = "asc" | "desc";
export type ViewModeType = "masonry" | "grid";

interface PhotosPageState {
  isCarouselOpen: boolean;
  groupBy: GroupByType;
  sortOrder: SortOrderType;
  viewMode: ViewModeType;
  searchQuery: string;
}

interface PhotosPageActions {
  openCarousel: (assetId: string) => void;
  closeCarousel: () => void;
  setGroupBy: (groupBy: GroupByType) => void;
  setSortOrder: (sortOrder: SortOrderType) => void;
  setViewMode: (viewMode: ViewModeType) => void;
  setSearchQuery: (query: string) => void;
}

const DEFAULT_STATE: Omit<PhotosPageState, "isCarouselOpen"> = {
  groupBy: "date",
  sortOrder: "desc",
  viewMode: "masonry",
  searchQuery: "",
};

export const usePhotosPageState = (): PhotosPageState & PhotosPageActions => {
  const navigate = useNavigate();
  const { assetId } = useParams<{ assetId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const [state, setState] = useState<Omit<PhotosPageState, "isCarouselOpen">>(
    () => ({
      groupBy:
        (searchParams.get("groupBy") as GroupByType) || DEFAULT_STATE.groupBy,
      sortOrder:
        (searchParams.get("sort") as SortOrderType) || DEFAULT_STATE.sortOrder,
      viewMode:
        (searchParams.get("view") as ViewModeType) || DEFAULT_STATE.viewMode,
      searchQuery: searchParams.get("q") || DEFAULT_STATE.searchQuery,
    }),
  );

  const isCarouselOpen = !!assetId;

  const updateURL = useCallback(() => {
    const params = new URLSearchParams();
    if (state.groupBy !== DEFAULT_STATE.groupBy) {
      params.set("groupBy", state.groupBy);
    }
    if (state.sortOrder !== DEFAULT_STATE.sortOrder) {
      params.set("sort", state.sortOrder);
    }
    if (state.viewMode !== DEFAULT_STATE.viewMode) {
      params.set("view", state.viewMode);
    }
    if (state.searchQuery) {
      params.set("q", state.searchQuery);
    }
    setSearchParams(params, { replace: true });
  }, [state, setSearchParams]);

  useEffect(() => {
    updateURL();
  }, [state, updateURL]);

  const openCarousel = useCallback(
    (newAssetId: string) => {
      const currentParams = new URLSearchParams(searchParams);
      navigate(`/photos/${newAssetId}?${currentParams.toString()}`);
    },
    [navigate, searchParams],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = new URLSearchParams(searchParams);
    navigate(`/photos?${currentParams.toString()}`);
  }, [navigate, searchParams]);

  const setGroupBy = (groupBy: GroupByType) =>
    setState((prev) => ({ ...prev, groupBy }));
  const setSortOrder = (sortOrder: SortOrderType) =>
    setState((prev) => ({ ...prev, sortOrder }));
  const setViewMode = (viewMode: ViewModeType) =>
    setState((prev) => ({ ...prev, viewMode }));
  const setSearchQuery = (searchQuery: string) =>
    setState((prev) => ({ ...prev, searchQuery }));

  return {
    ...state,
    isCarouselOpen,
    openCarousel,
    closeCarousel,
    setGroupBy,
    setSortOrder,
    setViewMode,
    setSearchQuery,
  };
};
