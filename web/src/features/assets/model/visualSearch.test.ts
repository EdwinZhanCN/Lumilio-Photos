import { describe, expect, it } from "vite-plus/test";

import { classifySearchError, isAssetSearchActive, isSearchByImageFile } from "./visualSearch";

describe("classifySearchError", () => {
  const instance = "urn:lumilio:problem:0123456789abcdef0123456789abcdef";

  it("maps the embedding-missing Problem type", () => {
    expect(
      classifySearchError({
        type: "https://lumilio.org/problems/media/image-embedding-missing",
        status: 409,
        instance,
      }),
    ).toBe("embedding_missing");
  });

  it("maps availability Problem types", () => {
    expect(
      classifySearchError({
        type: "https://lumilio.org/problems/service/unavailable",
        status: 503,
        instance,
      }),
    ).toBe("unavailable");
  });

  it("maps the invalid-media Problem type and ignores retired envelopes", () => {
    expect(
      classifySearchError({
        type: "https://lumilio.org/problems/media/invalid-request",
        status: 422,
        instance,
      }),
    ).toBe("invalid_image");
    expect(classifySearchError({ code: 400, message: "Unsupported image" })).toBe("generic");
  });
});

describe("isAssetSearchActive", () => {
  it("is true for text, similar, or a file query", () => {
    expect(isAssetSearchActive("bird", "", null)).toBe(true);
    expect(isAssetSearchActive("", "550e8400-e29b-41d4-a716-446655440000", null)).toBe(true);
    expect(isAssetSearchActive("", "", new File(["x"], "q.jpg", { type: "image/jpeg" }))).toBe(
      true,
    );
    expect(isAssetSearchActive("  ", "", null)).toBe(false);
  });
});

describe("isSearchByImageFile", () => {
  it("accepts JPEG by MIME and RAW by extension when MIME is empty", () => {
    expect(isSearchByImageFile(new File(["x"], "q.jpg", { type: "image/jpeg" }))).toBe(true);
    expect(isSearchByImageFile(new File(["x"], "shot.CR2", { type: "" }))).toBe(true);
    expect(isSearchByImageFile(new File(["x"], "notes.txt", { type: "text/plain" }))).toBe(false);
  });
});
