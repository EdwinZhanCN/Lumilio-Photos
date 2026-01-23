import { AssetsAction, SelectionState } from "../assets.types.ts";

export const initialSelectionState: SelectionState = {
  enabled: false,
  selectedIds: new Set(),
  lastSelectedId: undefined,
  selectionMode: "multiple",
};

export const selectionReducer = (
  state: SelectionState = initialSelectionState,
  action: AssetsAction,
): SelectionState => {
  switch (action.type) {
    case "SET_SELECTION_ENABLED":
      return {
        ...state,
        enabled: action.payload,
        // Clear selection when disabling
        selectedIds: action.payload ? state.selectedIds : new Set(),
        lastSelectedId: action.payload ? state.lastSelectedId : undefined,
      };

    case "TOGGLE_ASSET_SELECTION": {
      const { assetId } = action.payload;
      const newSelectedIds = new Set(state.selectedIds);

      if (newSelectedIds.has(assetId)) {
        newSelectedIds.delete(assetId);
        return {
          ...state,
          selectedIds: newSelectedIds,
          lastSelectedId: newSelectedIds.size > 0 ? state.lastSelectedId : undefined,
        };
      } else {
        // Handle selection mode
        if (state.selectionMode === "single") {
          newSelectedIds.clear();
        }
        newSelectedIds.add(assetId);
        return {
          ...state,
          selectedIds: newSelectedIds,
          lastSelectedId: assetId,
        };
      }
    }

    case "SELECT_ASSET": {
      const { assetId } = action.payload;
      const newSelectedIds = new Set(state.selectedIds);

      // Handle selection mode
      if (state.selectionMode === "single") {
        newSelectedIds.clear();
      }
      newSelectedIds.add(assetId);

      return {
        ...state,
        selectedIds: newSelectedIds,
        lastSelectedId: assetId,
      };
    }

    case "DESELECT_ASSET": {
      const { assetId } = action.payload;
      const newSelectedIds = new Set(state.selectedIds);
      newSelectedIds.delete(assetId);

      return {
        ...state,
        selectedIds: newSelectedIds,
        lastSelectedId: newSelectedIds.size > 0 ? state.lastSelectedId : undefined,
      };
    }

    case "SELECT_ALL": {
      const { assetIds } = action.payload;
      return {
        ...state,
        selectedIds: new Set(assetIds),
        lastSelectedId: assetIds[assetIds.length - 1],
      };
    }

    case "CLEAR_SELECTION":
      return {
        ...state,
        selectedIds: new Set(),
        lastSelectedId: undefined,
      };

    case "SET_SELECTION_MODE":
      return {
        ...state,
        selectionMode: action.payload,
        // Clear selection when switching to single mode if multiple items selected
        selectedIds: action.payload === "single" && state.selectedIds.size > 1
          ? new Set(state.lastSelectedId ? [state.lastSelectedId] : [])
          : state.selectedIds,
      };

    // Clean up selection when assets are deleted
    case "DELETE_ENTITY": {
      const { assetId } = action.payload;
      if (state.selectedIds.has(assetId)) {
        const newSelectedIds = new Set(state.selectedIds);
        newSelectedIds.delete(assetId);
        return {
          ...state,
          selectedIds: newSelectedIds,
          lastSelectedId: state.lastSelectedId === assetId
            ? (newSelectedIds.size > 0 ? Array.from(newSelectedIds)[0] : undefined)
            : state.lastSelectedId,
        };
      }
      return state;
    }

    case "REMOVE_ASSET_FROM_VIEWS": {
      const { assetId } = action.payload;
      if (state.selectedIds.has(assetId)) {
        const newSelectedIds = new Set(state.selectedIds);
        newSelectedIds.delete(assetId);
        return {
          ...state,
          selectedIds: newSelectedIds,
          lastSelectedId: state.lastSelectedId === assetId
            ? (newSelectedIds.size > 0 ? Array.from(newSelectedIds)[0] : undefined)
            : state.lastSelectedId,
        };
      }
      return state;
    }

    default:
      return state;
  }
};

// Selectors
export const selectSelectionEnabled = (state: SelectionState): boolean => state.enabled;

export const selectSelectedIds = (state: SelectionState): Set<string> => state.selectedIds;

export const selectSelectedCount = (state: SelectionState): number => state.selectedIds.size;

export const selectIsAssetSelected = (state: SelectionState, assetId: string): boolean => {
  return state.selectedIds.has(assetId);
};

export const selectLastSelectedId = (state: SelectionState): string | undefined => {
  return state.lastSelectedId;
};

export const selectSelectionMode = (state: SelectionState): "single" | "multiple" => {
  return state.selectionMode;
};

export const selectHasSelection = (state: SelectionState): boolean => {
  return state.selectedIds.size > 0;
};

export const selectSelectedAsArray = (state: SelectionState): string[] => {
  return Array.from(state.selectedIds);
};

// Utility functions
export const isSelectionEmpty = (state: SelectionState): boolean => {
  return state.selectedIds.size === 0;
};

export const canSelectMore = (state: SelectionState): boolean => {
  return state.selectionMode === "multiple" || state.selectedIds.size === 0;
};

export const shouldClearOnSingleMode = (state: SelectionState): boolean => {
  return state.selectionMode === "single" && state.selectedIds.size > 1;
};
