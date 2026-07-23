import { describe, expect, it } from "vite-plus/test";
import {
  clampToTexture,
  deriveRenderSize,
  displayedFrameSize,
  fitScale,
  mapRectDisplayedToSource,
  mapRectSourceToDisplayed,
  resolveExportSize,
  type SourceRect,
} from "./coordinateSystem";

describe("fitScale", () => {
  it("is 1 when already within budget", () => {
    expect(fitScale(1200, 800, 1800)).toBe(1);
    expect(fitScale(1800, 1000, 1800)).toBe(1);
  });

  it("shrinks the longest edge to the budget", () => {
    expect(fitScale(3600, 2400, 1800)).toBeCloseTo(0.5);
    expect(fitScale(2400, 3600, 1800)).toBeCloseTo(0.5);
  });
});

describe("deriveRenderSize", () => {
  it("renders the full frame and matches out size when unrotated and uncropped", () => {
    const size = deriveRenderSize(3600, 2400, 0, null, 1800);
    expect(size.scale).toBeCloseTo(0.5);
    expect(size.developWidth).toBe(1800);
    expect(size.developHeight).toBe(1200);
    expect(size.outWidth).toBe(1800);
    expect(size.outHeight).toBe(1200);
    expect(size.angle).toBe(0);
  });

  it("swaps out dimensions for a quarter turn but keeps the developed frame upright", () => {
    const size = deriveRenderSize(3600, 2400, 90, null, 1800);
    // Scale fits the rotated bounding box (still 3600×2400 rotated → same longest edge).
    expect(size.developWidth).toBe(1800);
    expect(size.developHeight).toBe(1200);
    expect(size.outWidth).toBe(1200);
    expect(size.outHeight).toBe(1800);
    expect(size.angle).toBe(90);
  });

  it("normalizes negative and over-360 rotations", () => {
    expect(deriveRenderSize(100, 100, -90, null, 1800).angle).toBe(270);
    expect(deriveRenderSize(100, 100, 450, null, 1800).angle).toBe(90);
  });

  it("scales so the cropped region fills the budget, developing the whole frame", () => {
    // A center crop half the size should render at twice the scale of the full frame.
    const crop: SourceRect = { x: 900, y: 600, width: 1800, height: 1200 };
    const size = deriveRenderSize(3600, 2400, 0, crop, 1800);
    expect(size.scale).toBe(1); // rotated crop longest edge 1800 == budget
    expect(size.developWidth).toBe(3600); // whole frame at scale 1
    expect(size.developHeight).toBe(2400);
    expect(size.outWidth).toBe(1800); // only the crop is presented
    expect(size.outHeight).toBe(1200);
  });
});

describe("resolveExportSize", () => {
  it("exports at native long edge when 'original' fits the ceiling", () => {
    const plan = resolveExportSize(6000, 4000, null, { kind: "original" }, 8192);
    expect(plan.nativeLongEdge).toBe(6000);
    expect(plan.maxSize).toBe(6000);
    expect(plan.downscaled).toBe(false);
  });

  it("guardrails 'original' down to the ceiling and flags it", () => {
    const plan = resolveExportSize(12000, 8000, null, { kind: "original" }, 8192);
    expect(plan.maxSize).toBe(8192);
    expect(plan.downscaled).toBe(true);
  });

  it("scales by percent of the native long edge", () => {
    const plan = resolveExportSize(6000, 4000, null, { kind: "percent", percent: 50 }, 8192);
    expect(plan.maxSize).toBe(3000);
    expect(plan.downscaled).toBe(false);
  });

  it("honors an explicit long edge but never upscales past native", () => {
    const within = resolveExportSize(6000, 4000, null, { kind: "longEdge", longEdge: 2000 }, 8192);
    expect(within.maxSize).toBe(2000);
    expect(within.downscaled).toBe(false);

    const beyond = resolveExportSize(6000, 4000, null, { kind: "longEdge", longEdge: 9000 }, 8192);
    expect(beyond.maxSize).toBe(6000); // capped to native, not upscaled
    expect(beyond.downscaled).toBe(true);
  });

  it("uses the crop's long edge as native when cropped", () => {
    const crop: SourceRect = { x: 0, y: 0, width: 3000, height: 2000 };
    const plan = resolveExportSize(6000, 4000, crop, { kind: "original" }, 8192);
    expect(plan.nativeLongEdge).toBe(3000);
    expect(plan.maxSize).toBe(3000);
  });
});

describe("crop coordinate mapping", () => {
  const W = 400;
  const H = 300;

  it("swaps frame dimensions on a quarter turn", () => {
    expect(displayedFrameSize(W, H, 0)).toEqual({ width: 400, height: 300 });
    expect(displayedFrameSize(W, H, 90)).toEqual({ width: 300, height: 400 });
    expect(displayedFrameSize(W, H, 270)).toEqual({ width: 300, height: 400 });
  });

  const cases: Array<[number, boolean, boolean]> = [
    [0, false, false],
    [90, false, false],
    [180, false, false],
    [270, false, false],
    [0, true, false],
    [0, false, true],
    [90, true, false],
    [270, true, true],
  ];
  const rect: SourceRect = { x: 40, y: 30, width: 120, height: 80 };

  it.each(cases)(
    "round-trips source→displayed→source at rotation %i flipH=%s flipV=%s",
    (rotation, flipH, flipV) => {
      const displayed = mapRectSourceToDisplayed(rect, W, H, rotation, flipH, flipV);
      const back = mapRectDisplayedToSource(displayed, W, H, rotation, flipH, flipV);
      expect(back.x).toBeCloseTo(rect.x);
      expect(back.y).toBeCloseTo(rect.y);
      expect(back.width).toBeCloseTo(rect.width);
      expect(back.height).toBeCloseTo(rect.height);
    },
  );

  it("moves a top-left source rect toward the top-right on a 90° turn", () => {
    // 90° clockwise: the source top edge becomes the displayed right edge.
    const displayed = mapRectSourceToDisplayed(
      { x: 0, y: 0, width: 100, height: 60 },
      W,
      H,
      90,
      false,
      false,
    );
    // Displayed frame is 300×400; the mapped rect should sit at its right side.
    expect(displayed.x + displayed.width).toBeCloseTo(300);
    expect(displayed.y).toBeCloseTo(0);
  });
});

describe("clampToTexture", () => {
  it("leaves a source that fits untouched", () => {
    expect(clampToTexture(4000, 3000, 8192)).toEqual({ width: 4000, height: 3000, clamped: false });
  });

  it("shrinks an oversized source to the texture limit, preserving aspect", () => {
    const r = clampToTexture(12000, 8000, 4096);
    expect(r.clamped).toBe(true);
    expect(Math.max(r.width, r.height)).toBe(4096);
    expect(r.width / r.height).toBeCloseTo(1.5);
  });
});
