import { AssetsAction, AssetsState } from "./types";
import {
  entitiesReducer,
  initialEntitiesState,
} from "./reducers/entities.reducer";
import { viewsReducer, initialViewsState } from "./reducers/views.reducer";
import { uiReducer, initialUIState } from "./reducers/ui.reducer";
import {
  filtersReducer,
  initialFiltersState,
} from "./reducers/filters.reducer";
import {
  selectionReducer,
  initialSelectionState,
} from "./reducers/selection.reducer";

export const initialAssetsState: AssetsState = {
  entities: initialEntitiesState,
  views: initialViewsState,
  ui: initialUIState,
  filters: initialFiltersState,
  selection: initialSelectionState,
};

export const assetsReducer = (
  state: AssetsState = initialAssetsState,
  action: AssetsAction,
): AssetsState => {
  return {
    entities: entitiesReducer(state.entities, action),
    views: viewsReducer(state.views, action),
    ui: uiReducer(state.ui, action),
    filters: filtersReducer(state.filters, action),
    selection: selectionReducer(state.selection, action),
  };
};

// Root selectors that combine multiple slices
// These selectors are deprecated - use useAssetsView hook instead
// Keeping for backwards compatibility but they return empty arrays

// Cleanup utility
export const cleanupAssetsState = (state: AssetsState): AssetsState => {
  // Remove stale views older than 30 minutes
  const maxAge = 30 * 60 * 1000;
  const now = Date.now();
  const activeViewKeys = new Set(state.views.activeViewKeys);
  const cleanedViews = { ...state.views.views };

  Object.entries(cleanedViews).forEach(([key, view]) => {
    if (!activeViewKeys.has(key) && now - view.lastFetchAt > maxAge) {
      delete cleanedViews[key];
    }
  });

  return {
    ...state,
    views: {
      ...state.views,
      views: cleanedViews,
    },
  };
};
