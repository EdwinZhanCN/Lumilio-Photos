import { describe, expect, it } from "vite-plus/test";

import { parseAssetBrowseParams, serializeAssetBrowseParams } from "./browseRouteState";

describe("asset browse route state", () => {
  it("round-trips all supported values", () => {
    const params = serializeAssetBrowseParams({
      query: "mountain",
      sort: "recently_added",
      filter: {
        type: "PHOTO",
        media_item: { composition: "jpeg_raw" },
        stack: { membership: "stacked", kinds: ["burst", "manual"] },
        rating: 0,
        liked: true,
        filename: { operator: "starts_with", value: "IMG_" },
        date: { from: "2026-01-01", to: "2026-07-01" },
        camera_model: "Fujifilm X-T5",
        lens: "XF 23mm",
        tag_names: ["travel", "summer"],
        location: { west: -123, south: 37, east: -122, north: 38 },
      },
    });

    expect(parseAssetBrowseParams(params)).toEqual({
      query: "mountain",
      similarAssetId: "",
      sort: "recently_added",
      filter: {
        type: "PHOTO",
        media_item: { composition: "jpeg_raw" },
        stack: { membership: "stacked", kinds: ["burst", "manual"] },
        rating: 0,
        liked: true,
        filename: { operator: "starts_with", value: "IMG_" },
        date: { from: "2026-01-01", to: "2026-07-01" },
        camera_model: "Fujifilm X-T5",
        lens: "XF 23mm",
        tag_names: ["travel", "summer"],
        location: { west: -123, south: 37, east: -122, north: 38 },
      },
    });
  });

  it("preserves unrelated route parameters", () => {
    const current = new URLSearchParams("pin=pin-1&q=old&tag=old");
    const next = serializeAssetBrowseParams(
      { query: "new", sort: "date_captured", filter: { liked: false } },
      current,
    );

    expect(next.get("pin")).toBe("pin-1");
    expect(next.get("q")).toBe("new");
    expect(next.getAll("tag")).toEqual([]);
    expect(next.get("liked")).toBe("false");
  });

  it("ignores invalid values", () => {
    expect(
      parseAssetBrowseParams(
        new URLSearchParams(
          "type=audio&composition=sidecar&stack_membership=grouped&stack_kind=panorama&rating=8&from=nope&bbox=200,95,-200,-95",
        ),
      ),
    ).toEqual({ query: "", similarAssetId: "", sort: "date_captured", filter: {} });
  });

  it("does not read the retired raw parameter", () => {
    expect(parseAssetBrowseParams(new URLSearchParams("raw=false")).filter).toEqual({});
  });

  it("keeps a lone stack kind without a membership", () => {
    expect(parseAssetBrowseParams(new URLSearchParams("stack_kind=burst")).filter).toEqual({
      stack: { kinds: ["burst"] },
    });
  });

  it("omits defaults from serialized URLs", () => {
    expect(
      serializeAssetBrowseParams({ query: "", sort: "date_captured", filter: {} }).toString(),
    ).toBe("");
  });

  it("round-trips similar and drops q when both are present", () => {
    const similar = "550e8400-e29b-41d4-a716-446655440000";
    const params = serializeAssetBrowseParams({
      query: "mountain",
      similarAssetId: similar,
      sort: "date_captured",
      filter: {},
    });
    expect(params.get("similar")).toBe(similar);
    expect(params.get("q")).toBeNull();
    expect(parseAssetBrowseParams(new URLSearchParams(`q=keep&similar=${similar}`))).toEqual({
      query: "",
      similarAssetId: similar,
      sort: "date_captured",
      filter: {},
    });
  });

  it("ignores invalid similar values", () => {
    expect(parseAssetBrowseParams(new URLSearchParams("similar=not-a-uuid")).similarAssetId).toBe(
      "",
    );
  });
});
