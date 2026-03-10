import { describe, expect, it } from "vitest";
import {
  flattenAssetGroups,
  formatAssetGroupLabel,
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
        { key: "date:year:2025", assets: [{ asset_id: "d" } as any] },
      ],
    );

    expect(groups).toHaveLength(3);
    expect(groups[1]?.key).toBe("date:yesterday");
    expect(groups[1]?.assets).toHaveLength(2);
    expect(flattenAssetGroups(groups).map((asset) => asset.asset_id)).toEqual([
      "a",
      "b",
      "c",
      "d",
    ]);
  });

  it("formats stable date and type keys for display", () => {
    const t = (_key: string, options?: { defaultValue?: string }) =>
      options?.defaultValue ?? _key;

    expect(formatAssetGroupLabel("date:today", t as any, "en")).toBe("Today");
    expect(formatAssetGroupLabel("type:image/jpeg", t as any, "en")).toBe(
      "image/jpeg",
    );
    expect(formatAssetGroupLabel("date:year:2025", t as any, "en")).toBe(
      "2025",
    );
  });
});
