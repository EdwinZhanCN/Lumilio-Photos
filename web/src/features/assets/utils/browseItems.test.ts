import { describe, expect, it } from "vitest";
import type { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "@/features/assets/types/assets.type";
import {
  browseGroupsFromQueryLikePage,
  browseGroupsFromSearchResultsPage,
  browseGroupsFromSearchTop,
  createBrowseItemsFromBrowseItemDTOs,
  createBrowseGroupsFromAssets,
  createBrowseGroupsFromAssetGroups,
  dedupeBrowseItemsById,
  findBrowseItemById,
  findBrowseItemIndexByAssetId,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  getBrowseItemAssetId,
  resolveBrowseSelectedAssetIds,
  resolveSelectedBrowseItems,
} from "./browseItems";

const createAsset = (assetId: string, overrides: Partial<Asset> = {}): Asset =>
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

  it("creates browse groups from flat assets", () => {
    const browseGroups = createBrowseGroupsFromAssets([
      createAsset("cover", {
        stack: {
          stack_id: "stack-1",
          stack_size: 2,
          stack_cover: true,
        },
      }),
      createAsset("member", {
        stack: {
          stack_id: "stack-1",
          stack_size: 2,
          stack_cover: false,
        },
      }),
      createAsset("solo"),
    ]);

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.items.map((item) => item.id)).toEqual([
      "stack:stack-1",
      "asset:solo",
    ]);
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

    const items = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups(groups),
    );

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

    const item = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups(groups),
    )[0]!;

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

    const items = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups(groups),
    );

    expect(findBrowseItemIndexByAssetId(items, "cover")).toBe(0);
    expect(findBrowseItemIndexByAssetId(items, "member")).toBe(0);
    expect(findBrowseItemIndexByAssetId(items, "solo")).toBe(1);
  });

  it("finds stack items by memberAssetIds when only backend browse payload is loaded", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "stack",
        stack: {
          stack_id: "stack-1",
          cover_asset_id: "cover",
          cover_asset: createAsset("cover"),
          member_asset_ids: ["cover", "member"],
          matched_member_ids: ["member"],
        },
      },
    ]);

    expect(findBrowseItemIndexByAssetId(items, "member")).toBe(0);
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

  it("flattens browse groups to visible representative assets", () => {
    const assets = flattenBrowseGroupsToAssets(
      createBrowseGroupsFromAssetGroups([
        {
          key: "flat:all",
          assets: [
            createAsset("cover", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: true,
              },
            }),
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
      ]),
    );

    expect(assets.map((asset) => asset.asset_id)).toEqual(["cover", "solo"]);
  });

  it("resolves selected browse items in selection order", () => {
    const items = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups([
        {
          key: "flat:all",
          assets: [
            createAsset("cover", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: true,
              },
            }),
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
      ]),
    );

    const resolved = resolveSelectedBrowseItems(
      ["asset:solo", "stack:stack-1", "asset:missing"],
      items,
    );

    expect(resolved.map((item) => item.id)).toEqual([
      "asset:solo",
      "stack:stack-1",
    ]);
    expect(getBrowseItemAsset(resolved[1]!).asset_id).toBe("cover");
  });

  it("resolves browse selection ids to representative asset ids", () => {
    const items = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups([
        {
          key: "flat:all",
          assets: [
            createAsset("cover", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: true,
              },
            }),
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
      ]),
    );

    expect(
      resolveBrowseSelectedAssetIds(
        ["stack:stack-1", "asset:solo", "asset:missing"],
        items,
      ),
    ).toEqual(["cover", "solo"]);
  });

  it("resolves stack browse selection ids to all member asset ids for whole-stack actions", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "stack",
        stack: {
          stack_id: "stack-1",
          cover_asset_id: "cover",
          cover_asset: createAsset("cover"),
          member_asset_ids: ["cover", "member"],
        },
      },
      {
        type: "asset",
        asset: createAsset("solo"),
      },
    ]);

    expect(
      resolveBrowseSelectedAssetIds(
        ["stack:stack-1", "asset:solo", "asset:missing"],
        items,
        { stackMode: "whole-stack" },
      ),
    ).toEqual(["cover", "member", "solo"]);
  });

  it("dedupes resolved whole-stack member asset ids", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "stack",
        stack: {
          stack_id: "stack-1",
          cover_asset_id: "cover",
          cover_asset: createAsset("cover"),
          member_asset_ids: ["cover", "member"],
        },
      },
      {
        type: "asset",
        asset: createAsset("member"),
      },
    ]);

    expect(
      resolveBrowseSelectedAssetIds(
        ["stack:stack-1", "asset:member"],
        items,
        { stackMode: "whole-stack" },
      ),
    ).toEqual(["cover", "member"]);
  });

  it("finds browse items by item id", () => {
    const items = flattenBrowseGroups(
      createBrowseGroupsFromAssetGroups([
        {
          key: "flat:all",
          assets: [createAsset("solo")],
        },
      ]),
    );

    expect(findBrowseItemById(items, "asset:solo")?.id).toBe("asset:solo");
    expect(findBrowseItemById(items, "asset:missing")).toBeUndefined();
  });

  it("maps query-like pages using BrowseItem DTO items", () => {
    const browseGroups = browseGroupsFromQueryLikePage({
      items: [
        {
          type: "stack",
          stack: {
            stack_id: "stack-1",
            cover_asset_id: "cover",
            cover_asset: createAsset("cover", {
              stack: {
                stack_id: "stack-1",
                stack_size: 2,
                stack_cover: true,
              },
            }),
            member_asset_ids: ["cover", "member"],
          },
        },
        {
          type: "asset",
          asset: createAsset("solo"),
        },
      ],
      sortBy: "date_captured",
    });

    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "stack:stack-1",
      "asset:solo",
    ]);
  });

  it("keeps search top results in one flat section", () => {
    const browseGroups = browseGroupsFromSearchTop({
      topItems: [
        {
          type: "asset",
          asset: createAsset("newer", { taken_time: "2026-05-02T00:00:00Z" }),
        },
        {
          type: "asset",
          asset: createAsset("older", { taken_time: "2024-01-01T00:00:00Z" }),
        },
      ],
    });

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.key).toBe("search:top_results");
    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "asset:newer",
      "asset:older",
    ]);
  });

  it("keeps search result pages in one flat results section", () => {
    const browseGroups = browseGroupsFromSearchResultsPage({
      resultItems: [
        {
          type: "asset",
          asset: createAsset("newer", { taken_time: "2026-05-02T00:00:00Z" }),
        },
        {
          type: "asset",
          asset: createAsset("older", { taken_time: "2024-01-01T00:00:00Z" }),
        },
      ],
    });

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.key).toBe("search:results");
    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "asset:newer",
      "asset:older",
    ]);
  });

  it("creates browse items from backend browse dto payloads", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "asset",
        asset: createAsset("solo"),
      },
      {
        type: "stack",
        stack: {
          stack_id: "stack-1",
          cover_asset_id: "cover",
          cover_asset: createAsset("cover"),
          member_asset_ids: ["cover", "member"],
          matched_member_ids: ["member"],
        },
      },
    ]);

    expect(items.map((item) => item.id)).toEqual([
      "asset:solo",
      "stack:stack-1",
    ]);
    expect(items[1]).toMatchObject({
      type: "stack",
      memberAssetIds: ["cover", "member"],
      matchedMemberIds: ["member"],
    });
  });
});
