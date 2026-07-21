import { describe, expect, it } from "vite-plus/test";
import {
  DEFAULT_CANVAS,
  isCanvasActive,
  normalizeCanvasSpec,
  resolveCanvasGeometry,
  type CanvasSpec,
} from "./canvasSpec";

function spec(overrides: Partial<CanvasSpec> = {}): CanvasSpec {
  return { ...DEFAULT_CANVAS, pad: { ...DEFAULT_CANVAS.pad }, ...overrides };
}

describe("resolveCanvasGeometry", () => {
  it("resolves padding against the short edge, not the width", () => {
    // The same spec must give a landscape and a portrait photo the same
    // visual weight. A width basis would make portrait margins look thin.
    const landscape = resolveCanvasGeometry(3000, 2000, spec({ pad: { top: 0, right: 0, bottom: 0.1, left: 0 } }));
    const portrait = resolveCanvasGeometry(2000, 3000, spec({ pad: { top: 0, right: 0, bottom: 0.1, left: 0 } }));

    expect(landscape.padPx.bottom).toBe(200); // 0.1 x 2000 (height is shorter)
    expect(portrait.padPx.bottom).toBe(200); // 0.1 x 2000 (width is shorter)
  });

  it("grows the output by the padding and offsets the photo", () => {
    const geometry = resolveCanvasGeometry(
      1000,
      1000,
      spec({ pad: { top: 0.05, right: 0.1, bottom: 0.2, left: 0.1 } }),
    );
    expect(geometry.outWidth).toBe(1200);
    expect(geometry.outHeight).toBe(1250);
    expect(geometry.photoX).toBe(100);
    expect(geometry.photoY).toBe(50);
  });

  it("leaves the photo untouched with no padding", () => {
    const geometry = resolveCanvasGeometry(800, 600, spec());
    expect(geometry.outWidth).toBe(800);
    expect(geometry.outHeight).toBe(600);
    expect(geometry.photoX).toBe(0);
  });
});

describe("normalizeCanvasSpec", () => {
  it("returns defaults for junk input", () => {
    expect(normalizeCanvasSpec(null)).toEqual(DEFAULT_CANVAS);
    expect(normalizeCanvasSpec("nope")).toEqual(DEFAULT_CANVAS);
  });

  it("keeps each background kind's own fields", () => {
    const frosted = normalizeCanvasSpec({
      background: { kind: "frosted", blur: 0.08, brightness: -0.3, overscan: 1.2 },
    });
    expect(frosted.background).toEqual({
      kind: "frosted",
      blur: 0.08,
      brightness: -0.3,
      overscan: 1.2,
    });

    const gradient = normalizeCanvasSpec({
      background: { kind: "gradient", from: "#000000", to: "#ffffff", angle: 45 },
    });
    expect(gradient.background).toMatchObject({ kind: "gradient", angle: 45 });
  });

  it("falls back to solid for an unknown background kind", () => {
    expect(normalizeCanvasSpec({ background: { kind: "hologram" } }).background).toEqual({
      kind: "solid",
      color: "#ffffff",
    });
  });

  it("rejects malformed colors rather than passing them to the renderer", () => {
    const normalized = normalizeCanvasSpec({
      background: { kind: "solid", color: "javascript:alert(1)" },
    });
    expect(normalized.background).toEqual({ kind: "solid", color: "#ffffff" });
  });

  it("clamps out-of-range geometry", () => {
    const normalized = normalizeCanvasSpec({
      pad: { top: -5, right: 99, bottom: 0.1, left: 0 },
      vignette: 4,
      outerRadius: -1,
    });
    expect(normalized.pad.top).toBe(0);
    expect(normalized.pad.right).toBe(2);
    expect(normalized.vignette).toBe(1);
    expect(normalized.outerRadius).toBe(0);
  });

  it("drops a scrim that is not an object", () => {
    expect(normalizeCanvasSpec({ scrim: "bottom" }).scrim).toBeNull();
    expect(normalizeCanvasSpec({ scrim: { edge: "top", height: 0.4 } }).scrim).toMatchObject({
      edge: "top",
      height: 0.4,
    });
  });
});

describe("isCanvasActive", () => {
  it("is false for null and for a spec that changes nothing", () => {
    expect(isCanvasActive(null)).toBe(false);
    expect(isCanvasActive(spec())).toBe(false);
  });

  it("is true once the spec would alter a pixel", () => {
    expect(isCanvasActive(spec({ pad: { top: 0, right: 0, bottom: 0.1, left: 0 } }))).toBe(true);
    expect(isCanvasActive(spec({ vignette: 0.3 }))).toBe(true);
    expect(isCanvasActive(spec({ outerRadius: 0.02 }))).toBe(true);
  });

  it("ignores a background choice with no padding to fill", () => {
    // A frosted background with zero padding has nowhere to show; treating it
    // as active would allocate a canvas copy for nothing.
    expect(
      isCanvasActive(spec({ background: { kind: "frosted", blur: 0.06, brightness: 0, overscan: 1.1 } })),
    ).toBe(false);
  });
});
