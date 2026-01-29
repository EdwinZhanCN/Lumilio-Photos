import { useAssetsStore } from "./assets.store";
import { useShallow } from "zustand/react/shallow";

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

export const useGroupBy = () =>
    useAssetsStore((s) => s.ui.groupBy);

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
    useAssetsStore((s) => {
        if (!s.filters.enabled) return 0;
        let count = 0;
        if (s.filters.raw !== undefined) count++;
        if (s.filters.rating !== undefined) count++;
        if (s.filters.liked !== undefined) count++;
        if (s.filters.filename && s.filters.filename.value.trim()) count++;
        if (s.filters.date && (s.filters.date.from || s.filters.date.to)) count++;
        if (s.filters.camera_make && s.filters.camera_make.trim()) count++;
        if (s.filters.lens && s.filters.lens.trim()) count++;
        return count;
    });

export const useFilterState = () =>
    useAssetsStore(useShallow((s) => s.filters));

// ===== Entity Selectors =====
export const useAsset = (assetId: string) =>
    useAssetsStore((s) => s.entities.assets[assetId]);

export const useAssetMeta = (assetId: string) =>
    useAssetsStore((s) => s.entities.meta[assetId]);

export const useAllAssets = () =>
    useAssetsStore((s) => s.entities.assets);

// ===== View Selectors =====
export const useView = (viewKey: string) =>
    useAssetsStore((s) => s.views[viewKey]);

const EMPTY_ARRAY: string[] = [];

export const useViewAssetIds = (viewKey: string) =>
    useAssetsStore((s) => s.views[viewKey]?.assetIds || EMPTY_ARRAY);

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
        selectRange: PLACEHOLDER_SELECT_RANGE,
    })));

const PLACEHOLDER_SELECT_RANGE = () => console.warn('selectRange not implemented in store yet');

export const useUIActions = () =>
    useAssetsStore(useShallow((s) => ({
        setTab: s.setCurrentTab,
        setGroupBy: s.setGroupBy,
        setSearchQuery: s.setSearchQuery,
        setSearchMode: s.setSearchMode,
    })));

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
