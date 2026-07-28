import { describe, expect, it } from "vite-plus/test";
import type { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "../types";
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
  type BrowseItemDTO,
} from "./browseItems";

const createAsset = (assetId: string, overrides: Partial<Asset> = {}): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
    ...overrides,
  }) as Asset;

/** Server browse rows are media items; the media item id doubles as the asset id here. */
const mediaItemDTO = (assetId: string, overrides: Partial<Asset> = {}): BrowseItemDTO => ({
  type: "media_item",
  id: `media:${assetId}`,
  media_item: {
    media_item_id: assetId,
    primary_asset: createAsset(assetId, overrides),
  },
});

const memberDTO = (assetId: string) => ({
  media_item_id: assetId,
  primary_asset_id: assetId,
});

const stackDTO = (
  stackId: string,
  options: {
    coverAssetId: string;
    coverOverrides?: Partial<Asset>;
    memberAssetIds: string[];
    matchedAssetIds?: string[];
    stackKind?: "burst" | "manual";
  },
): BrowseItemDTO => ({
  type: "stack",
  id: `stack:${stackId}`,
  stack: {
    stack_id: stackId,
    stack_kind: options.stackKind ?? "burst",
    cover: {
      media_item_id: options.coverAssetId,
      primary_asset: createAsset(options.coverAssetId, options.coverOverrides),
    },
    members: options.memberAssetIds.map(memberDTO),
    matched_members: (options.matchedAssetIds ?? options.memberAssetIds).map(memberDTO),
  },
});

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

    expect(items.map((item) => item.id)).toEqual(["media:a", "media:b"]);
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
    expect(browseGroups[0]?.items.map((item) => item.id)).toEqual(["stack:stack-1", "media:solo"]);
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
    expect(browseGroups[0]?.items.map((item) => item.id)).toEqual(["stack:stack-1"]);
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

  it("finds stack items by member primary asset id when only the browse payload is loaded", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      stackDTO("stack-1", {
        coverAssetId: "cover",
        memberAssetIds: ["cover", "member"],
        matchedAssetIds: ["member"],
      }),
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

    expect(deduped.map((item) => item.id)).toEqual(["stack:stack-1", "media:solo"]);
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
      ["media:solo", "stack:stack-1", "media:missing"],
      items,
    );

    expect(resolved.map((item) => item.id)).toEqual(["media:solo", "stack:stack-1"]);
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
      resolveBrowseSelectedAssetIds(["stack:stack-1", "media:solo", "asset:missing"], items),
    ).toEqual(["cover", "solo"]);
  });

  it("resolves stack browse selection ids to all member asset ids for whole-stack actions", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      stackDTO("stack-1", { coverAssetId: "cover", memberAssetIds: ["cover", "member"] }),
      mediaItemDTO("solo"),
    ]);

    expect(
      resolveBrowseSelectedAssetIds(["stack:stack-1", "media:solo", "media:missing"], items, {
        stackMode: "whole-stack",
      }),
    ).toEqual(["cover", "member", "solo"]);
  });

  it("dedupes resolved whole-stack member asset ids", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      stackDTO("stack-1", { coverAssetId: "cover", memberAssetIds: ["cover", "member"] }),
      mediaItemDTO("member"),
    ]);

    expect(
      resolveBrowseSelectedAssetIds(["stack:stack-1", "media:member"], items, {
        stackMode: "whole-stack",
      }),
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

    expect(findBrowseItemById(items, "media:solo")?.id).toBe("media:solo");
    expect(findBrowseItemById(items, "media:missing")).toBeUndefined();
  });

  it("maps query-like pages using BrowseItem DTO items", () => {
    const browseGroups = browseGroupsFromQueryLikePage({
      items: [
        stackDTO("stack-1", {
          coverAssetId: "cover",
          coverOverrides: {
            stack: { stack_id: "stack-1", stack_size: 2, stack_cover: true },
          },
          memberAssetIds: ["cover", "member"],
        }),
        mediaItemDTO("solo"),
      ],
      sortBy: "date_captured",
    });

    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "stack:stack-1",
      "media:solo",
    ]);
  });

  it("keeps search top results in one flat section", () => {
    const browseGroups = browseGroupsFromSearchTop({
      topItems: [
        mediaItemDTO("newer", { taken_time: "2026-05-02T00:00:00Z" }),
        mediaItemDTO("older", { taken_time: "2024-01-01T00:00:00Z" }),
      ],
    });

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.key).toBe("search:top_results");
    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "media:newer",
      "media:older",
    ]);
  });

  it("keeps search result pages in one flat results section", () => {
    const browseGroups = browseGroupsFromSearchResultsPage({
      resultItems: [
        mediaItemDTO("newer", { taken_time: "2026-05-02T00:00:00Z" }),
        mediaItemDTO("older", { taken_time: "2024-01-01T00:00:00Z" }),
      ],
    });

    expect(browseGroups).toHaveLength(1);
    expect(browseGroups[0]?.key).toBe("search:results");
    expect(flattenBrowseGroups(browseGroups).map((item) => item.id)).toEqual([
      "media:newer",
      "media:older",
    ]);
  });

  it("creates browse items from backend browse dto payloads", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      mediaItemDTO("solo"),
      stackDTO("stack-1", {
        coverAssetId: "cover",
        memberAssetIds: ["cover", "member"],
        matchedAssetIds: ["member"],
        stackKind: "manual",
      }),
    ]);

    expect(items.map((item) => item.id)).toEqual(["media:solo", "stack:stack-1"]);
    expect(items[1]).toMatchObject({
      type: "stack",
      stackKind: "manual",
      members: [
        { mediaItemId: "cover", primaryAssetId: "cover" },
        { mediaItemId: "member", primaryAssetId: "member" },
      ],
      matchedMembers: [{ mediaItemId: "member", primaryAssetId: "member" }],
    });
  });

  it("carries media item composition facts through the dto mapping", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "media_item",
        id: "media:pair",
        media_item: {
          media_item_id: "pair",
          primary_asset: createAsset("pair"),
          composition: {
            component_count: 2,
            has_raw: true,
            has_jpeg: true,
            has_edited: false,
            has_live_motion: false,
          },
        },
      },
    ]);

    expect(items[0]).toMatchObject({
      type: "media_item",
      id: "media:pair",
      composition: { componentCount: 2, hasRaw: true, hasJpeg: true },
    });
  });

  it("grafts the stack preview onto the cover asset so overlays keep working", () => {
    const items = createBrowseItemsFromBrowseItemDTOs([
      {
        type: "stack",
        id: "stack:stack-1",
        stack: {
          stack_id: "stack-1",
          stack_kind: "burst",
          cover: {
            media_item_id: "cover",
            primary_asset: createAsset("cover"),
            stack: { stack_id: "stack-1", stack_size: 3, stack_cover: true, stack_kind: "burst" },
          },
          members: ["cover", "member", "member-2"].map(memberDTO),
          matched_members: [memberDTO("member")],
        },
      },
    ]);

    expect(getBrowseItemAsset(items[0]!).stack).toMatchObject({
      stack_id: "stack-1",
      stack_size: 3,
    });
  });
});
