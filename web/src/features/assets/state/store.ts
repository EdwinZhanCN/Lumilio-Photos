import { createContext, useContext } from "react";
import { createStore, StoreApi } from "zustand/vanilla";
import { useStore } from "zustand";
import { immer } from "zustand/middleware/immer";
import { enableMapSet } from "immer";
import { createUISlice, UISlice } from "./slices/ui.slice";
import { createFiltersSlice, FiltersSlice } from "./slices/filters.slice";
import { createSelectionSlice, SelectionSlice } from "./slices/selection.slice";
import { FiltersState, SelectionState, UIState } from "../types";

enableMapSet();

export type AssetsStore = UISlice & FiltersSlice & SelectionSlice;
export type AssetsStoreApi = StoreApi<AssetsStore>;

export type AssetsStoreInitialState = {
  ui?: Partial<UIState>;
  filters?: Partial<FiltersState>;
  selection?: Partial<Omit<SelectionState, "selectedIds">> & {
    selectedIds?: Set<string> | string[];
  };
};

const normalizeSelectedIds = (selectedIds?: Set<string> | string[]): Set<string> => {
  if (selectedIds instanceof Set) return new Set(selectedIds);
  if (Array.isArray(selectedIds)) return new Set(selectedIds);
  return new Set<string>();
};

export const createAssetsStore = (initialState: AssetsStoreInitialState = {}): AssetsStoreApi =>
  createStore<AssetsStore>()(
    immer((set, get, store) => {
      const uiSlice = createUISlice(set as any, get as any, store as any);
      const filtersSlice = createFiltersSlice(set as any, get as any, store as any);
      const selectionSlice = createSelectionSlice(set as any, get as any, store as any);

      return {
        ...uiSlice,
        ...filtersSlice,
        ...selectionSlice,
        ui: {
          ...uiSlice.ui,
          ...initialState.ui,
        },
        filters: {
          ...filtersSlice.filters,
          ...initialState.filters,
        },
        selection: {
          ...selectionSlice.selection,
          ...initialState.selection,
          selectedIds: normalizeSelectedIds(initialState.selection?.selectedIds),
        },
      };
    }),
  );

export const AssetsStoreContext = createContext<AssetsStoreApi | null>(null);

export const useAssetsStoreApi = (): AssetsStoreApi => {
  const store = useContext(AssetsStoreContext);
  if (!store) {
    throw new Error("useAssetsStore must be used within an AssetsProvider");
  }
  return store;
};

export function useAssetsStore<T>(selector: (state: AssetsStore) => T): T {
  const store = useAssetsStoreApi();
  return useStore(store, selector);
}
