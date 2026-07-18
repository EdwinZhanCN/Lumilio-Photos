import { describe, expect, it } from "vitest";

import {
  countActiveAssetUserFilters,
  getConstrainedFilterKeys,
  mergeAssetFilters,
  normalizeAssetUserFilter,
} from "./filter";

describe("asset filter model", () => {
  it("preserves meaningful false and zero values", () => {
    expect(
      normalizeAssetUserFilter({
        raw: false,
        liked: false,
        rating: 0,
      }),
    ).toEqual({ raw: false, liked: false, rating: 0 });
  });

  it("normalizes text and tag values", () => {
    expect(
      normalizeAssetUserFilter({
        filename: { operator: "starts_with", value: "  IMG_ " },
        camera_model: " Fujifilm X-T5 ",
        lens: " ",
        tag_names: [" Travel ", "travel", "", "Summer"],
      }),
    ).toEqual({
      filename: { operator: "starts_with", value: "IMG_" },
      camera_model: "Fujifilm X-T5",
      tag_names: ["Travel", "Summer"],
    });
  });

  it("lets the page constraint override user-controlled fields", () => {
    expect(
      mergeAssetFilters({ liked: false, type: "VIDEO", rating: 4 }, { liked: true, album_id: 42 }),
    ).toEqual({ liked: true, type: "VIDEO", rating: 4, album_id: 42 });
  });

  it("derives locked user fields from active constraints", () => {
    expect(
      getConstrainedFilterKeys({
        liked: false,
        location: { north: 40, south: 30, east: 20, west: 10 },
        album_id: 42,
      }),
    ).toEqual(new Set(["liked", "location"]));
  });

  it("counts canonical active filters", () => {
    expect(
      countActiveAssetUserFilters({
        raw: false,
        rating: 0,
        tag_names: ["travel"],
      }),
    ).toBe(3);
  });
});
