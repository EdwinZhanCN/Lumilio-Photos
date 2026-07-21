import { describe, expect, it } from "vite-plus/test";

import { parseAssetBrowseParams, serializeAssetBrowseParams } from "./browseRouteState";

describe("asset browse route state", () => {
  it("round-trips all supported values", () => {
    const params = serializeAssetBrowseParams({
      query: "mountain",
      sort: "recently_added",
      filter: {
        type: "PHOTO",
        raw: false,
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
      sort: "recently_added",
      filter: {
        type: "PHOTO",
        raw: false,
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
        new URLSearchParams("type=audio&raw=maybe&rating=8&from=nope&bbox=200,95,-200,-95"),
      ),
    ).toEqual({ query: "", sort: "date_captured", filter: {} });
  });

  it("omits defaults from serialized URLs", () => {
    expect(
      serializeAssetBrowseParams({ query: "", sort: "date_captured", filter: {} }).toString(),
    ).toBe("");
  });
});
