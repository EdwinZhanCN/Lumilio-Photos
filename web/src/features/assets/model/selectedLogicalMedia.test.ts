import { describe, expect, it } from "vitest";
import { toSelectedLogicalMedia } from "./selectedLogicalMedia";

describe("selected logical media", () => {
  it("emits one logical identity for a media row", () => {
    const result = toSelectedLogicalMedia({
      type: "media_item",
      id: "media:m1",
      mediaItemId: "m1",
      asset: { asset_id: "a1" },
    } as never);
    expect(result).toEqual({
      browse_item_id: "media:m1",
      media_item_ids: ["m1"],
      representative_asset_ids: ["a1"],
      complete: true,
    });
  });

  it("deduplicates stack members and reports incomplete hydration", () => {
    const result = toSelectedLogicalMedia({
      type: "stack",
      id: "stack:s1",
      members: [
        { mediaItemId: "m1", primaryAssetId: "a1" },
        { mediaItemId: "m1", primaryAssetId: "a1" },
        { mediaItemId: "m2", primaryAssetId: "a2" },
      ],
      assets: [{ id: "a1" }],
    } as never);
    expect(result.media_item_ids).toEqual(["m1", "m2"]);
    expect(result.complete).toBe(false);
  });
});
