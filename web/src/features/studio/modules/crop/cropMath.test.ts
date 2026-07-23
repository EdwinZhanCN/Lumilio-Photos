import { describe, expect, it } from "vite-plus/test";
import {
  ASPECT_PRESETS,
  MIN_CROP_SIZE,
  createDefaultCropRect,
  getAspectRatio,
  moveCropRect,
  resizeCropRect,
  type CropBounds,
} from "./cropMath";

const bounds: CropBounds = { width: 4000, height: 3000 };

describe("getAspectRatio", () => {
  it("resolves numeric, free, and original presets", () => {
    expect(getAspectRatio("1:1", 4000 / 3000)).toBe(1);
    expect(getAspectRatio("16:9", 4000 / 3000)).toBeCloseTo(16 / 9);
    expect(getAspectRatio("free", 4000 / 3000)).toBeNull();
    expect(getAspectRatio("original", 4000 / 3000)).toBeCloseTo(4000 / 3000);
  });

  it("has Free as the first preset", () => {
    expect(ASPECT_PRESETS[0].key).toBe("free");
  });
});

describe("createDefaultCropRect", () => {
  it("fills the whole frame when free", () => {
    expect(createDefaultCropRect(bounds, null)).toEqual({
      x: 0,
      y: 0,
      width: 4000,
      height: 3000,
    });
  });

  it("centers the largest square for a 1:1 crop", () => {
    const rect = createDefaultCropRect(bounds, 1);
    expect(rect.width).toBe(3000);
    expect(rect.height).toBe(3000);
    expect(rect.x).toBe(500); // centered horizontally
    expect(rect.y).toBe(0);
  });
});

describe("moveCropRect", () => {
  it("translates and clamps inside the bounds", () => {
    const rect = { x: 100, y: 100, width: 1000, height: 800 };
    expect(moveCropRect(rect, bounds, 50, -30)).toMatchObject({ x: 150, y: 70 });
    // Clamped at the edge, never past it.
    expect(moveCropRect(rect, bounds, -500, 0).x).toBe(0);
    expect(moveCropRect(rect, bounds, 5000, 0).x).toBe(bounds.width - rect.width);
  });
});

describe("resizeCropRect", () => {
  it("free-resizes from a corner and honors the minimum size", () => {
    const rect = { x: 1000, y: 1000, width: 1000, height: 1000 };
    const resized = resizeCropRect(rect, "se", { x: 1600, y: 1400 }, bounds, null);
    expect(resized).toEqual({ x: 1000, y: 1000, width: 600, height: 400 });

    // Dragging the SE handle onto the NW corner collapses to the minimum.
    const tiny = resizeCropRect(rect, "se", { x: 1000, y: 1000 }, bounds, null);
    expect(tiny.width).toBe(MIN_CROP_SIZE);
    expect(tiny.height).toBe(MIN_CROP_SIZE);
  });

  it("keeps the aspect ratio when locked", () => {
    const rect = createDefaultCropRect(bounds, 1); // 3000×3000 square
    const resized = resizeCropRect(rect, "se", { x: 2000, y: 3000 }, bounds, 1);
    expect(resized.width).toBeCloseTo(resized.height); // still square
    expect(resized.width).toBeLessThanOrEqual(3000);
  });
});
