import { describe, expect, it } from "vitest";
import { createAssetsStore } from "./assets.store";

describe("createAssetsStore", () => {
  it("keeps filters, search, group, and selection isolated per store", () => {
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
    mainStore.getState().setGroupBy("flat");
    mainStore.getState().setSelectionEnabled(true);
    mainStore.getState().selectAsset("asset-main");

    pickerStore.getState().batchUpdateFilters({
      enabled: true,
      liked: true,
    });
    pickerStore.getState().setSearchQuery("cover");
    pickerStore.getState().setGroupBy("date");
    pickerStore.getState().setSelectionEnabled(true);
    pickerStore.getState().selectAsset("asset-picker");

    expect(mainStore.getState().filters).toMatchObject({
      enabled: true,
      raw: false,
      liked: undefined,
    });
    expect(mainStore.getState().ui.searchQuery).toBe("raw exclude");
    expect(mainStore.getState().ui.groupBy).toBe("flat");
    expect(mainStore.getState().selection.selectionMode).toBe("multiple");
    expect(mainStore.getState().selection.selectedIds.has("asset-main")).toBe(
      true,
    );
    expect(mainStore.getState().selection.selectedIds.has("asset-picker")).toBe(
      false,
    );

    expect(pickerStore.getState().filters).toMatchObject({
      enabled: true,
      liked: true,
      raw: undefined,
    });
    expect(pickerStore.getState().ui.searchQuery).toBe("cover");
    expect(pickerStore.getState().ui.groupBy).toBe("date");
    expect(pickerStore.getState().selection.selectionMode).toBe("single");
    expect(pickerStore.getState().selection.selectedIds.has("asset-picker")).toBe(
      true,
    );
    expect(pickerStore.getState().selection.selectedIds.has("asset-main")).toBe(
      false,
    );
  });
});
