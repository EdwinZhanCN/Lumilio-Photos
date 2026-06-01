import { describe, expect, it } from "vite-plus/test";

import type { Asset } from "@/lib/http-commons";
import { isAudio, isPhoto, isRawPhoto, isVideo } from "./mediaTypes";

describe("mediaTypes", () => {
  it("prefers asset.type over MIME prefixes", () => {
    const asset = {
      type: "VIDEO",
      mime_type: "image/jpeg",
      specific_metadata: {},
    } as Asset;

    expect(isVideo(asset)).toBe(true);
    expect(isPhoto(asset)).toBe(false);
    expect(isAudio(asset)).toBe(false);
  });

  it("falls back to MIME prefixes for legacy assets without type", () => {
    const asset = {
      type: "",
      mime_type: "audio/mpeg",
      specific_metadata: {},
    } as Asset;

    expect(isAudio(asset)).toBe(true);
    expect(isVideo(asset)).toBe(false);
    expect(isPhoto(asset)).toBe(false);
  });

  it("reads RAW state from specific metadata", () => {
    const asset = {
      type: "PHOTO",
      mime_type: "image/x-canon-cr3",
      specific_metadata: { is_raw: true },
    } as Asset;

    expect(isRawPhoto(asset)).toBe(true);
  });
});
