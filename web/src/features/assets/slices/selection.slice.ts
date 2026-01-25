import { StateCreator } from "zustand";
import { SelectionState } from "../types/assets.type";

export interface SelectionSlice {
  selection: SelectionState;
  toggleAssetSelection: (assetId: string) => void;
  clearSelection: () => void;
  setSelectionEnabled: (enabled: boolean) => void;
  selectAsset: (assetId: string) => void;
  deselectAsset: (assetId: string) => void;
  selectAll: (assetIds: string[]) => void;
  setSelectionMode: (mode: "single" | "multiple") => void;
}

export const createSelectionSlice: StateCreator<
  SelectionSlice,
  [["zustand/immer", never]],
  [],
  SelectionSlice
> = (set) => ({
  selection: {
    enabled: false,
    selectedIds: new Set<string>(),
    selectionMode: "multiple",
    lastSelectedId: undefined,
  },

  toggleAssetSelection: (assetId) =>
    set((state) => {
      if (state.selection.selectedIds.has(assetId)) {
        state.selection.selectedIds.delete(assetId);
      } else {
        if (state.selection.selectionMode === "single") {
          state.selection.selectedIds.clear();
        }
        state.selection.selectedIds.add(assetId);
      }
      state.selection.lastSelectedId = assetId;
    }),

  clearSelection: () =>
    set((state) => {
      state.selection.selectedIds.clear();
      state.selection.lastSelectedId = undefined;
    }),

  setSelectionEnabled: (enabled) =>
    set((state) => {
      state.selection.enabled = enabled;
      state.selection.selectedIds = new Set<string>();
      state.selection.lastSelectedId = undefined;
    }),

  selectAsset: (assetId) =>
    set((state) => {
      if (state.selection.selectionMode === "single") {
        state.selection.selectedIds.clear();
      }
      state.selection.selectedIds.add(assetId);
      state.selection.lastSelectedId = assetId;
    }),

  deselectAsset: (assetId) =>
    set((state) => {
      state.selection.selectedIds.delete(assetId);
      state.selection.lastSelectedId =
        state.selection.selectedIds.size > 0
          ? state.selection.lastSelectedId
          : undefined;
    }),

  selectAll: (assetIds) =>
    set((state) => {
      state.selection.selectedIds = new Set(assetIds);
      state.selection.lastSelectedId = assetIds[assetIds.length - 1];
    }),

  setSelectionMode: (mode) =>
    set((state) => {
      state.selection.selectionMode = mode;
      // Clear selection when switching to single mode if multiple items selected
      if (mode === "single" && state.selection.selectedIds.size > 1) {
        state.selection.selectedIds = new Set(
          state.selection.lastSelectedId ? [state.selection.lastSelectedId] : [],
        );
      }
    }),
});

// Selectors - work with both SelectionSlice (store) and SelectionState (legacy context)
type SelectionInput = SelectionSlice | SelectionState;

// Helper to normalize input - handles both slice shape and direct state shape
const getSelectionState = (input: SelectionInput): SelectionState => {
  if ('selection' in input && input.selection && 'selectedIds' in input.selection) {
    return input.selection;
  }
  return input as SelectionState;
};

export const selectSelectionEnabled = (input: SelectionInput): boolean => {
  const state = getSelectionState(input);
  return state.enabled;
};

export const selectSelectedIds = (input: SelectionInput): Set<string> => {
  const state = getSelectionState(input);
  return state.selectedIds;
};

export const selectSelectedCount = (input: SelectionInput): number => {
  const state = getSelectionState(input);
  return state.selectedIds.size;
};

export const selectIsSelected = (
  input: SelectionInput,
  assetId: string,
): boolean => {
  const state = getSelectionState(input);
  return state.selectedIds.has(assetId);
};

// Alias for hooks that use this name
export const selectIsAssetSelected = selectIsSelected;

export const selectSelectionMode = (
  input: SelectionInput,
): "single" | "multiple" => {
  const state = getSelectionState(input);
  return state.selectionMode;
};

export const selectHasSelection = (input: SelectionInput): boolean => {
  const state = getSelectionState(input);
  return state.selectedIds.size > 0;
};

export const selectLastSelectedId = (input: SelectionInput): string | undefined => {
  const state = getSelectionState(input);
  return state.lastSelectedId;
};

export const selectSelectedAsArray = (input: SelectionInput): string[] => {
  const state = getSelectionState(input);
  return Array.from(state.selectedIds);
};
