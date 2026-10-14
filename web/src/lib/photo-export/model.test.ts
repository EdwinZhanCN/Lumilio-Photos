import { describe, expect, it } from "vite-plus/test";
import { defaultPhotoExportSettings, exportDimensions, photoExportFilename } from "./model";

describe("photo export contract", () => {
  it("defaults both entry points to a named JPEG at 92 percent", () => {
    const settings = defaultPhotoExportSettings("trip.final.CR3");
    expect(settings).toEqual({
      format: "jpeg",
      quality: 0.92,
      sizeMode: { kind: "original" },
      filename: "trip.final-lumilio",
    });
    expect(photoExportFilename(settings)).toBe("trip.final-lumilio.jpg");
  });
  it("never upscales and keeps portrait and tiny dimensions valid", () => {
    expect(exportDimensions(2000, 3000, { kind: "longEdge", longEdge: 1200 })).toEqual({
      width: 800,
      height: 1200,
    });
    expect(exportDimensions(2000, 3000, { kind: "longEdge", longEdge: 9000 })).toEqual({
      width: 2000,
      height: 3000,
    });
    expect(exportDimensions(1, 100, { kind: "percent", percent: 1 })).toEqual({
      width: 1,
      height: 1,
    });
    expect(exportDimensions(0, 0, { kind: "original" })).toBeNull();
  });
  it("uses the chosen extension and makes file names safe without losing dotted names", () => {
    const settings = defaultPhotoExportSettings();
    expect(photoExportFilename({ ...settings, filename: "trip.final.webp", format: "jpeg" })).toBe(
      "trip.final.jpg",
    );
    expect(photoExportFilename({ ...settings, filename: "../photo:edit" })).toBe(
      ".._photo_edit.jpg",
    );
    expect(photoExportFilename({ ...settings, filename: " ... " })).toBe("export.jpg");
  });
});
