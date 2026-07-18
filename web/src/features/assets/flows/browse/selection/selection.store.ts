import { createContext, useContext } from "react";
import { enableMapSet } from "immer";
import { useStore } from "zustand";
import { immer } from "zustand/middleware/immer";
import { createStore, type StoreApi } from "zustand/vanilla";

enableMapSet();

export interface AssetSelectionState {
  enabled: boolean;
  selectedIds: Set<string>;
  lastSelectedId?: string;
  selectionMode: "single" | "multiple";
}

export interface AssetSelectionStore {
  selection: AssetSelectionState;
  toggleAssetSelection: (assetId: string) => void;
  clearSelection: () => void;
  setSelectionEnabled: (enabled: boolean) => void;
  selectAsset: (assetId: string) => void;
  deselectAsset: (assetId: string) => void;
  selectAll: (assetIds: string[]) => void;
  setSelectionMode: (mode: "single" | "multiple") => void;
}

export type AssetSelectionStoreApi = StoreApi<AssetSelectionStore>;

export type AssetSelectionInitialState = Partial<Omit<AssetSelectionState, "selectedIds">> & {
  selectedIds?: Set<string> | string[];
};

const normalizeSelectedIds = (selectedIds?: Set<string> | string[]): Set<string> => {
  if (selectedIds instanceof Set) return new Set(selectedIds);
  return new Set(selectedIds ?? []);
};

export const createAssetSelectionStore = (
  initialState: AssetSelectionInitialState = {},
): AssetSelectionStoreApi =>
  createStore<AssetSelectionStore>()(
    immer((set) => ({
      selection: {
        enabled: false,
        selectionMode: "multiple",
        ...initialState,
        selectedIds: normalizeSelectedIds(initialState.selectedIds),
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
          state.selection.selectedIds.clear();
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
          if (state.selection.selectedIds.size === 0) {
            state.selection.lastSelectedId = undefined;
          }
        }),
      selectAll: (assetIds) =>
        set((state) => {
          state.selection.selectedIds = new Set(assetIds);
          state.selection.lastSelectedId = assetIds.at(-1);
        }),
      setSelectionMode: (selectionMode) =>
        set((state) => {
          state.selection.selectionMode = selectionMode;
          if (selectionMode === "single" && state.selection.selectedIds.size > 1) {
            state.selection.selectedIds = new Set(
              state.selection.lastSelectedId ? [state.selection.lastSelectedId] : [],
            );
          }
        }),
    })),
  );

export const AssetSelectionStoreContext = createContext<AssetSelectionStoreApi | null>(null);

export const useAssetSelectionStoreApi = (): AssetSelectionStoreApi => {
  const store = useContext(AssetSelectionStoreContext);
  if (!store) {
    throw new Error("Asset selection must be used within an AssetBrowserScope");
  }
  return store;
};

export function useAssetSelectionStore<Value>(
  selector: (state: AssetSelectionStore) => Value,
): Value {
  return useStore(useAssetSelectionStoreApi(), selector);
}

export const selectLastSelectedId = (state: AssetSelectionStore): string | undefined =>
  state.selection.lastSelectedId;
