import { describe, it, expect } from "vite-plus/test";
import { applyGeometry, isGeometryIdentity, type GeometryOptions } from "./geometry";

// Runs in real Chromium: applyGeometry uses OffscreenCanvas + drawImage, and
// the whole point of these assertions is to pin down orientation — a mirrored or
// upside-down draw fails a concrete corner check instead of needing an eyeball.

const RED = [255, 0, 0];
const GREEN = [0, 255, 0];
const BLUE = [0, 0, 255];
const YELLOW = [255, 255, 0];

/** A 100×100 canvas with four distinctly colored quadrants (TL/TR/BL/BR). */
function quadrants(size = 100): OffscreenCanvas {
  const canvas = new OffscreenCanvas(size, size);
  const ctx = canvas.getContext("2d")!;
  const h = size / 2;
  ctx.fillStyle = "rgb(255,0,0)";
  ctx.fillRect(0, 0, h, h); // TL red
  ctx.fillStyle = "rgb(0,255,0)";
  ctx.fillRect(h, 0, h, h); // TR green
  ctx.fillStyle = "rgb(0,0,255)";
  ctx.fillRect(0, h, h, h); // BL blue
  ctx.fillStyle = "rgb(255,255,0)";
  ctx.fillRect(h, h, h, h); // BR yellow
  return canvas;
}

function pixel(canvas: OffscreenCanvas, x: number, y: number): number[] {
  const data = canvas.getContext("2d")!.getImageData(x, y, 1, 1).data;
  return [data[0], data[1], data[2]];
}

function near(actual: number[], expected: number[], tol = 24): boolean {
  return expected.every((c, i) => Math.abs(actual[i] - c) <= tol);
}

const base: GeometryOptions = {
  crop: null,
  rotation: 0,
  flipHorizontal: false,
  flipVertical: false,
  scale: 1,
};

describe("isGeometryIdentity", () => {
  it("is true only when nothing moves", () => {
    expect(isGeometryIdentity(base)).toBe(true);
    expect(isGeometryIdentity({ ...base, rotation: 360 })).toBe(true);
    expect(isGeometryIdentity({ ...base, rotation: 90 })).toBe(false);
    expect(isGeometryIdentity({ ...base, flipHorizontal: true })).toBe(false);
    expect(isGeometryIdentity({ ...base, crop: { x: 0, y: 0, width: 1, height: 1 } })).toBe(false);
  });
});

describe("applyGeometry", () => {
  it("keeps every quadrant in place at identity (no accidental flip)", () => {
    const out = applyGeometry(quadrants(), base);
    expect([out.width, out.height]).toEqual([100, 100]);
    expect(near(pixel(out, 10, 10), RED)).toBe(true); // TL
    expect(near(pixel(out, 90, 10), GREEN)).toBe(true); // TR
    expect(near(pixel(out, 10, 90), BLUE)).toBe(true); // BL
    expect(near(pixel(out, 90, 90), YELLOW)).toBe(true); // BR
  });

  it("rotates 90° clockwise: top-left red moves to top-right", () => {
    const out = applyGeometry(quadrants(), { ...base, rotation: 90 });
    expect(near(pixel(out, 90, 10), RED)).toBe(true); // TL -> TR
    expect(near(pixel(out, 90, 90), GREEN)).toBe(true); // TR -> BR
    expect(near(pixel(out, 10, 10), BLUE)).toBe(true); // BL -> TL
  });

  it("flips horizontally: top-left red mirrors to top-right", () => {
    const out = applyGeometry(quadrants(), { ...base, flipHorizontal: true });
    expect(near(pixel(out, 90, 10), RED)).toBe(true);
    expect(near(pixel(out, 10, 10), GREEN)).toBe(true);
  });

  it("flips vertically: top-left red mirrors to bottom-left", () => {
    const out = applyGeometry(quadrants(), { ...base, flipVertical: true });
    expect(near(pixel(out, 10, 90), RED)).toBe(true);
    expect(near(pixel(out, 10, 10), BLUE)).toBe(true);
  });

  it("crops to the top-left quadrant in source pixels", () => {
    const out = applyGeometry(quadrants(), {
      ...base,
      crop: { x: 0, y: 0, width: 50, height: 50 },
    });
    expect([out.width, out.height]).toEqual([50, 50]);
    expect(near(pixel(out, 25, 25), RED)).toBe(true);
  });

  it("maps the crop through scale (developed pixels = source × scale)", () => {
    // Source authored in "source" pixels 200×200; developed frame is 100×100.
    const developed = quadrants(100);
    const out = applyGeometry(developed, {
      ...base,
      scale: 0.5,
      crop: { x: 0, y: 0, width: 100, height: 100 }, // source 100 → developed 50
    });
    expect([out.width, out.height]).toEqual([50, 50]);
    expect(near(pixel(out, 25, 25), RED)).toBe(true);
  });
});
