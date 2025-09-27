import {
  AssetsAction,
  ViewsState,
  ViewState,
  AssetViewDefinition,
} from "../types";
// Browser-compatible hash function (no crypto import needed)

export const initialViewsState: ViewsState = {
  views: {},
  activeViewKeys: [],
};

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

export const viewsReducer = (
  state: ViewsState = initialViewsState,
  action: AssetsAction,
): ViewsState => {
  switch (action.type) {
    case "CREATE_VIEW": {
      const { viewKey, definition } = action.payload;

      // Don't recreate if already exists
      if (state.views[viewKey]) {
        return {
          ...state,
          activeViewKeys: state.activeViewKeys.includes(viewKey)
            ? state.activeViewKeys
            : [...state.activeViewKeys, viewKey],
        };
      }

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: createInitialViewState(definition),
        },
        activeViewKeys: [...state.activeViewKeys, viewKey],
      };
    }

    case "SET_VIEW_LOADING": {
      const { viewKey, loading } = action.payload;
      const view = state.views[viewKey];
      if (!view) return state;

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: {
            ...view,
            isLoading: loading,
            error: loading ? null : view.error,
          },
        },
      };
    }

    case "SET_VIEW_ASSETS": {
      const {
        viewKey,
        assetIds,
        hasMore,
        pageInfo,
        replace = true,
      } = action.payload;
      const view = state.views[viewKey];
      if (!view) return state;

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: {
            ...view,
            assetIds: replace ? assetIds : [...view.assetIds, ...assetIds],
            hasMore,
            pageInfo,
            isLoading: false,
            error: null,
            lastFetchAt: Date.now(),
          },
        },
      };
    }

    case "APPEND_VIEW_ASSETS": {
      const { viewKey, assetIds, hasMore, pageInfo } = action.payload;
      const view = state.views[viewKey];
      if (!view) return state;

      // Avoid duplicates when appending
      const existingIds = new Set(view.assetIds);
      const newIds = assetIds.filter((id) => !existingIds.has(id));

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: {
            ...view,
            assetIds: [...view.assetIds, ...newIds],
            hasMore,
            pageInfo,
            isLoadingMore: false,
            lastFetchAt: Date.now(),
          },
        },
      };
    }

    case "SET_VIEW_ERROR": {
      const { viewKey, error } = action.payload;
      const view = state.views[viewKey];
      if (!view) return state;

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: {
            ...view,
            error,
            isLoading: false,
            isLoadingMore: false,
          },
        },
      };
    }

    case "SET_VIEW_LOADING_MORE": {
      const { viewKey, loading } = action.payload;
      const view = state.views[viewKey];
      if (!view) return state;

      return {
        ...state,
        views: {
          ...state.views,
          [viewKey]: {
            ...view,
            isLoadingMore: loading,
          },
        },
      };
    }

    case "REMOVE_VIEW": {
      const { viewKey } = action.payload;
      const newViews = { ...state.views };
      delete newViews[viewKey];

      return {
        ...state,
        views: newViews,
        activeViewKeys: state.activeViewKeys.filter((key) => key !== viewKey),
      };
    }

    case "REMOVE_ASSET_FROM_VIEWS": {
      const { assetId } = action.payload;
      const updatedViews = { ...state.views };

      // Remove asset from all views
      Object.keys(updatedViews).forEach((viewKey) => {
        const view = updatedViews[viewKey];
        const filteredIds = view.assetIds.filter((id) => id !== assetId);
        if (filteredIds.length !== view.assetIds.length) {
          updatedViews[viewKey] = {
            ...view,
            assetIds: filteredIds,
          };
        }
      });

      return {
        ...state,
        views: updatedViews,
      };
    }

    default:
      return state;
  }
};

// Selectors
export const selectView = (
  state: ViewsState,
  viewKey: string,
): ViewState | undefined => {
  return state.views[viewKey];
};

export const selectViewAssetIds = (
  state: ViewsState,
  viewKey: string,
): string[] => {
  const view = state.views[viewKey];
  return view?.assetIds || [];
};

export const selectActiveViews = (
  state: ViewsState,
): Record<string, ViewState> => {
  const activeViews: Record<string, ViewState> = {};
  state.activeViewKeys.forEach((key) => {
    if (state.views[key]) {
      activeViews[key] = state.views[key];
    }
  });
  return activeViews;
};

export const selectViewExists = (
  state: ViewsState,
  viewKey: string,
): boolean => {
  return viewKey in state.views;
};

// Cleanup utilities
export const cleanupStaleViews = (
  state: ViewsState,
  maxAge: number = 30 * 60 * 1000,
): ViewsState => {
  const now = Date.now();
  const activeKeys = new Set(state.activeViewKeys);
  const cleanedViews = { ...state.views };

  Object.entries(cleanedViews).forEach(([key, view]) => {
    // Keep active views and recently accessed views
    if (!activeKeys.has(key) && now - view.lastFetchAt > maxAge) {
      delete cleanedViews[key];
    }
  });

  return {
    ...state,
    views: cleanedViews,
  };
};
