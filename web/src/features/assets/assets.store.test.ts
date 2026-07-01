import { describe, expect, it } from "vite-plus/test";
import { createAssetsStore } from "./assets.store";

describe("createAssetsStore", () => {
  it("keeps filters, search, sort, and selection isolated per store", () => {
    const mainStore = createAssetsStore();
    const pickerStore = createAssetsStore({
      selection: {
        selectionMode: "single",
      },
    });

    mainStore.getState().batchUpdateFilters({
      enabled: true,
      raw: false,
    });
    mainStore.getState().setSearchQuery("raw exclude");
    mainStore.getState().setSortBy("recently_added");
    mainStore.getState().setSelectionEnabled(true);
    mainStore.getState().selectAsset("asset-main");

    pickerStore.getState().batchUpdateFilters({
      enabled: true,
      liked: true,
    });
    pickerStore.getState().setSearchQuery("cover");
    pickerStore.getState().setSortBy("date_captured");
    pickerStore.getState().setSelectionEnabled(true);
    pickerStore.getState().selectAsset("asset-picker");

    expect(mainStore.getState().filters).toMatchObject({
      enabled: true,
      raw: false,
      liked: undefined,
    });
    expect(mainStore.getState().ui.searchQuery).toBe("raw exclude");
    expect(mainStore.getState().ui.sortBy).toBe("recently_added");
    expect(mainStore.getState().selection.selectionMode).toBe("multiple");
    expect(mainStore.getState().selection.selectedIds.has("asset-main")).toBe(true);
    expect(mainStore.getState().selection.selectedIds.has("asset-picker")).toBe(false);

    expect(pickerStore.getState().filters).toMatchObject({
      enabled: true,
      liked: true,
      raw: undefined,
    });
    expect(pickerStore.getState().ui.searchQuery).toBe("cover");
    expect(pickerStore.getState().ui.sortBy).toBe("date_captured");
    expect(pickerStore.getState().selection.selectionMode).toBe("single");
    expect(pickerStore.getState().selection.selectedIds.has("asset-picker")).toBe(true);
    expect(pickerStore.getState().selection.selectedIds.has("asset-main")).toBe(false);
  });
});
