import { describe, expect, it } from "vite-plus/test";
import { DEFAULT_STUDIO_ADJUSTMENTS } from "../../model/editTypes";
import { DEFAULT_CANVAS, resolveCanvasGeometry } from "../../model/canvasSpec";
import { deriveRenderSize } from "./coordinateSystem";
import { composedExportDimensions, studioExportSize } from "./exportSize";

describe("final Studio export size", () => {
  const adjustments = {
    ...DEFAULT_STUDIO_ADJUSTMENTS,
    rotation: 90,
    crop: { x: 0, y: 0, width: 1200, height: 800 },
  };
  const canvas = { ...DEFAULT_CANVAS, pad: { top: 0.1, right: 0.1, bottom: 0.1, left: 0.1 } };
  it("estimates the rotated crop with its frame", () => {
    expect(composedExportDimensions(3000, 2000, adjustments, canvas)).toEqual({
      width: 960,
      height: 1360,
    });
  });
  it("bounds the composed image, not just the photo inside it", () => {
    const plan = studioExportSize(
      3000,
      2000,
      adjustments,
      canvas,
      { kind: "longEdge", longEdge: 680 },
      8192,
    );
    const image = deriveRenderSize(3000, 2000, 90, adjustments.crop, plan.renderLongEdge);
    const frame = resolveCanvasGeometry(image.outWidth, image.outHeight, canvas);
    expect({ width: frame.outWidth, height: frame.outHeight }).toEqual({ width: 480, height: 680 });
    expect(plan.downscaled).toBe(false);
  });
  it("reports hardware limits and caps requests at native composition size", () => {
    expect(
      studioExportSize(3000, 2000, adjustments, canvas, { kind: "original" }, 1024),
    ).toMatchObject({ outputLongEdge: 1024, downscaled: true });
    expect(
      studioExportSize(3000, 2000, adjustments, canvas, { kind: "longEdge", longEdge: 9999 }, 8192),
    ).toMatchObject({ outputLongEdge: 1360, downscaled: false });
  });
});
