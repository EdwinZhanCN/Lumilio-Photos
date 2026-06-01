import { describe, expect, it, vi, beforeEach } from "vite-plus/test";
import type { Asset, StackMemberDTO } from "@/lib/assets/types";
import client from "@/lib/http-commons/client";
import {
  normalizeStackMembers,
  resolveStackCarouselAssets,
} from "./useStackCarouselAssets";

vi.mock("@/lib/http-commons/client", () => ({
  default: {
    GET: vi.fn(),
  },
}));

const createAsset = (
  assetId: string,
  overrides: Partial<Asset> = {},
): Asset =>
  ({
    asset_id: assetId,
    original_filename: `${assetId}.jpg`,
    ...overrides,
  }) as Asset;

const createMember = (
  assetId: string,
  position?: number,
  overrides: Partial<StackMemberDTO> = {},
): StackMemberDTO => ({
  asset_id: assetId,
  position,
  relation: "alternative",
  ...overrides,
});

describe("useStackCarouselAssets helpers", () => {
  beforeEach(() => {
    vi.mocked(client.GET).mockReset();
  });

  it("sorts members by position with undefined positions last", () => {
    const members = normalizeStackMembers([
      createMember("c"),
      createMember("b", 1),
      createMember("a", 0),
    ]);

    expect(members.map((member) => member.asset_id)).toEqual(["a", "b", "c"]);
  });

  it("reuses the current asset and preserves member order", async () => {
    const currentAsset = createAsset("cover");
    vi.mocked(client.GET)
      .mockResolvedValueOnce({ data: createAsset("member-1") } as never)
      .mockResolvedValueOnce({ data: createAsset("member-2") } as never);

    const assets = await resolveStackCarouselAssets(currentAsset, [
      createMember("member-1", 0),
      createMember("cover", 1),
      createMember("member-2", 2),
    ]);

    expect(client.GET).toHaveBeenCalledTimes(2);
    expect(assets.map((asset) => asset.asset_id)).toEqual([
      "member-1",
      "cover",
      "member-2",
    ]);
  });

  it("filters failed fetches but keeps successful assets", async () => {
    const currentAsset = createAsset("cover");
    vi.mocked(client.GET)
      .mockRejectedValueOnce(new Error("boom"))
      .mockResolvedValueOnce({ data: createAsset("member-2") } as never);

    const assets = await resolveStackCarouselAssets(currentAsset, [
      createMember("member-1", 0),
      createMember("member-2", 1),
      createMember("cover", 2),
    ]);

    expect(assets.map((asset) => asset.asset_id)).toEqual([
      "member-2",
      "cover",
    ]);
  });

  it("dedupes duplicate member ids", async () => {
    const currentAsset = createAsset("cover");
    vi.mocked(client.GET).mockResolvedValue({
      data: createAsset("member-1"),
    } as never);

    const assets = await resolveStackCarouselAssets(currentAsset, [
      createMember("member-1", 0),
      createMember("member-1", 1),
      createMember("cover", 2),
    ]);

    expect(assets.map((asset) => asset.asset_id)).toEqual(["member-1", "cover"]);
  });
});
