import { useCallback, useMemo } from "react";
import { useAssetsContext } from "./useAssetsContext";
import { SelectionResult } from "../types";
import {
  selectSelectionEnabled,
  selectSelectedIds,
  selectSelectedCount,
  selectIsAssetSelected,
  selectLastSelectedId,
  selectSelectionMode,
  selectHasSelection,
  selectSelectedAsArray,
} from "../reducers/selection.reducer";
import { useAssetActions } from "./useAssetActions";
import { albumService } from "@/services/albumService";
import { assetService } from "@/services/assetsService";

/**
 * Hook for managing asset selection state and operations.
 * Provides comprehensive selection functionality including single/multiple modes,
 * bulk operations, and selection persistence.
 *
 * @returns SelectionResult with selection state and operations
 */
export const useSelection = (): SelectionResult => {
  const { state, dispatch } = useAssetsContext();

  // Memoized selectors
  const enabled = useMemo(
    () => selectSelectionEnabled(state.selection),
    [state.selection],
  );

  const selectedIds = useMemo(
    () => selectSelectedIds(state.selection),
    [state.selection],
  );

  const selectedCount = useMemo(
    () => selectSelectedCount(state.selection),
    [state.selection],
  );

  const selectionMode = useMemo(
    () => selectSelectionMode(state.selection),
    [state.selection],
  );

  const hasSelection = useMemo(
    () => selectHasSelection(state.selection),
    [state.selection],
  );

  const selectedAsArray = useMemo(
    () => selectSelectedAsArray(state.selection),
    [state.selection],
  );

  // Selection operations
  const isSelected = useCallback(
    (assetId: string): boolean => {
      return selectIsAssetSelected(state.selection, assetId);
    },
    [state.selection],
  );

  const toggle = useCallback(
    (assetId: string): void => {
      if (!enabled) return;

      dispatch({
        type: "TOGGLE_ASSET_SELECTION",
        payload: { assetId },
      });
    },
    [enabled, dispatch],
  );

  const select = useCallback(
    (assetId: string): void => {
      if (!enabled) return;

      dispatch({
        type: "SELECT_ASSET",
        payload: { assetId },
      });
    },
    [enabled, dispatch],
  );

  const deselect = useCallback(
    (assetId: string): void => {
      if (!enabled) return;

      dispatch({
        type: "DESELECT_ASSET",
        payload: { assetId },
      });
    },
    [enabled, dispatch],
  );

  const selectAll = useCallback(
    (assetIds?: string[]): void => {
      if (!enabled) return;

      // If no specific asset IDs provided, we can't select all
      if (!assetIds || assetIds.length === 0) {
        console.warn("selectAll called without asset IDs");
        return;
      }

      dispatch({
        type: "SELECT_ALL",
        payload: { assetIds },
      });
    },
    [enabled, dispatch],
  );

  const clear = useCallback((): void => {
    dispatch({ type: "CLEAR_SELECTION" });
  }, [dispatch]);

  const setEnabled = useCallback(
    (newEnabled: boolean): void => {
      dispatch({
        type: "SET_SELECTION_ENABLED",
        payload: newEnabled,
      });
    },
    [dispatch],
  );

  const setSelectionMode = useCallback(
    (mode: "single" | "multiple"): void => {
      dispatch({
        type: "SET_SELECTION_MODE",
        payload: mode,
      });
    },
    [dispatch],
  );

  // Bulk operations helpers
  const selectRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]): void => {
      if (!enabled || selectionMode !== "multiple") return;

      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);

      if (fromIndex === -1 || toIndex === -1) return;

      const startIndex = Math.min(fromIndex, toIndex);
      const endIndex = Math.max(fromIndex, toIndex);
      const rangeIds = assetIds.slice(startIndex, endIndex + 1);

      rangeIds.forEach((assetId) => {
        if (!isSelected(assetId)) {
          select(assetId);
        }
      });
    },
    [enabled, selectionMode, isSelected, select],
  );

  const toggleRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]): void => {
      if (!enabled || selectionMode !== "multiple") return;

      const fromIndex = assetIds.indexOf(fromAssetId);
      const toIndex = assetIds.indexOf(toAssetId);

      if (fromIndex === -1 || toIndex === -1) return;

      const startIndex = Math.min(fromIndex, toIndex);
      const endIndex = Math.max(fromIndex, toIndex);
      const rangeIds = assetIds.slice(startIndex, endIndex + 1);

      // Check if all items in range are selected
      const allSelected = rangeIds.every((id) => isSelected(id));

      rangeIds.forEach((assetId) => {
        if (allSelected) {
          deselect(assetId);
        } else {
          select(assetId);
        }
      });
    },
    [enabled, selectionMode, isSelected, select, deselect],
  );

  const selectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      if (!enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(select);
    },
    [enabled, select],
  );

  const deselectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      if (!enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(deselect);
    },
    [enabled, deselect],
  );

  const invertSelection = useCallback(
    (assetIds: string[]): void => {
      if (!enabled) return;

      assetIds.forEach(toggle);
    },
    [enabled, toggle],
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
    clear,
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
  const { state } = useAssetsContext();

  const handleClick = useCallback(
    (assetId: string, event: MouseEvent | React.MouseEvent): void => {
      if (!selection.enabled) return;

      event.preventDefault();

      if (event.ctrlKey || event.metaKey) {
        // Ctrl/Cmd + Click: Toggle individual item
        selection.toggle(assetId);
      } else if (event.shiftKey && selection.selectedCount > 0) {
        // Shift + Click: Select range
        const lastSelected = selectLastSelectedId(state.selection);
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
    [selection, assetIds, state.selection],
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
  const { state } = useAssetsContext();

  return useMemo(
    () => ({
      enabled: selectSelectionEnabled(state.selection),
      selectedIds: selectSelectedIds(state.selection),
      selectedCount: selectSelectedCount(state.selection),
      selectionMode: selectSelectionMode(state.selection),
      hasSelection: selectHasSelection(state.selection),
      isSelected: (assetId: string) =>
        selectIsAssetSelected(state.selection, assetId),
    }),
    [state.selection],
  );
};

/**
 * Hook for bulk operations on selected assets.
 */
export const useBulkAssetOperations = () => {
  const selection = useSelection();
  const { toggleLike, deleteAsset, batchUpdateAssets } = useAssetActions();

  const bulkUpdateRating = useCallback(
    async (rating: number): Promise<void> => {
      const updates = Array.from(selection.selectedIds).map((assetId) => ({
        assetId,
        updates: {
          rating,
        },
      }));

      await batchUpdateAssets(updates);
    },
    [selection.selectedIds, batchUpdateAssets],
  );

  const bulkToggleLike = useCallback(async (): Promise<void> => {
    await Promise.all(
      Array.from(selection.selectedIds).map((assetId) => toggleLike(assetId)),
    );
  }, [selection.selectedIds, toggleLike]);

  const bulkDelete = useCallback(async (): Promise<void> => {
    await Promise.all(
      Array.from(selection.selectedIds).map((assetId) => deleteAsset(assetId)),
    );
    selection.clear();
  }, [selection.selectedIds, selection.clear, deleteAsset]);

  const bulkDownload = useCallback(async (): Promise<void> => {
    const ids = Array.from(selection.selectedIds);
    if (ids.length === 0) return;

    for (const id of ids) {
      try {
        const response = await assetService.getOriginalFile(id);
        const blob = response.data;
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        
        // Try to get filename from content-disposition or use a fallback
        const contentDisposition = response.headers['content-disposition'];
        let filename = `asset-${id}`;
        if (contentDisposition) {
          const filenameMatch = contentDisposition.match(/filename="?(.+)"?/i);
          if (filenameMatch) filename = filenameMatch[1];
        }
        
        link.setAttribute('download', filename);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
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
      ids.map(assetId => albumService.addAssetToAlbum(albumId, assetId, {}))
    );
  }, [selection.selectedIds]);

  return {
    bulkUpdateRating,
    bulkToggleLike,
    bulkDelete,
    bulkDownload,
    bulkAddToAlbum,
    selectedCount: selection.selectedCount,
    hasSelection: selection.selectedCount > 0,
  };
};
