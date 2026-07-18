import { useCallback } from "react";
import { useAssetSelectionFromStore } from "./useAssetSelection";
import { selectLastSelectedId, useAssetSelectionStoreApi } from "./selection.store";

export function useSelectionKeyboard(assetIds: string[]) {
  const store = useAssetSelectionStoreApi();
  const selection = useAssetSelectionFromStore(store);

  const handleClick = useCallback(
    (assetId: string, event: MouseEvent | React.MouseEvent) => {
      if (!selection.enabled) return;
      event.preventDefault();
      if (event.ctrlKey || event.metaKey) {
        selection.toggle(assetId);
        return;
      }
      if (event.shiftKey && selection.selectedCount > 0) {
        const lastSelected = selectLastSelectedId(store.getState());
        if (lastSelected) selection.selectRange(lastSelected, assetId, assetIds);
        else selection.select(assetId);
        return;
      }
      if (selection.selectionMode === "multiple") selection.clear();
      selection.select(assetId);
    },
    [assetIds, selection, store],
  );

  const handleKeyDown = useCallback(
    (event: KeyboardEvent | React.KeyboardEvent) => {
      if (!selection.enabled) return;
      if (event.key === "Escape") {
        event.preventDefault();
        selection.clear();
      } else if ((event.key === "a" || event.key === "A") && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        selection.selectAll(assetIds);
      } else if (event.key === "Delete" || event.key === "Backspace") {
        event.preventDefault();
      }
    },
    [assetIds, selection],
  );

  return { ...selection, handleClick, handleKeyDown };
}
