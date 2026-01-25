import { StateCreator } from "zustand";
import { GroupByType, TabType, UIState } from "../types/assets.type";

export interface UISlice {
  ui: UIState;
  setCurrentTab: (tab: TabType) => void;
  setGroupBy: (groupBy: GroupByType) => void;
  setSearchQuery: (query: string) => void;
  setSearchMode: (mode: "filename" | "semantic") => void;
  setCarouselOpen: (isOpen: boolean) => void;
  setActiveAssetId: (assetId: string | undefined) => void;
  hydrateUIFromURL: (
    params: Partial<Pick<UIState, "groupBy" | "searchQuery">>,
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
    groupBy: "date",
    searchQuery: "",
    searchMode: "filename",
    isCarouselOpen: false,
    activeAssetId: undefined,
  },

  setCurrentTab: (tab) =>
    set((state) => {
      state.ui.currentTab = tab;
    }),

  setGroupBy: (groupBy) =>
    set((state) => {
      state.ui.groupBy = groupBy;
    }),

  setSearchQuery: (query) =>
    set((state) => {
      state.ui.searchQuery = query;
    }),

  setSearchMode: (mode) =>
    set((state) => {
      state.ui.searchMode = mode;
    }),

  setCarouselOpen: (isOpen) =>
    set((state) => {
      state.ui.isCarouselOpen = isOpen;
      // Clear active asset when closing carousel
      if (!isOpen) {
        state.ui.activeAssetId = undefined;
      }
    }),

  setActiveAssetId: (assetId) =>
    set((state) => {
      state.ui.activeAssetId = assetId;
      // Open carousel when setting active asset, close when clearing asset
      state.ui.isCarouselOpen = !!assetId;
    }),

  hydrateUIFromURL: (params) =>
    set((state) => {
      if (params.groupBy) {
        state.ui.groupBy = params.groupBy;
      }
      if (params.searchQuery !== undefined) {
        state.ui.searchQuery = params.searchQuery;
      }
    }),
});

// Selectors
export const selectCurrentTab = (state: UISlice): TabType =>
  state.ui.currentTab;

export const selectGroupBy = (state: UISlice): GroupByType => state.ui.groupBy;

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
