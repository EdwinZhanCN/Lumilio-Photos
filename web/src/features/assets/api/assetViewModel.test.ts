import { describe, expect, it } from "vite-plus/test";
import { buildAssetApiFilter, DEFAULT_ASSET_TYPES } from "./assetViewModel";

describe("gallery media defaults", () => {
  it.each([
    {},
    { event_id: "event" },
    { album_id: 1 },
    { event_id: "event", repository_id: "repository" },
  ])("requests photos and videos for %j", (constraint) => {
    expect(buildAssetApiFilter({ types: DEFAULT_ASSET_TYPES }, constraint)).toEqual({
      ...constraint,
      types: ["PHOTO", "VIDEO"],
    });
  });

  it("preserves a photo or video filter within a gallery", () => {
    expect(buildAssetApiFilter({ types: DEFAULT_ASSET_TYPES }, { type: "VIDEO" })).toEqual({
      type: "VIDEO",
    });
  });
});
