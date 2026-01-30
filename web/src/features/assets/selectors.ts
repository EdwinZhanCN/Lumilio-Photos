import { useAssetsStore } from "./assets.store";
import { useShallow } from "zustand/react/shallow";
import { selectActiveFilterCount } from "./slices/filters.slice";
import { useSearchParams } from "react-router-dom";
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
export const useCurrentTab = () =>
    useAssetsStore((s) => s.ui.currentTab);

export const useGroupBy = (): GroupByType => {
    const [searchParams] = useSearchParams();
    const { state: settingsState } = useSettingsContext();
    return resolveGroupByFromUrl(
        searchParams.get("groupBy"),
        settingsState.ui.asset_page?.layout,
    );
};

export const useSearchQuery = () =>
    useAssetsStore((s) => s.ui.searchQuery);

export const useSearchMode = () =>
    useAssetsStore((s) => s.ui.searchMode);

export const useIsCarouselOpen = () =>
    useAssetsStore((s) => s.ui.isCarouselOpen);

export const useActiveAssetId = () =>
    useAssetsStore((s) => s.ui.activeAssetId);

// ===== Filter Selectors =====
export const useFiltersEnabled = () =>
    useAssetsStore((s) => s.filters.enabled);

export const useActiveFilterCount = () =>
    useAssetsStore((s) => selectActiveFilterCount(s.filters));

export const useFilterState = () =>
    useAssetsStore(useShallow((s) => s.filters));

// ===== Actions (stable references) =====
export const useSelectionActions = () =>
    useAssetsStore(useShallow((s) => ({
        toggle: s.toggleAssetSelection,
        select: s.selectAsset,
        deselect: s.deselectAsset,
        selectAll: s.selectAll,
        clear: s.clearSelection,
        setEnabled: s.setSelectionEnabled,
        setMode: s.setSelectionMode,
    })));

export const useUIActions = () => {
    const store = useAssetsStore(useShallow((s) => ({
        setTab: s.setCurrentTab,
        setSearchQuery: s.setSearchQuery,
        setSearchMode: s.setSearchMode,
    })));

    const [searchParams, setSearchParams] = useSearchParams();

    const setGroupBy = useCallback((groupBy: GroupByType) => {
        const params = new URLSearchParams(searchParams);
        params.set("groupBy", groupBy);
        setSearchParams(params, { replace: true });
    }, [searchParams, setSearchParams]);

    const setSearchQuery = useCallback((query: string) => {
        store.setSearchQuery(query);

        // Sync to URL
        const params = new URLSearchParams(searchParams);

        if (query.trim()) {
            params.set("q", query);
        } else {
            params.delete("q");
        }

        setSearchParams(params, { replace: true });
    }, [store.setSearchQuery, searchParams, setSearchParams]);

    return {
        ...store,
        setGroupBy,
        setSearchQuery,
    };
};

export const useFilterActions = () =>
    useAssetsStore(useShallow((s) => ({
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
    })));
