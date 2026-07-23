import { describe, expect, it } from "vite-plus/test";
import { buildPreservedTags } from "./exif";

describe("buildPreservedTags", () => {
  const source = {
    Make: "FUJIFILM",
    Model: "X-T5",
    LensModel: "XF23mmF1.4 R LM WR",
    FNumber: 1.4,
    ExposureTime: "1/500",
    ISO: 200,
    DateTimeOriginal: "2026:06:01 12:30:00",
    GPSLatitude: 40.7128,
    GPSLongitude: -74.006,
    Orientation: 6, // rotated original; must be overridden
    ExifImageWidth: 6000,
    ExifImageHeight: 4000,
    ThumbnailImage: "(binary)", // not in the allowlist — must be dropped
    unrelated: { nested: true }, // non-scalar — must be dropped
  };

  it("copies the descriptive allowlist under group-qualified keys", () => {
    const tags = buildPreservedTags(source, 3000, 2000);
    expect(tags["EXIF:Make"]).toBe("FUJIFILM");
    expect(tags["EXIF:LensModel"]).toBe("XF23mmF1.4 R LM WR");
    expect(tags["EXIF:FNumber"]).toBe(1.4);
    expect(tags["EXIF:DateTimeOriginal"]).toBe("2026:06:01 12:30:00");
    expect(tags["GPS:GPSLatitude"]).toBe(40.7128);
    expect(tags["GPS:GPSLongitude"]).toBe(-74.006);
  });

  it("forces Orientation upright because rotation is baked into pixels", () => {
    expect(buildPreservedTags(source, 3000, 2000)["EXIF:Orientation"]).toBe(1);
  });

  it("sets the pixel dimensions to the exported size", () => {
    const tags = buildPreservedTags(source, 3000, 2000);
    expect(tags["EXIF:ExifImageWidth"]).toBe(3000);
    expect(tags["EXIF:ExifImageHeight"]).toBe(2000);
  });

  it("drops tags outside the allowlist and non-scalar values", () => {
    const tags = buildPreservedTags(source, 3000, 2000);
    expect(tags).not.toHaveProperty("EXIF:ThumbnailImage");
    expect(tags).not.toHaveProperty("EXIF:unrelated");
  });

  it("omits absent tags rather than writing empty values", () => {
    const tags = buildPreservedTags({ Make: "Canon" }, 100, 100);
    expect(tags).not.toHaveProperty("EXIF:Model");
    expect(tags).not.toHaveProperty("GPS:GPSLatitude");
    expect(tags["EXIF:Make"]).toBe("Canon");
  });
});
