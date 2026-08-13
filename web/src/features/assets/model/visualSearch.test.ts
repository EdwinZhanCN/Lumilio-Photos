import { describe, expect, it } from "vite-plus/test";

import { classifySearchError, isAssetSearchActive, isSearchByImageFile } from "./visualSearch";

describe("classifySearchError", () => {
  it("maps embedding_missing and 409", () => {
    expect(classifySearchError({ error: "embedding_missing", code: 409 })).toBe(
      "embedding_missing",
    );
    expect(
      classifySearchError(Object.assign(new Error("embedding_missing"), { status: 409 })),
    ).toBe("embedding_missing");
  });

  it("maps 503 to unavailable", () => {
    expect(classifySearchError({ code: 503, message: "down" })).toBe("unavailable");
  });

  it("maps unsupported image to invalid_image", () => {
    expect(classifySearchError({ code: 400, message: "Unsupported or unreadable image" })).toBe(
      "invalid_image",
    );
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
