import { useCallback, useMemo } from "react";
import { useAssetsStore } from "../assets.store";
import { SelectionResult } from "@/features/assets";
import {
  useSelectionEnabled,
  useSelectedIds,
  useSelectionMode,
  useSelectionActions,
} from "../selectors";
import {
  selectLastSelectedId,
} from "../slices/selection.slice";
import { useAssetActions } from "./useAssetActions";
import { assetUrls } from "@/lib/assets/assetUrls";
import { $api } from "@/lib/http-commons/queryClient";
import { Asset } from "@/lib/assets/types";

/**
 * Hook for managing asset selection state and operations.
 * Provides comprehensive selection functionality including single/multiple modes,
 * bulk operations, and selection persistence.
 *
 * @returns SelectionResult with selection state and operations
 */
export const useSelection = (): SelectionResult => {
  // Fine-grained subscriptions
  const enabled = useSelectionEnabled();
  const selectedIds = useSelectedIds();
  const selectionMode = useSelectionMode();

  // Actions
  const {
    toggle: toggleAssetSelection,
    select: selectAsset,
    deselect: deselectAsset,
    selectAll: selectAllAssets,
    clear: clearSelection,
    setEnabled,
    setMode: setSelectionMode,
  } = useSelectionActions();

  // Derived values
  const selectedCount = selectedIds.size;
  const hasSelection = selectedCount > 0;

  const selectedAsArray = useMemo(
    () => Array.from(selectedIds),
    [selectedIds],
  );

  // Selection operations
  const isSelected = useCallback(
    (assetId: string): boolean => {
      // Direct access from the subscribed state in this component
      return selectedIds.has(assetId);
    },
    [selectedIds],
  );

  // Wrappers for store actions
  const toggle = useCallback(
    (assetId: string): void => {
      if (!enabled) return;
      toggleAssetSelection(assetId);
    },
    [enabled, toggleAssetSelection],
  );

  const select = useCallback(
    (assetId: string): void => {
      if (!enabled) return;
      selectAsset(assetId);
    },
    [enabled, selectAsset],
  );

  const deselect = useCallback(
    (assetId: string): void => {
      if (!enabled) return;
      deselectAsset(assetId);
    },
    [enabled, deselectAsset],
  );

  const selectAll = useCallback(
    (assetIds?: string[]): void => {
      if (!enabled) return;

      // If no specific asset IDs provided, we can't select all
      if (!assetIds || assetIds.length === 0) {
        console.warn("selectAll called without asset IDs");
        return;
      }

      selectAllAssets(assetIds);
    },
    [enabled, selectAllAssets],
  );

  // Bulk operations helpers - using getState to avoid stale closures and re-renders
  const selectRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]): void => {
      // Check current state from store
      const state = useAssetsStore.getState();
      if (!state.selection.enabled || state.selection.selectionMode !== "multiple") return;

      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);

      if (fromIndex === -1 || toIndex === -1) return;

      const startIndex = Math.min(fromIndex, toIndex);
      const endIndex = Math.max(fromIndex, toIndex);
      const rangeIds = assetIds.slice(startIndex, endIndex + 1);

      rangeIds.forEach((assetId) => {
        if (!state.selection.selectedIds.has(assetId)) {
          selectAsset(assetId);
        }
      });
    },
    [selectAsset],
  );

  const toggleRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]): void => {
      const state = useAssetsStore.getState();
      if (!state.selection.enabled || state.selection.selectionMode !== "multiple") return;

      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);

      if (fromIndex === -1 || toIndex === -1) return;

      const startIndex = Math.min(fromIndex, toIndex);
      const endIndex = Math.max(fromIndex, toIndex);
      const rangeIds = assetIds.slice(startIndex, endIndex + 1);

      // Check if all items in range are selected based on current state
      const allSelected = rangeIds.every((id) => state.selection.selectedIds.has(id));

      rangeIds.forEach((assetId) => {
        if (allSelected) {
          deselectAsset(assetId);
        } else {
          selectAsset(assetId);
        }
      });
    },
    [selectAsset, deselectAsset],
  );

  const selectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      const state = useAssetsStore.getState();
      if (!state.selection.enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(selectAsset);
    },
    [selectAsset],
  );

  const deselectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      const state = useAssetsStore.getState();
      if (!state.selection.enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(deselectAsset);
    },
    [deselectAsset],
  );

  const invertSelection = useCallback(
    (assetIds: string[]): void => {
      const state = useAssetsStore.getState();
      if (!state.selection.enabled) return;

      assetIds.forEach(toggleAssetSelection);
    },
    [toggleAssetSelection],
  );

  return {
    // State
    enabled,
    selectedIds,
    selectedCount,
    selectionMode,

    // Basic operations
    isSelected,
    toggle,
    select,
    deselect,
    selectAll,
    clear: clearSelection,
    setEnabled,
    setSelectionMode,

    // Extended operations
    selectRange,
    toggleRange,
    selectFiltered,
    deselectFiltered,
    invertSelection,

    // Computed properties
    hasSelection,
    selectedAsArray,
  };
};

/**
 * Hook for keyboard-enhanced selection operations.
 */
export const useKeyboardSelection = (assetIds: string[]) => {
  const selection = useSelection();
  // We need to access the store state directly for event handlers

  const handleClick = useCallback(
    (assetId: string, event: MouseEvent | React.MouseEvent): void => {
      if (!selection.enabled) return;

      event.preventDefault();

      if (event.ctrlKey || event.metaKey) {
        // Ctrl/Cmd + Click: Toggle individual item
        selection.toggle(assetId);
      } else if (event.shiftKey && selection.selectedCount > 0) {
        // Shift + Click: Select range
        const state = useAssetsStore.getState();
        const lastSelected = selectLastSelectedId(state.selection);
        // Or simpler:
        // const lastSelected = state.selection.lastSelectedId;
        if (lastSelected) {
          selection.selectRange(lastSelected, assetId, assetIds);
        } else {
          selection.select(assetId);
        }
      } else {
        // Regular click: Select only this item (clear others first if multiple mode)
        if (selection.selectionMode === "multiple") {
          selection.clear();
        }
        selection.select(assetId);
      }
    },
    [selection, assetIds],
  );

  const handleKeyDown = useCallback(
    (event: KeyboardEvent | React.KeyboardEvent): void => {
      if (!selection.enabled) return;

      switch (event.key) {
        case "Escape":
          event.preventDefault();
          selection.clear();
          break;
        case "a":
        case "A":
          if (event.ctrlKey || event.metaKey) {
            event.preventDefault();
            selection.selectAll(assetIds);
          }
          break;
        case "Delete":
        case "Backspace":
          event.preventDefault();
          break;
      }
    },
    [selection, assetIds],
  );

  return {
    ...selection,
    handleClick,
    handleKeyDown,
  };
};

/**
 * Hook for selection state without operations.
 */
export const useSelectionState = () => {
  // Replaced context usage with direct store access
  const enabled = useSelectionEnabled();
  const selectedIds = useSelectedIds();
  const selectedCount = selectedIds.size;
  const selectionMode = useSelectionMode();

  return useMemo(
    () => ({
      enabled,
      selectedIds,
      selectedCount,
      selectionMode,
      hasSelection: selectedCount > 0,
      isSelected: (assetId: string) => selectedIds.has(assetId),
    }),
    [enabled, selectedIds, selectedCount, selectionMode],
  );
};

/**
 * Hook for bulk operations on selected assets.
 */
export const useBulkAssetOperations = () => {
  const selection = useSelection();
  const { deleteAsset, batchUpdateAssets } = useAssetActions();
  const { mutateAsync: addAssetToAlbum } = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/assets/{assetId}",
  );

  const bulkUpdateRating = useCallback(
    async (rating: number): Promise<void> => {
      const updates = Array.from(selection.selectedIds).map((assetId) => ({
        assetId,
        updates: {
          rating,
        },
      }));

      await batchUpdateAssets(updates);
    }, [selection.selectedIds, batchUpdateAssets]);

  const bulkSetLike = useCallback(async (liked: boolean): Promise<void> => {
    const updates = Array.from(selection.selectedIds).map((assetId) => ({
      assetId,
      updates: {
        liked,
      },
    }));

    await batchUpdateAssets(updates);
  }, [selection.selectedIds, batchUpdateAssets]);

  const bulkDelete = useCallback(async (): Promise<void> => {
    await Promise.all(
      Array.from(selection.selectedIds).map((assetId) => deleteAsset(assetId)),
    );
    selection.clear();
  }, [selection.selectedIds, selection.clear, deleteAsset]);

  const bulkDownload = useCallback(async (assets?: Asset[]): Promise<void> => {
    const ids = Array.from(selection.selectedIds);
    if (ids.length === 0) return;

    for (const id of ids) {
      try {
        const url = assetUrls.getOriginalFileUrl(id as string);
        const response = await fetch(url);
        const blob = await response.blob();
        const blobUrl = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = blobUrl;

        // Try to get filename from asset object first, then content-disposition, then fallback
        let filename = `asset-${id}`;

        // 1. Try asset object
        if (assets) {
          const asset = assets.find(a => a.asset_id === id);
          if (asset?.original_filename) {
            filename = asset.original_filename;
          }
        }

        // 2. Try content-disposition if we didn't find it
        if (filename === `asset-${id}`) {
          const contentDisposition = response.headers.get('content-disposition');
          if (contentDisposition) {
            const filenameMatch = contentDisposition.match(/filename="?(.+)"?/i);
            if (filenameMatch) filename = filenameMatch[1];
          }
        }

        link.setAttribute('download', filename);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(blobUrl);
      } catch (error) {
        console.error(`Failed to download asset ${id}:`, error);
      }
      // Small delay to prevent browser from blocking multiple downloads
      await new Promise(resolve => setTimeout(resolve, 300));
    }
  }, [selection.selectedIds]);

  const bulkAddToAlbum = useCallback(async (albumId: number): Promise<void> => {
    const ids = Array.from(selection.selectedIds);
    await Promise.all(
      ids.map((assetId) =>
        addAssetToAlbum({
          params: { path: { id: albumId, assetId: assetId as string } },
          body: {},
        }),
      ),
    );
  }, [selection.selectedIds, addAssetToAlbum]);

  return {
    bulkUpdateRating,
    bulkSetLike,
    bulkDelete,
    bulkDownload: (assets?: Asset[]) => bulkDownload(assets),
    bulkAddToAlbum,
    selectedCount: selection.selectedCount,
    hasSelection: selection.selectedCount > 0,
  };
};
