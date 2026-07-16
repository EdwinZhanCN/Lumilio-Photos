import { describe, expect, it } from "vite-plus/test";

import { encodeGeohash } from "./geohash";

describe("encodeGeohash", () => {
  it("matches the standard reference example", () => {
    expect(encodeGeohash(42.6, -5.6, 5)).toBe("ezs42");
  });

  it("uses seven characters by default", () => {
    expect(encodeGeohash(39.9042, 116.4074)).toHaveLength(7);
  });

  it("rejects invalid coordinates and precision", () => {
    expect(encodeGeohash(Number.NaN, 0)).toBeNull();
    expect(encodeGeohash(91, 0)).toBeNull();
    expect(encodeGeohash(0, 181)).toBeNull();
    expect(encodeGeohash(0, 0, 0)).toBeNull();
  });
});
