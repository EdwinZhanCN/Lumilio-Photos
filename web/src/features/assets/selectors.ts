import { useAssetsStore } from "./assets.store";
import { useShallow } from "zustand/react/shallow";
import { selectActiveFilterCount } from "./slices/filters.slice";
import { useSettingsContext } from "@/features/settings";
import { useCallback } from "react";
import { GroupByType } from "./types/assets.type";
import { resolveGroupByFromUrl } from "./utils/groupBy";

// ===== Selection Selectors =====
export const useSelectionEnabled = () =>
  useAssetsStore((s) => s.selection.enabled);

export const useSelectedIds = () =>
  useAssetsStore((s) => s.selection.selectedIds);

export const useSelectedCount = () =>
  useAssetsStore((s) => s.selection.selectedIds.size);

export const useIsAssetSelected = (assetId: string) =>
  useAssetsStore((s) => s.selection.selectedIds.has(assetId));

export const useSelectionMode = () =>
  useAssetsStore((s) => s.selection.selectionMode);

// ===== UI Selectors =====
export const useCurrentTab = () => useAssetsStore((s) => s.ui.currentTab);

export const useGroupBy = (): GroupByType => {
  const { state: settingsState } = useSettingsContext();
  const groupBy = useAssetsStore((s) => s.ui.groupBy);
  return resolveGroupByFromUrl(groupBy, settingsState.ui.asset_page?.layout);
};

export const useSearchQuery = () => useAssetsStore((s) => s.ui.searchQuery);

export const useIsCarouselOpen = () =>
  useAssetsStore((s) => s.ui.isCarouselOpen);

export const useActiveAssetId = () => useAssetsStore((s) => s.ui.activeAssetId);

// ===== Filter Selectors =====
export const useFiltersEnabled = () => useAssetsStore((s) => s.filters.enabled);

export const useActiveFilterCount = () =>
  useAssetsStore((s) => selectActiveFilterCount(s.filters));

export const useFilterState = () =>
  useAssetsStore(useShallow((s) => s.filters));

// ===== Actions (stable references) =====
export const useSelectionActions = () =>
  useAssetsStore(
    useShallow((s) => ({
      toggle: s.toggleAssetSelection,
      select: s.selectAsset,
      deselect: s.deselectAsset,
      selectAll: s.selectAll,
      clear: s.clearSelection,
      setEnabled: s.setSelectionEnabled,
      setMode: s.setSelectionMode,
    })),
  );

export const useUIActions = () => {
  const store = useAssetsStore(
    useShallow((s) => ({
      setTab: s.setCurrentTab,
      setSearchQueryState: s.setSearchQuery,
      setGroupByState: s.setGroupBy,
    })),
  );

  const setGroupBy = useCallback(
    (groupBy: GroupByType) => {
      store.setGroupByState(groupBy);
    },
    [store],
  );

  const setSearchQuery = useCallback(
    (query: string) => {
      const normalizedQuery = query.trim();
      store.setSearchQueryState(normalizedQuery);
    },
    [store],
  );

  const applySearch = useCallback(
    (query: string) => {
      const normalizedQuery = query.trim();
      store.setSearchQueryState(normalizedQuery);
    },
    [store],
  );

  return {
    ...store,
    setGroupBy,
    setSearchQuery,
    applySearch,
  };
};

export const useFilterActions = () =>
  useAssetsStore(
    useShallow((s) => ({
      setFiltersEnabled: s.setFiltersEnabled,
      setRaw: s.setFilterRaw,
      setRating: s.setFilterRating,
      setLiked: s.setFilterLiked,
      setFilename: s.setFilterFilename,
      setDate: s.setFilterDate,
      setCameraModel: s.setFilterCameraModel,
      setLens: s.setFilterLens,
      setLocation: s.setFilterLocation,
      resetFilters: s.resetFilters,
      batchUpdateFilters: s.batchUpdateFilters,
    })),
  );
