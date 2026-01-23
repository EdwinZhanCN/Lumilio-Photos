import { AssetsAction, UIState, TabType } from "../assets.types.ts";

export const initialUIState: UIState = {
  currentTab: "photos",
  groupBy: "date",
  searchQuery: "",
  searchMode: "filename",
  isCarouselOpen: false,
  activeAssetId: undefined,
};

export const uiReducer = (
  state: UIState = initialUIState,
  action: AssetsAction,
): UIState => {
  switch (action.type) {
    case "SET_CURRENT_TAB":
      return {
        ...state,
        currentTab: action.payload,
      };

    case "SET_GROUP_BY":
      return {
        ...state,
        groupBy: action.payload,
      };

    case "SET_SEARCH_QUERY":
      return {
        ...state,
        searchQuery: action.payload,
      };

    case "SET_SEARCH_MODE":
      return {
        ...state,
        searchMode: action.payload,
      };

    case "SET_CAROUSEL_OPEN":
      return {
        ...state,
        isCarouselOpen: action.payload,
        // Clear active asset when closing carousel
        activeAssetId: action.payload ? state.activeAssetId : undefined,
      };

    case "SET_ACTIVE_ASSET_ID":
      return {
        ...state,
        activeAssetId: action.payload,
        // Open carousel when setting active asset, close when clearing asset
        isCarouselOpen: !!action.payload,
      };

    case "HYDRATE_UI_FROM_URL":
      return {
        ...state,
        groupBy: action.payload.groupBy ?? state.groupBy,
        searchQuery: action.payload.searchQuery ?? state.searchQuery,
      };

    default:
      return state;
  }
};

// Selectors
export const selectCurrentTab = (state: UIState): TabType => state.currentTab;

export const selectGroupBy = (state: UIState) => state.groupBy;

export const selectSearchQuery = (state: UIState): string => state.searchQuery;

export const selectIsCarouselOpen = (state: UIState): boolean =>
  state.isCarouselOpen;

export const selectActiveAssetId = (state: UIState): string | undefined =>
  state.activeAssetId;

export const selectIsSearchActive = (state: UIState): boolean => {
  return state.searchQuery.trim().length > 0;
};

// Utility selectors
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
