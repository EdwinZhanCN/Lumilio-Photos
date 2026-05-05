import { StateCreator } from "zustand";
import { SortByType, TabType, UIState } from "../types/assets.type";

export interface UISlice {
  ui: UIState;
  setCurrentTab: (tab: TabType) => void;
  setSortBy: (sortBy: SortByType) => void;
  setSearchQuery: (query: string) => void;
  setCarouselOpen: (isOpen: boolean) => void;
  setActiveAssetId: (assetId: string | undefined) => void;
  hydrateUI: (
    params: Partial<Pick<UIState, "sortBy" | "searchQuery">>,
  ) => void;
}

export const createUISlice: StateCreator<
  UISlice,
  [["zustand/immer", never]],
  [],
  UISlice
> = (set) => ({
  ui: {
    currentTab: "photos",
    sortBy: "date_captured",
    searchQuery: "",
    isCarouselOpen: false,
    activeAssetId: undefined,
  },

  setCurrentTab: (tab) =>
    set((state) => {
      if (state.ui.currentTab === tab) return;
      state.ui.currentTab = tab;
    }),

  setSortBy: (sortBy) =>
    set((state) => {
      if (state.ui.sortBy === sortBy) return;
      state.ui.sortBy = sortBy;
    }),

  setSearchQuery: (query) =>
    set((state) => {
      if (state.ui.searchQuery === query) return;
      state.ui.searchQuery = query;
    }),

  setCarouselOpen: (isOpen) =>
    set((state) => {
      if (
        state.ui.isCarouselOpen === isOpen &&
        (isOpen || state.ui.activeAssetId === undefined)
      ) {
        return;
      }
      state.ui.isCarouselOpen = isOpen;
      // Clear active asset when closing carousel
      if (!isOpen) {
        state.ui.activeAssetId = undefined;
      }
    }),

  setActiveAssetId: (assetId) =>
    set((state) => {
      if (
        state.ui.activeAssetId === assetId &&
        state.ui.isCarouselOpen === !!assetId
      ) {
        return;
      }
      state.ui.activeAssetId = assetId;
      // Open carousel when setting active asset, close when clearing asset
      state.ui.isCarouselOpen = !!assetId;
    }),

  hydrateUI: (params) =>
    set((state) => {
      if (params.sortBy && state.ui.sortBy !== params.sortBy) {
        state.ui.sortBy = params.sortBy;
      }
      if (
        params.searchQuery !== undefined &&
        state.ui.searchQuery !== params.searchQuery
      ) {
        state.ui.searchQuery = params.searchQuery;
      }
    }),
});

// Selectors
export const selectCurrentTab = (state: UISlice): TabType =>
  state.ui.currentTab;

export const selectSortBy = (state: UISlice): SortByType => state.ui.sortBy;

export const selectSearchQuery = (state: UISlice): string =>
  state.ui.searchQuery;

export const selectIsCarouselOpen = (state: UISlice): boolean =>
  state.ui.isCarouselOpen;

export const selectActiveAssetId = (state: UISlice): string | undefined =>
  state.ui.activeAssetId;

export const selectIsSearchActive = (state: UISlice): boolean => {
  return state.ui.searchQuery.trim().length > 0;
};

// Utility selectors (Static helpers)
export const selectTabAssetTypes = (tab: TabType): TabType[] => {
  switch (tab) {
    case "photos":
      return ["photos"];
    case "videos":
      return ["videos"];
    case "audios":
      return ["audios"];
    default:
      return ["photos"];
  }
};

export const selectTabTitle = (tab: TabType): string => {
  switch (tab) {
    case "photos":
      return "Photos";
    case "videos":
      return "Videos";
    case "audios":
      return "Audios";
    default:
      return "Photos";
  }
};

export const selectTabSupportsSemanticSearch = (tab: TabType): boolean => {
  // Only photos support semantic search currently
  return tab === "photos";
};
