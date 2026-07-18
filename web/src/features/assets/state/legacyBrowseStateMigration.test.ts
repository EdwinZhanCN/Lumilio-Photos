import { describe, expect, it } from "vite-plus/test";
import { convertLegacyAssetBrowseState } from "./legacyBrowseStateMigration";

describe("legacy asset browse state migration", () => {
  it("converts enabled legacy filters and old filename operators", () => {
    expect(
      convertLegacyAssetBrowseState({
        filters: {
          enabled: true,
          raw: false,
          rating: 0,
          filename: { mode: "startswith", value: " IMG_ " },
          tag_names: ["travel", "TRAVEL", 42],
        },
        ui: { sortBy: "recently_added", searchQuery: " beach " },
      }),
    ).toEqual({
      query: "beach",
      sort: "recently_added",
      filter: {
        raw: false,
        rating: 0,
        filename: { operator: "starts_with", value: "IMG_" },
        tag_names: ["travel"],
      },
    });
  });

  it("does not restore filters that were globally disabled", () => {
    expect(
      convertLegacyAssetBrowseState({
        filters: { enabled: false, liked: true },
        selection: { selectionMode: "single" },
      }),
    ).toEqual({ query: "", sort: "date_captured", filter: {} });
  });
});
