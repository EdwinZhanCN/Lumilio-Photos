import { useCallback, useContext, useMemo, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useStore } from "zustand";
import {
  AssetsStoreApi,
  AssetsStoreContext,
  createAssetsStore,
  useAssetsStoreApi,
} from "../assets.store";
import type { SelectionResult } from "../types/assets.type";
import { useSelectionEnabled, useSelectedIds, useSelectionMode } from "../selectors";
import { selectLastSelectedId } from "../slices/selection.slice";
import { useAssetActions } from "./useAssetActions";
import { assetUrls } from "@/lib/assets/assetUrls";
import { getToken } from "@/lib/http-commons/auth";
import { $api } from "@/lib/http-commons/queryClient";
import { Asset } from "@/lib/assets/types";

const useFallbackAssetsStore = (): AssetsStoreApi => {
  const storeRef = useRef<AssetsStoreApi | null>(null);
  if (storeRef.current === null) {
    storeRef.current = createAssetsStore({
      selection: {
        enabled: false,
      },
    });
  }
  return storeRef.current;
};

/**
 * Hook for managing asset selection state and operations.
 * Provides comprehensive selection functionality including single/multiple modes,
 * bulk operations, and selection persistence.
 *
 * @returns SelectionResult with selection state and operations
 */
const useSelectionFromStore = (store: AssetsStoreApi): SelectionResult => {
  // Fine-grained subscriptions
  const enabled = useStore(store, (state) => state.selection.enabled);
  const selectedIds = useStore(store, (state) => state.selection.selectedIds);
  const selectionMode = useStore(store, (state) => state.selection.selectionMode);

  // Actions
  const toggleAssetSelection = useStore(store, (state) => state.toggleAssetSelection);
  const selectAsset = useStore(store, (state) => state.selectAsset);
  const deselectAsset = useStore(store, (state) => state.deselectAsset);
  const selectAllAssets = useStore(store, (state) => state.selectAll);
  const clearSelection = useStore(store, (state) => state.clearSelection);
  const setEnabled = useStore(store, (state) => state.setSelectionEnabled);
  const setSelectionMode = useStore(store, (state) => state.setSelectionMode);

  // Derived values
  const selectedCount = selectedIds.size;
  const hasSelection = selectedCount > 0;

  const selectedAsArray = useMemo(() => Array.from(selectedIds), [selectedIds]);

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
      const state = store.getState();
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
    [selectAsset, store],
  );

  const toggleRange = useCallback(
    (fromAssetId: string, toAssetId: string, assetIds: string[]): void => {
      const state = store.getState();
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
    [selectAsset, deselectAsset, store],
  );

  const selectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      const state = store.getState();
      if (!state.selection.enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(selectAsset);
    },
    [selectAsset, store],
  );

  const deselectFiltered = useCallback(
    (assetIds: string[], predicate: (assetId: string) => boolean): void => {
      const state = store.getState();
      if (!state.selection.enabled) return;

      const filteredIds = assetIds.filter(predicate);
      filteredIds.forEach(deselectAsset);
    },
    [deselectAsset, store],
  );

  const invertSelection = useCallback(
    (assetIds: string[]): void => {
      const state = store.getState();
      if (!state.selection.enabled) return;

      assetIds.forEach(toggleAssetSelection);
    },
    [toggleAssetSelection, store],
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

export const useSelection = (): SelectionResult => {
  const store = useAssetsStoreApi();
  return useSelectionFromStore(store);
};

/**
 * Hook for keyboard-enhanced selection operations.
 */
const useKeyboardSelectionFromStore = (store: AssetsStoreApi, assetIds: string[]) => {
  const selection = useSelectionFromStore(store);
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
        const state = store.getState();
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
    [selection, assetIds, store],
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

export const useKeyboardSelection = (assetIds: string[]) => {
  const store = useAssetsStoreApi();
  return useKeyboardSelectionFromStore(store, assetIds);
};

export const useOptionalKeyboardSelection = (assetIds: string[]) => {
  const contextStore = useContext(AssetsStoreContext);
  const fallbackStore = useFallbackAssetsStore();
  return useKeyboardSelectionFromStore(contextStore ?? fallbackStore, assetIds);
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

const triggerDownload = (blob: Blob, filename: string): void => {
  const blobUrl = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = blobUrl;
  link.setAttribute("download", filename);
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(blobUrl);
};

const filenameFromContentDisposition = (contentDisposition: string | null): string | undefined => {
  if (!contentDisposition) return undefined;
  const utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1]);
  }
  const filenameMatch = contentDisposition.match(/filename="?([^"]+)"?/i);
  return filenameMatch?.[1];
};

/**
 * Hook for bulk operations on selected assets.
 */
export const useBulkAssetOperations = (resolvedAssetIds?: string[]) => {
  const selection = useSelection();
  const queryClient = useQueryClient();
  const { deleteAsset, batchUpdateAssets } = useAssetActions();
  const { mutateAsync: addAssetToAlbum } = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/assets/{assetId}",
  );
  const { mutateAsync: addAssetTag } = $api.useMutation("post", "/api/v1/assets/{id}/tags");

  const bulkUpdateRating = useCallback(
    async (rating: number): Promise<void> => {
      const targetIds = resolvedAssetIds ?? Array.from(selection.selectedIds);
      const updates = targetIds.map((assetId) => ({
        assetId,
        updates: {
          rating,
        },
      }));

      await batchUpdateAssets(updates);
    },
    [resolvedAssetIds, selection.selectedIds, batchUpdateAssets],
  );

  const bulkSetLike = useCallback(
    async (liked: boolean): Promise<void> => {
      const targetIds = resolvedAssetIds ?? Array.from(selection.selectedIds);
      const updates = targetIds.map((assetId) => ({
        assetId,
        updates: {
          liked,
        },
      }));

      await batchUpdateAssets(updates);
    },
    [resolvedAssetIds, selection.selectedIds, batchUpdateAssets],
  );

  const bulkDelete = useCallback(async (): Promise<void> => {
    const targetIds = resolvedAssetIds ?? Array.from(selection.selectedIds);
    await Promise.all(targetIds.map((assetId) => deleteAsset(assetId)));
    selection.clear();
  }, [resolvedAssetIds, selection.selectedIds, selection.clear, deleteAsset]);

  const bulkDownload = useCallback(
    async (assets?: Asset[]): Promise<void> => {
      const ids = resolvedAssetIds ?? Array.from(selection.selectedIds);
      if (ids.length === 0) return;

      if (ids.length > 10) {
        const headers = new Headers({
          "Content-Type": "application/json",
        });
        const token = getToken();
        if (token) {
          headers.set("Authorization", `Bearer ${token}`);
        }

        const response = await fetch(assetUrls.getBulkDownloadUrl(), {
          method: "POST",
          headers,
          body: JSON.stringify({ asset_ids: ids }),
        });
        if (!response.ok) {
          throw new Error(`Bulk download failed with ${response.status}`);
        }

        const blob = await response.blob();
        const filename =
          filenameFromContentDisposition(response.headers.get("content-disposition")) ??
          "lumilio-assets.zip";
        triggerDownload(blob, filename);
        return;
      }

      for (const id of ids) {
        try {
          const url = assetUrls.getOriginalFileUrl(id as string);
          const response = await fetch(url);
          const blob = await response.blob();

          // Try to get filename from asset object first, then content-disposition, then fallback
          let filename = `asset-${id}`;

          // 1. Try asset object
          if (assets) {
            const asset = assets.find((a) => a.asset_id === id);
            if (asset?.original_filename) {
              filename = asset.original_filename;
            }
          }

          // 2. Try content-disposition if we didn't find it
          if (filename === `asset-${id}`) {
            filename =
              filenameFromContentDisposition(response.headers.get("content-disposition")) ??
              filename;
          }

          triggerDownload(blob, filename);
        } catch (error) {
          console.error(`Failed to download asset ${id}:`, error);
        }
        // Small delay to prevent browser from blocking multiple downloads
        await new Promise((resolve) => setTimeout(resolve, 300));
      }
    },
    [resolvedAssetIds, selection.selectedIds],
  );

  const bulkAddToAlbum = useCallback(
    async (albumId: number): Promise<void> => {
      const ids = resolvedAssetIds ?? Array.from(selection.selectedIds);
      await Promise.all(
        ids.map((assetId) =>
          addAssetToAlbum({
            params: { path: { id: albumId, assetId: assetId as string } },
            body: {},
          }),
        ),
      );
    },
    [resolvedAssetIds, selection.selectedIds, addAssetToAlbum],
  );

  const bulkAddTags = useCallback(
    async (tagNames: string[]): Promise<void> => {
      const names = [
        ...new Set(tagNames.map((name) => name.trim()).filter((name) => name.length > 0)),
      ];
      if (names.length === 0) return;

      const ids = resolvedAssetIds ?? Array.from(selection.selectedIds);
      await Promise.all(
        ids.flatMap((assetId) =>
          names.map((tagName) =>
            addAssetTag({
              params: { path: { id: assetId } },
              body: { tag_name: tagName },
            }),
          ),
        ),
      );

      await Promise.all([
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/{id}/tags"],
        }),
        queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/assets/tags"],
        }),
        queryClient.invalidateQueries({
          predicate: (query) => {
            const path = query.queryKey[1];
            return path === "/api/v1/assets/list" || path === "/api/v1/assets/search";
          },
        }),
      ]);
    },
    [resolvedAssetIds, selection.selectedIds, addAssetTag, queryClient],
  );

  return {
    bulkUpdateRating,
    bulkSetLike,
    bulkDelete,
    bulkDownload: (assets?: Asset[]) => bulkDownload(assets),
    bulkAddToAlbum,
    bulkAddTags,
    selectedCount: selection.selectedCount,
    hasSelection: selection.selectedCount > 0,
  };
};
