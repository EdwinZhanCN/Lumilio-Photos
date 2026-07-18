import { useCallback, useMemo } from "react";
import { useStore } from "zustand";
import {
  type AssetSelectionStoreApi,
  useAssetSelectionStore,
  useAssetSelectionStoreApi,
} from "./selection.store";

export interface AssetSelectionResult {
  enabled: boolean;
  selectedIds: Set<string>;
  selectedCount: number;
  selectionMode: "single" | "multiple";
  isSelected: (assetId: string) => boolean;
  toggle: (assetId: string) => void;
  select: (assetId: string) => void;
  deselect: (assetId: string) => void;
  selectAll: (assetIds?: string[]) => void;
  clear: () => void;
  setEnabled: (enabled: boolean) => void;
  setSelectionMode: (mode: "single" | "multiple") => void;
  selectRange: (fromAssetId: string, toAssetId: string, assetIds: string[]) => void;
  toggleRange: (fromAssetId: string, toAssetId: string, assetIds: string[]) => void;
  selectFiltered: (assetIds: string[], predicate: (assetId: string) => boolean) => void;
  deselectFiltered: (assetIds: string[], predicate: (assetId: string) => boolean) => void;
  invertSelection: (assetIds: string[]) => void;
  hasSelection: boolean;
  selectedAsArray: string[];
}

export function useAssetSelectionFromStore(store: AssetSelectionStoreApi): AssetSelectionResult {
  const enabled = useStore(store, (state) => state.selection.enabled);
  const selectedIds = useStore(store, (state) => state.selection.selectedIds);
  const selectionMode = useStore(store, (state) => state.selection.selectionMode);
  const toggleAssetSelection = useStore(store, (state) => state.toggleAssetSelection);
  const selectAsset = useStore(store, (state) => state.selectAsset);
  const deselectAsset = useStore(store, (state) => state.deselectAsset);
  const selectAllAssets = useStore(store, (state) => state.selectAll);
  const clear = useStore(store, (state) => state.clearSelection);
  const setEnabled = useStore(store, (state) => state.setSelectionEnabled);
  const setSelectionMode = useStore(store, (state) => state.setSelectionMode);
  const selectedCount = selectedIds.size;

  const isSelected = useCallback((assetId: string) => selectedIds.has(assetId), [selectedIds]);
  const toggle = useCallback(
    (assetId: string) => {
      if (enabled) toggleAssetSelection(assetId);
    },
    [enabled, toggleAssetSelection],
  );
  const select = useCallback(
    (assetId: string) => {
      if (enabled) selectAsset(assetId);
    },
    [enabled, selectAsset],
  );
  const deselect = useCallback(
    (assetId: string) => {
      if (enabled) deselectAsset(assetId);
    },
    [deselectAsset, enabled],
  );
  const selectAll = useCallback(
    (assetIds?: string[]) => {
      if (enabled && assetIds?.length) selectAllAssets(assetIds);
    },
    [enabled, selectAllAssets],
  );
  const selectRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]) => {
      const state = store.getState();
      if (!state.selection.enabled || state.selection.selectionMode !== "multiple") return;
      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);
      if (fromIndex === -1 || toIndex === -1) return;
      assetIds
        .slice(Math.min(fromIndex, toIndex), Math.max(fromIndex, toIndex) + 1)
        .forEach((assetId) => {
          if (!state.selection.selectedIds.has(assetId)) selectAsset(assetId);
        });
    },
    [selectAsset, store],
  );
  const toggleRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]) => {
      const state = store.getState();
      if (!state.selection.enabled || state.selection.selectionMode !== "multiple") return;
      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);
      if (fromIndex === -1 || toIndex === -1) return;
      const range = assetIds.slice(Math.min(fromIndex, toIndex), Math.max(fromIndex, toIndex) + 1);
      const allSelected = range.every((assetId) => state.selection.selectedIds.has(assetId));
      range.forEach(allSelected ? deselectAsset : selectAsset);
    },
    [deselectAsset, selectAsset, store],
  );
  const selectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean) => {
      if (!store.getState().selection.enabled) return;
      assetIds.filter(predicate).forEach(selectAsset);
    },
    [selectAsset, store],
  );
  const deselectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean) => {
      if (!store.getState().selection.enabled) return;
      assetIds.filter(predicate).forEach(deselectAsset);
    },
    [deselectAsset, store],
  );
  const invertSelection = useCallback(
    (assetIds: string[]) => {
      if (!store.getState().selection.enabled) return;
      assetIds.forEach(toggleAssetSelection);
    },
    [store, toggleAssetSelection],
  );
  const selectedAsArray = useMemo(() => Array.from(selectedIds), [selectedIds]);

  return {
    enabled,
    selectedIds,
    selectedCount,
    selectionMode,
    isSelected,
    toggle,
    select,
    deselect,
    selectAll,
    clear,
    setEnabled,
    setSelectionMode,
    selectRange,
    toggleRange,
    selectFiltered,
    deselectFiltered,
    invertSelection,
    hasSelection: selectedCount > 0,
    selectedAsArray,
  };
}

export function useAssetSelection(): AssetSelectionResult {
  return useAssetSelectionFromStore(useAssetSelectionStoreApi());
}

export function useAssetSelectionActions() {
  const clear = useAssetSelectionStore((state) => state.clearSelection);
  const setEnabled = useAssetSelectionStore((state) => state.setSelectionEnabled);
  const setMode = useAssetSelectionStore((state) => state.setSelectionMode);
  return useMemo(() => ({ clear, setEnabled, setMode }), [clear, setEnabled, setMode]);
}
