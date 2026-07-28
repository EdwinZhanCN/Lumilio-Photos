import { describe, expect, it } from "vite-plus/test";

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
        liked: false,
        rating: 0,
      }),
    ).toEqual({ liked: false, rating: 0 });
  });

  it("keeps whitelisted composition and stack values", () => {
    expect(
      normalizeAssetUserFilter({
        media_item: { composition: "jpeg_raw" },
        stack: { membership: "stacked", kinds: ["manual", "burst", "manual"] },
      }),
    ).toEqual({
      media_item: { composition: "jpeg_raw" },
      stack: { membership: "stacked", kinds: ["burst", "manual"] },
    });
  });

  it("accepts the live photo composition", () => {
    expect(normalizeAssetUserFilter({ media_item: { composition: "live_photo" } })).toEqual({
      media_item: { composition: "live_photo" },
    });
  });

  it("drops unknown composition and stack values", () => {
    expect(
      normalizeAssetUserFilter({
        media_item: { composition: "sidecar" as never },
        stack: { membership: "grouped" as never, kinds: ["panorama" as never] },
      }),
    ).toEqual({});
  });

  it("drops stack kinds when membership is unstacked", () => {
    expect(
      normalizeAssetUserFilter({
        stack: { membership: "unstacked", kinds: ["burst"] },
      }),
    ).toEqual({ stack: { membership: "unstacked" } });
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
        media_item: { composition: "no_raw" },
        stack: { kinds: ["burst"] },
        location: { north: 40, south: 30, east: 20, west: 10 },
        album_id: 42,
      }),
    ).toEqual(new Set(["media_item", "stack", "liked", "location"]));
  });

  it("counts canonical active filters", () => {
    expect(
      countActiveAssetUserFilters({
        media_item: { composition: "contains_raw" },
        stack: { membership: "stacked" },
        rating: 0,
        tag_names: ["travel"],
      }),
    ).toBe(4);
  });
});
