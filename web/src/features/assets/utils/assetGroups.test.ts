import { describe, expect, it } from "vite-plus/test";
import {
  flattenAssetGroups,
  formatAssetGroupLabel,
  groupAssetsBySort,
  mergeAdjacentAssetGroups,
} from "./assetGroups";

describe("assetGroups", () => {
  it("merges adjacent groups with the same key", () => {
    const groups = mergeAdjacentAssetGroups(
      [
        { key: "date:today", assets: [{ asset_id: "a" } as any] },
        { key: "date:yesterday", assets: [{ asset_id: "b" } as any] },
      ],
      [
        { key: "date:yesterday", assets: [{ asset_id: "c" } as any] },
        { key: "date:month:2025-03", assets: [{ asset_id: "d" } as any] },
      ],
    );

    expect(groups).toHaveLength(3);
    expect(groups[1]?.key).toBe("date:yesterday");
    expect(groups[1]?.assets).toHaveLength(2);
    expect(flattenAssetGroups(groups).map((asset) => asset.asset_id)).toEqual(["a", "b", "c", "d"]);
  });

  it("formats stable date and type keys for display", () => {
    const t = (_key: string, options?: { defaultValue?: string }) => options?.defaultValue ?? _key;

    expect(formatAssetGroupLabel("date:today", t as any, "en")).toBe("Today");
    expect(formatAssetGroupLabel("type:image/jpeg", t as any, "en")).toBe("image/jpeg");
    expect(formatAssetGroupLabel("date:month:2025-03", t as any, "en")).toBe("March 2025");
  });

  it("groups older assets by month instead of year", () => {
    const groups = groupAssetsBySort(
      [
        {
          asset_id: "march",
          taken_time: "2024-03-15T12:00:00.000Z",
        } as any,
        {
          asset_id: "feb",
          taken_time: "2024-02-15T12:00:00.000Z",
        } as any,
      ],
      "date_captured",
      new Date("2026-05-05T12:00:00.000Z"),
    );

    expect(groups.map((group) => group.key)).toEqual(["date:month:2024-03", "date:month:2024-02"]);
  });
});
