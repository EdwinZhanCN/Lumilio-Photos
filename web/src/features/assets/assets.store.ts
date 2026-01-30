import { create } from "zustand";
import { immer } from "zustand/middleware/immer";
import { enableMapSet } from "immer";
import { createUISlice, UISlice } from "./slices/ui.slice";
import { createFiltersSlice, FiltersSlice } from "./slices/filters.slice";
import {
  createSelectionSlice,
  SelectionSlice,
} from "./slices/selection.slice";

// Enable Map and Set support in Immer
enableMapSet();

export type AssetsStore =
  UISlice &
  FiltersSlice &
  SelectionSlice;

export const useAssetsStore = create<AssetsStore>()(
  immer((set, get, store) => ({
    ...createUISlice(set as any, get as any, store as any),
    ...createFiltersSlice(set as any, get as any, store as any),
    ...createSelectionSlice(set as any, get as any, store as any),
  })),
);
