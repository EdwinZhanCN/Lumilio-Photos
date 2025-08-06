import { useState, useEffect, useCallback } from "react";
import {
  useSearchParams,
  useNavigate,
  useParams,
  useLocation,
} from "react-router-dom";

export type GroupByType = "date" | "type" | "album";
export type SortOrderType = "asc" | "desc";

interface AssetsPageState {
  isCarouselOpen: boolean;
  groupBy: GroupByType;
  sortOrder: SortOrderType;
  searchQuery: string;
}

interface AssetsPageActions {
  openCarousel: (assetId: string) => void;
  closeCarousel: () => void;
  setGroupBy: (groupBy: GroupByType) => void;
  setSortOrder: (sortOrder: SortOrderType) => void;
  setSearchQuery: (query: string) => void;
}

const DEFAULT_STATE: Omit<AssetsPageState, "isCarouselOpen"> = {
  groupBy: "date",
  sortOrder: "desc",
  searchQuery: "",
};

export const useAssetsPageState = (): AssetsPageState & AssetsPageActions => {
  const navigate = useNavigate();
  const location = useLocation();
  const { assetId } = useParams<{ assetId: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  const [state, setState] = useState<Omit<AssetsPageState, "isCarouselOpen">>(
    () => ({
      groupBy:
        (searchParams.get("groupBy") as GroupByType) || DEFAULT_STATE.groupBy,
      sortOrder:
        (searchParams.get("sort") as SortOrderType) || DEFAULT_STATE.sortOrder,
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
      const path = location.pathname;

      // Determine the correct base path based on current location
      let basePath = "/assets/photos";
      if (path.includes("/videos")) {
        basePath = "/assets/videos";
      } else if (path.includes("/audios")) {
        basePath = "/assets/audios";
      }

      navigate(`${basePath}/${newAssetId}?${currentParams.toString()}`);
    },
    [navigate, searchParams, location.pathname],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = new URLSearchParams(searchParams);
    const path = location.pathname;

    // Determine the correct base path based on current location
    let basePath = "/assets/photos";
    if (path.includes("/videos")) {
      basePath = "/assets/videos";
    } else if (path.includes("/audios")) {
      basePath = "/assets/audios";
    }

    navigate(`${basePath}?${currentParams.toString()}`);
  }, [navigate, searchParams, location.pathname]);

  const setGroupBy = (groupBy: GroupByType) =>
    setState((prev) => ({ ...prev, groupBy }));
  const setSortOrder = (sortOrder: SortOrderType) =>
    setState((prev) => ({ ...prev, sortOrder }));
  const setSearchQuery = (searchQuery: string) =>
    setState((prev) => ({ ...prev, searchQuery }));

  return {
    ...state,
    isCarouselOpen,
    openCarousel,
    closeCarousel,
    setGroupBy,
    setSortOrder,
    setSearchQuery,
  };
};
