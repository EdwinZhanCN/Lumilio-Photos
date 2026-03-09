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

  const updateSearchParams = useCallback(
    (updater: (params: URLSearchParams) => void) => {
      if (typeof window === "undefined") return;

      const url = new URL(window.location.href);
      const params = new URLSearchParams(url.search);
      updater(params);

      const nextSearch = params.toString();
      const nextUrl = `${url.pathname}${nextSearch ? `?${nextSearch}` : ""}${url.hash}`;
      window.history.replaceState(window.history.state, "", nextUrl);
    },
    [],
  );

  const setGroupBy = useCallback(
    (groupBy: GroupByType) => {
      store.setGroupByState(groupBy);
      updateSearchParams((params) => {
        params.set("groupBy", groupBy);
      });
    },
    [store, updateSearchParams],
  );

  const setSearchQuery = useCallback(
    (query: string) => {
      const normalizedQuery = query.trim();
      store.setSearchQueryState(normalizedQuery);
      updateSearchParams((params) => {
        if (normalizedQuery) {
          params.set("q", normalizedQuery);
        } else {
          params.delete("q");
        }
      });
    },
    [store, updateSearchParams],
  );

  const applySearch = useCallback(
    (query: string) => {
      const normalizedQuery = query.trim();
      store.setSearchQueryState(normalizedQuery);
      if (normalizedQuery) {
        store.setGroupByState("flat");
      }

      updateSearchParams((params) => {
        if (normalizedQuery) {
          params.set("q", normalizedQuery);
          params.set("groupBy", "flat");
        } else {
          params.delete("q");
        }
      });
    },
    [store, updateSearchParams],
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
      setCameraMake: s.setFilterCameraMake,
      setLens: s.setFilterLens,
      resetFilters: s.resetFilters,
      batchUpdateFilters: s.batchUpdateFilters,
    })),
  );
