import { StateCreator } from "zustand";
import { AssetViewDefinition, ViewState, ViewsState } from "../types/assets.type";

// Helper functions
const createInitialViewState = (
  definition: AssetViewDefinition,
): ViewState => ({
  assetIds: [],
  isLoading: false,
  isLoadingMore: false,
  hasMore: true,
  error: null,
  pageInfo: {
    cursor: undefined,
    page: 1,
    total: undefined,
  },
  definitionHash: generateViewKey(definition),
  lastFetchAt: 0,
});

export const generateViewKey = (definition: AssetViewDefinition): string => {
  // If manual key is provided, use it
  if (definition.key) {
    return definition.key;
  }

  // Create a stable hash from the definition
  const normalizedDef = {
    types: definition.types?.sort() || [],
    filter: definition.filter || {},
    inheritGlobalFilter: definition.inheritGlobalFilter ?? true,
    search: definition.search,
    groupBy: definition.groupBy || "date",
    sort: definition.sort || { field: "taken_time", direction: "desc" },
    pageSize: definition.pageSize || 50,
    pagination: definition.pagination || "cursor",
  };

  const hashInput = JSON.stringify(normalizedDef);

  // Simple hash function for browser environment
  let hash = 0;
  for (let i = 0; i < hashInput.length; i++) {
    const char = hashInput.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash = hash & hash; // Convert to 32-bit integer
  }

  return `view_${Math.abs(hash).toString(36)}`;
};

export interface ViewsSlice {
  views: Record<string, ViewState>;
  activeViewKeys: string[];

  createView: (viewKey: string, definition: AssetViewDefinition) => void;
  setViewLoading: (viewKey: string, loading: boolean) => void;
  setViewAssets: (
    viewKey: string,
    assetIds: string[],
    hasMore: boolean,
    pageInfo: ViewState["pageInfo"],
    replace?: boolean,
  ) => void;
  appendViewAssets: (
    viewKey: string,
    assetIds: string[],
    hasMore: boolean,
    pageInfo: ViewState["pageInfo"],
  ) => void;
  setViewError: (viewKey: string, error: string | null) => void;
  setViewLoadingMore: (viewKey: string, loading: boolean) => void;
  removeView: (viewKey: string) => void;
  removeAssetFromViews: (assetId: string) => void;
  cleanupStaleViews: (maxAge?: number) => void;
}

export const createViewsSlice: StateCreator<
  ViewsSlice,
  [["zustand/immer", never]],
  [],
  ViewsSlice
> = (set) => ({
  views: {},
  activeViewKeys: [],

  createView: (viewKey, definition) =>
    set((state) => {
      if (state.views[viewKey]) {
        if (!state.activeViewKeys.includes(viewKey)) {
          state.activeViewKeys.push(viewKey);
        }
        return;
      }
      state.views[viewKey] = createInitialViewState(definition);
      state.activeViewKeys.push(viewKey);
    }),

  setViewLoading: (viewKey, loading) =>
    set((state) => {
      const view = state.views[viewKey];
      if (view) {
        view.isLoading = loading;
        if (loading) view.error = null;
      }
    }),

  setViewAssets: (viewKey, assetIds, hasMore, pageInfo, replace = true) =>
    set((state) => {
      const view = state.views[viewKey];
      if (view) {
        view.assetIds = replace ? assetIds : [...view.assetIds, ...assetIds];
        view.hasMore = hasMore;
        view.pageInfo = pageInfo;
        view.isLoading = false;
        view.error = null;
        view.lastFetchAt = Date.now();
      }
    }),

  appendViewAssets: (viewKey, assetIds, hasMore, pageInfo) =>
    set((state) => {
      const view = state.views[viewKey];
      if (view) {
        const existingIds = new Set(view.assetIds);
        const newIds = assetIds.filter((id) => !existingIds.has(id));
        view.assetIds.push(...newIds);
        view.hasMore = hasMore;
        view.pageInfo = pageInfo;
        view.isLoadingMore = false;
        view.lastFetchAt = Date.now();
      }
    }),

  setViewError: (viewKey, error) =>
    set((state) => {
      const view = state.views[viewKey];
      if (view) {
        view.error = error;
        view.isLoading = false;
        view.isLoadingMore = false;
      }
    }),

  setViewLoadingMore: (viewKey, loading) =>
    set((state) => {
      const view = state.views[viewKey];
      if (view) {
        view.isLoadingMore = loading;
      }
    }),

  removeView: (viewKey) =>
    set((state) => {
      delete state.views[viewKey];
      state.activeViewKeys = state.activeViewKeys.filter(
        (key) => key !== viewKey,
      );
    }),

  removeAssetFromViews: (assetId) =>
    set((state) => {
      Object.values(state.views).forEach((view) => {
        const newIds = view.assetIds.filter((id) => id !== assetId);
        if (newIds.length !== view.assetIds.length) {
          view.assetIds = newIds;
        }
      });
    }),

  cleanupStaleViews: (maxAge = 30 * 60 * 1000) =>
    set((state) => {
      const now = Date.now();
      const activeKeys = new Set(state.activeViewKeys);
      Object.keys(state.views).forEach((key) => {
        if (
          !activeKeys.has(key) &&
          now - state.views[key].lastFetchAt > maxAge
        ) {
          delete state.views[key];
        }
      });
    }),
});

// Selectors - work with both ViewsSlice (store) and ViewsState (legacy context)
type ViewsInput = ViewsSlice | ViewsState;

// Helper to normalize input
const getViewsState = (input: ViewsInput): ViewsState => {
  // Check if it has slice actions (meaning it's ViewsSlice)
  if ('createView' in input) {
    // It's a ViewsSlice, extract just the state portions
    return { views: input.views, activeViewKeys: input.activeViewKeys };
  }
  // Otherwise it's already ViewsState
  return input;
};


export const selectView = (
  input: ViewsInput,
  viewKey: string,
): ViewState | undefined => {
  const state = getViewsState(input);
  return state.views[viewKey];
};

// Stable empty array to avoid creating new instances on each selector call
const EMPTY_ASSET_IDS: string[] = [];

export const selectViewAssetIds = (
  input: ViewsInput,
  viewKey: string,
): string[] => {
  const state = getViewsState(input);
  const view = state.views[viewKey];
  return view?.assetIds || EMPTY_ASSET_IDS;
};

export const selectActiveViews = (
  input: ViewsInput,
): Record<string, ViewState> => {
  const state = getViewsState(input);
  const activeViews: Record<string, ViewState> = {};
  state.activeViewKeys.forEach((key) => {
    if (state.views[key]) {
      activeViews[key] = state.views[key];
    }
  });
  return activeViews;
};

export const selectViewExists = (
  input: ViewsInput,
  viewKey: string,
): boolean => {
  const state = getViewsState(input);
  return viewKey in state.views;
};

