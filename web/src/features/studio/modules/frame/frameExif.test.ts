import { describe, expect, it } from "vite-plus/test";
import {
  cameraLabel,
  captureDate,
  extractFrameExif,
  formatExifLine,
  hasSufficientExif,
  resolveTextTokens,
  shootingParams,
} from "./frameExif";

describe("extractFrameExif", () => {
  it("formats exiftool-style human-readable values", () => {
    const exif = extractFrameExif({
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
    const exif = extractFrameExif({
      Model: "Canon EOS R7",
      FocalLength: "400.0 mm",
      ExposureTime: 0.0015625, // 1/640
    });
    expect(exif.focalLength).toBe("400mm");
    expect(exif.shutter).toBe("1/640s");
  });

  it("omits fields that are missing or empty", () => {
    const exif = extractFrameExif({ Model: "X", FNumber: "" });
    expect(exif.aperture).toBeUndefined();
    expect(shootingParams(exif)).toHaveLength(0);
  });
});

describe("hasSufficientExif", () => {
  it("requires a camera label plus at least one shooting parameter", () => {
    expect(hasSufficientExif(extractFrameExif({ Model: "X", ISO: 200 }))).toBe(true);
    expect(hasSufficientExif(extractFrameExif({ Model: "X" }))).toBe(false);
    expect(hasSufficientExif(extractFrameExif({ ISO: 200 }))).toBe(false);
    expect(hasSufficientExif(extractFrameExif(null))).toBe(false);
  });

  it("prefers model over make for the camera label", () => {
    expect(cameraLabel(extractFrameExif({ Make: "SONY", Model: "ILCE-7M4" }))).toBe("ILCE-7M4");
    expect(cameraLabel(extractFrameExif({ Make: "SONY" }))).toBe("SONY");
  });
});

describe("formatExifLine", () => {
  const exif = extractFrameExif({
    Model: "X-T5",
    FocalLength: "35 mm",
    FNumber: 2,
    ExposureTime: "1/250",
    ISO: 400,
  });

  it("joins plain values with a spaced pipe", () => {
    expect(formatExifLine(exif, ["focal", "aperture"])).toBe("35mm   |   f/2");
  });

  it("drops the unit prefix in labeled mode so the label is not repeated", () => {
    // "ISO | ISO 400" would read wrong.
    expect(formatExifLine(exif, ["iso"], { labeled: true })).toBe("ISO | 400");
    expect(formatExifLine(exif, ["aperture"], { labeled: true })).toBe("Aperture | f/2");
  });

  it("omits fields the photo has no value for", () => {
    const sparse = extractFrameExif({ Model: "X", ISO: 200 });
    expect(formatExifLine(sparse, ["focal", "aperture", "iso"])).toBe("ISO 200");
  });
});

describe("resolveTextTokens", () => {
  const exif = extractFrameExif({
    Make: "FUJIFILM",
    Model: "X-T5",
    LensModel: "XF33mmF1.4",
    DateTimeOriginal: "2026:05:12 09:30:00",
  });

  it("substitutes known tokens", () => {
    expect(resolveTextTokens("{camera_model}", exif)).toBe("X-T5");
    expect(resolveTextTokens("{lens_model}", exif)).toBe("XF33mmF1.4");
    expect(resolveTextTokens("{date}", exif)).toBe("2026.05.12");
  });

  it("resolves unknown or unavailable tokens to nothing", () => {
    expect(resolveTextTokens("{nonsense}", exif)).toBe("");
    expect(resolveTextTokens("{lens_model}", extractFrameExif({ Model: "GR III" }))).toBe("");
  });
});

describe("captureDate", () => {
  it("is empty when the photo has no timestamp", () => {
    expect(captureDate(extractFrameExif({ Model: "X" }))).toBe("");
  });
});
