import { describe, expect, it } from "vite-plus/test";
import {
  allBrands,
  findBrand,
  matchBrand,
  pickVariant,
  resolveLogoColor,
} from "./logoRegistry";

describe("matchBrand", () => {
  it("matches common makers", () => {
    expect(matchBrand("Canon", "Canon EOS R7")?.id).toBe("canon");
    expect(matchBrand("OLYMPUS IMAGING CORP.", "STYLUS1")?.id).toBe("olympus");
    expect(matchBrand("NIKON CORPORATION", "NIKON Z6")?.id).toBe("nikon");
    expect(matchBrand("Apple", "iPhone 15 Pro")?.id).toBe("apple");
  });

  it("prefers Pentax over Ricoh for Ricoh-built Pentax bodies", () => {
    // Both report Make "RICOH IMAGING COMPANY, LTD."; only the model separates
    // them, so rule order in the manifest is load-bearing.
    expect(matchBrand("RICOH IMAGING COMPANY, LTD.", "PENTAX K-1")?.id).toBe("pentax");
    expect(matchBrand("RICOH IMAGING COMPANY, LTD.", "RICOH GR III")?.id).toBe("ricoh");
  });

  it("matches on the model when the make omits the brand", () => {
    expect(matchBrand("", "LUMIX S5")?.id).toBe("panasonic");
  });

  it("returns null for unsupported brands and empty input", () => {
    expect(matchBrand("Acme Cameras", "Ac-1")).toBeNull();
    expect(matchBrand(undefined, undefined)).toBeNull();
  });
});

describe("pickVariant", () => {
  it("returns the requested variant when present", () => {
    expect(pickVariant(findBrand("sony"), { variantId: "symbol" })?.id).toBe("symbol");
  });

  it("falls back to the first variant when the request is not strict", () => {
    // Canon ships only a wordmark; a template asking for a symbol still renders.
    expect(pickVariant(findBrand("canon"), { variantId: "symbol" })?.id).toBe("wordmark");
  });

  it("returns null instead of falling back when strict", () => {
    // Dual-mark templates rely on this to skip a slot rather than print the
    // same mark twice.
    expect(pickVariant(findBrand("canon"), { variantId: "symbol", strict: true })).toBeNull();
  });

  it("returns null for an unknown brand", () => {
    expect(pickVariant(null, { variantId: "symbol" })).toBeNull();
  });
});

describe("resolveLogoColor", () => {
  const sonySymbol = pickVariant(findBrand("sony"), { variantId: "symbol", strict: true })!;
  const leicaSymbol = pickVariant(findBrand("leica"), { variantId: "symbol", strict: true })!;
  const hasselbladWordmark = pickVariant(findBrand("hasselblad"), {
    variantId: "wordmark",
    strict: true,
  })!;

  it("keeps a color-locked mark original regardless of ink or override", () => {
    expect(resolveLogoColor(leicaSymbol, "#ffffff", "#000000")).toBeNull();
  });

  it("lets an iconic variant color win over neutral template ink", () => {
    expect(resolveLogoColor(sonySymbol, "#141414", null)).toBe("#f47521");
  });

  it("lets non-neutral template ink win over an iconic color", () => {
    expect(resolveLogoColor(sonySymbol, "#b08d4c", null)).toBe("#b08d4c");
  });

  it("lets a user override win over both", () => {
    expect(resolveLogoColor(sonySymbol, "#141414", "#ffffff")).toBe("#ffffff");
  });

  it("passes template ink through for a mark with no iconic color", () => {
    expect(resolveLogoColor(hasselbladWordmark, "#1a1a1a", null)).toBe("#1a1a1a");
  });
});

describe("manifest integrity", () => {
  it("gives every variant a positive aspect and height multiplier", () => {
    // These drive layout maths; a zero or missing value silently collapses a
    // mark to nothing rather than erroring.
    for (const brand of allBrands()) {
      expect(brand.variants.length).toBeGreaterThan(0);
      for (const variant of brand.variants) {
        expect(variant.aspect).toBeGreaterThan(0);
        expect(variant.h).toBeGreaterThan(0);
      }
    }
  });

  it("points every match rule at a brand that exists", () => {
    expect(matchBrand("zeiss", "")?.id).toBe("zeiss");
    for (const brand of allBrands()) {
      expect(findBrand(brand.id)).not.toBeNull();
    }
  });
});
