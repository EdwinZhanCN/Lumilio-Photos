import { describe, expect, it } from "vitest";
import type { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "@/features/assets/types/assets.type";
import {
  createBrowseGroupsFromAssetGroups,
  dedupeBrowseItemsById,
  findBrowseItemIndexByAssetId,
  flattenBrowseGroups,
  getBrowseItemAsset,
  getBrowseItemAssetId,
} from "./browseItems";

const createAsset = (
  assetId: string,
  overrides: Partial<Asset> = {},
): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
    ...overrides,
  }) as Asset;

describe("browseItems", () => {
  it("creates asset items for non-stacked assets", () => {
    const groups: AssetGroup[] = [
      {
        key: "flat:all",
        assets: [createAsset("a"), createAsset("b")],
      },
    ];

    const browseGroups = createBrowseGroupsFromAssetGroups(groups);
    const items = flattenBrowseGroups(browseGroups);

    expect(items.map((item) => item.id)).toEqual(["asset:a", "asset:b"]);
    expect(items.map(getBrowseItemAssetId)).toEqual(["a", "b"]);
  });

  it("collapses stacked assets within the same group", () => {
    const groups: AssetGroup[] = [
      {
        key: "flat:all",
        assets: [
          createAsset("stack-1", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
          createAsset("stack-2", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: true,
            },
          }),
        ],
      },
    ];

    const items = flattenBrowseGroups(createBrowseGroupsFromAssetGroups(groups));

    expect(items).toHaveLength(1);
    expect(items[0]?.id).toBe("stack:stack-1");
    expect(getBrowseItemAssetId(items[0]!)).toBe("stack-2");
  });

  it("keeps only one stack item across groups and moves it to the representative group", () => {
    const groups: AssetGroup[] = [
      {
        key: "date:this_month",
        assets: [
          createAsset("member", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
        ],
      },
      {
        key: "date:yesterday",
        assets: [
          createAsset("cover", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: true,
            },
          }),
        ],
      },
    ];

    const browseGroups = createBrowseGroupsFromAssetGroups(groups);

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.key).toBe("date:yesterday");
    expect(browseGroups[0]?.items.map((item) => item.id)).toEqual([
      "stack:stack-1",
    ]);
  });

  it("falls back to the first loaded member when no cover is present", () => {
    const groups: AssetGroup[] = [
      {
        key: "flat:all",
        assets: [
          createAsset("first", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
          createAsset("second", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
        ],
      },
    ];

    const item = flattenBrowseGroups(createBrowseGroupsFromAssetGroups(groups))[0]!;

    expect(getBrowseItemAssetId(item)).toBe("first");
  });

  it("finds stack items by representative or member asset id", () => {
    const groups: AssetGroup[] = [
      {
        key: "flat:all",
        assets: [
          createAsset("member", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
          createAsset("cover", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: true,
            },
          }),
          createAsset("solo"),
        ],
      },
    ];

    const items = flattenBrowseGroups(createBrowseGroupsFromAssetGroups(groups));

    expect(findBrowseItemIndexByAssetId(items, "cover")).toBe(0);
    expect(findBrowseItemIndexByAssetId(items, "member")).toBe(0);
    expect(findBrowseItemIndexByAssetId(items, "solo")).toBe(1);
  });

  it("dedupes browse items by id while keeping first occurrence order", () => {
    const browseGroups = createBrowseGroupsFromAssetGroups([
      {
        key: "search:top_results",
        assets: [
          createAsset("cover", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: true,
            },
          }),
        ],
      },
      {
        key: "search:results",
        assets: [
          createAsset("member", {
            stack: {
              stack_id: "stack-1",
              stack_size: 2,
              stack_cover: false,
            },
          }),
          createAsset("solo"),
        ],
      },
    ]);

    const deduped = dedupeBrowseItemsById(flattenBrowseGroups(browseGroups));

    expect(deduped.map((item) => item.id)).toEqual([
      "stack:stack-1",
      "asset:solo",
    ]);
    expect(getBrowseItemAsset(deduped[0]!).asset_id).toBe("cover");
  });
});
