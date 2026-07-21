import { describe, expect, it } from "vite-plus/test";
import { cameraLabel, extractBorderExif, hasSufficientExif, shootingParams } from "./exifInfo";
import { brandDisplayName, matchBrandKey } from "./logoAssets";

describe("extractBorderExif", () => {
  it("formats exiftool-style human-readable values", () => {
    const exif = extractBorderExif({
      Make: "OLYMPUS IMAGING CORP.",
      Model: "STYLUS1",
      FocalLength: "16.1 mm",
      FNumber: 2.8,
      ExposureTime: "1/1000",
      ISO: 100,
      DateTimeOriginal: "2014:01:23 14:57:18",
    });

    expect(exif.make).toBe("OLYMPUS IMAGING CORP.");
    expect(exif.model).toBe("STYLUS1");
    expect(exif.focalLength).toBe("16.1mm");
    expect(exif.aperture).toBe("f/2.8");
    expect(exif.shutter).toBe("1/1000s");
    expect(exif.iso).toBe("ISO 100");
    expect(exif.dateTime).toBe("2014-01-23 14:57:18");
  });

  it("handles numeric exposure times and trailing-zero focal lengths", () => {
    const exif = extractBorderExif({
      Model: "Canon EOS R7",
      FocalLength: "400.0 mm",
      ExposureTime: 0.0015625, // 1/640
    });
    expect(exif.focalLength).toBe("400mm");
    expect(exif.shutter).toBe("1/640s");
  });

  it("omits fields that are missing or empty", () => {
    const exif = extractBorderExif({ Model: "X", FNumber: "" });
    expect(exif.aperture).toBeUndefined();
    expect(shootingParams(exif)).toHaveLength(0);
  });
});

describe("hasSufficientExif", () => {
  it("requires a camera label plus at least one shooting parameter", () => {
    expect(hasSufficientExif(extractBorderExif({ Model: "X", ISO: 200 }))).toBe(true);
    expect(hasSufficientExif(extractBorderExif({ Model: "X" }))).toBe(false);
    expect(hasSufficientExif(extractBorderExif({ ISO: 200 }))).toBe(false);
    expect(hasSufficientExif(extractBorderExif(null))).toBe(false);
  });

  it("prefers model over make for the camera label", () => {
    expect(cameraLabel(extractBorderExif({ Make: "SONY", Model: "ILCE-7M4" }))).toBe("ILCE-7M4");
    expect(cameraLabel(extractBorderExif({ Make: "SONY" }))).toBe("SONY");
  });
});

describe("matchBrandKey", () => {
  it("matches common makers to bundled logo keys", () => {
    expect(matchBrandKey("Canon", "Canon EOS R7")).toBe("canon");
    expect(matchBrandKey("OLYMPUS IMAGING CORP.", "STYLUS1")).toBe("olympus");
    expect(matchBrandKey("NIKON CORPORATION", "NIKON Z6")).toBe("nikon");
    expect(matchBrandKey("Apple", "iPhone 15 Pro")).toBe("apple");
  });

  it("prefers Pentax over Ricoh for Ricoh-built Pentax bodies", () => {
    expect(matchBrandKey("RICOH IMAGING COMPANY, LTD.", "PENTAX K-1")).toBe("pentax");
    expect(matchBrandKey("RICOH IMAGING COMPANY, LTD.", "RICOH GR III")).toBe("ricoh");
  });

  it("returns null for unsupported brands", () => {
    expect(matchBrandKey("Acme Cameras", "Ac-1")).toBeNull();
  });
});

describe("brandDisplayName", () => {
  it("uses curated display names for matched brands", () => {
    expect(brandDisplayName("OLYMPUS IMAGING CORP.", "olympus")).toBe("OLYMPUS");
    expect(brandDisplayName("Canon", "canon")).toBe("Canon");
  });

  it("falls back to a tidied Make for unmatched brands", () => {
    expect(brandDisplayName("Acme Imaging Corp.", null)).toBe("Acme");
    expect(brandDisplayName(undefined, null)).toBeNull();
  });
});
