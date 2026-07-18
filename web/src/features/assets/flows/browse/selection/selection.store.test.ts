import { describe, expect, it } from "vite-plus/test";
import { createAssetSelectionStore } from "./selection.store";

describe("createAssetSelectionStore", () => {
  it("keeps selection isolated per browser scope", () => {
    const mainStore = createAssetSelectionStore();
    const pickerStore = createAssetSelectionStore({ selectionMode: "single" });

    mainStore.getState().setSelectionEnabled(true);
    mainStore.getState().selectAsset("asset-main");

    pickerStore.getState().setSelectionEnabled(true);
    pickerStore.getState().selectAsset("asset-picker");

    expect(mainStore.getState().selection.selectionMode).toBe("multiple");
    expect(mainStore.getState().selection.selectedIds.has("asset-main")).toBe(true);
    expect(mainStore.getState().selection.selectedIds.has("asset-picker")).toBe(false);

    expect(pickerStore.getState().selection.selectionMode).toBe("single");
    expect(pickerStore.getState().selection.selectedIds.has("asset-picker")).toBe(true);
    expect(pickerStore.getState().selection.selectedIds.has("asset-main")).toBe(false);
  });
});
