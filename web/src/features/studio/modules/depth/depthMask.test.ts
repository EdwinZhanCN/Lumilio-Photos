import { describe, expect, it } from "vite-plus/test";
import { buildDepthAlphaMask, depthAlpha } from "./depthMask";

describe("depthAlpha", () => {
  it("shows the layer where the scene is farther than its plane", () => {
    expect(depthAlpha(0.1, 0.5, 0.08)).toBe(1); // far scene → layer visible
  });

  it("hides the layer where the scene is nearer", () => {
    expect(depthAlpha(0.9, 0.5, 0.08)).toBe(0); // near scene → occluded
  });

  it("feathers linearly across the transition band", () => {
    // z=0.5, f=0.2 → lo=0.4, hi=0.6; midpoint 0.5 → half alpha.
    expect(depthAlpha(0.5, 0.5, 0.2)).toBeCloseTo(0.5);
    expect(depthAlpha(0.45, 0.5, 0.2)).toBeCloseTo(0.75);
  });

  it("clamps the feather window to the valid depth range", () => {
    // z near 0: window clamps at 0, so depth 0 stays fully visible.
    expect(depthAlpha(0, 0.02, 0.2)).toBe(1);
  });
});

describe("buildDepthAlphaMask", () => {
  it("returns null when the plane is fully in front", () => {
    expect(buildDepthAlphaMask(new Uint8ClampedArray(4), 1, 1, 1, 0.08)).toBeNull();
  });

  it("writes white pixels with occlusion in the alpha channel", () => {
    // Two pixels: one far (depth 0), one near (depth 255).
    const depth = new Uint8ClampedArray([0, 0, 0, 255, 255, 255, 255, 255]);
    const mask = buildDepthAlphaMask(depth, 2, 1, 0.5, 0)!;
    expect([mask[0], mask[1], mask[2]]).toEqual([255, 255, 255]);
    expect(mask[3]).toBe(255); // far → visible
    expect(mask[7]).toBe(0); // near → hidden
  });
});
