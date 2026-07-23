import { describe, it, expect } from "vite-plus/test";
import { DEFAULT_STUDIO_ADJUSTMENTS } from "../../model/editTypes";
import { DevelopEngine } from "./developEngine";

/**
 * WebGL2 is a capability: on a GPU/SwiftShader-backed Chromium these run for
 * real, and where no WebGL2 backend exists they skip rather than fail — the
 * point of a capability test is to exercise the capability, not to assert the
 * runner has a GPU.
 */
function webgl2Available(): boolean {
  try {
    return Boolean(new OffscreenCanvas(1, 1).getContext("webgl2"));
  } catch {
    return false;
  }
}

// Real Chromium + WebGL2. These assertions cover the two things typecheck can't:
// that the persistent pipeline actually develops (identity ≈ input, exposure
// brightens) and — critically — that reading the WebGL canvas back with
// drawImage is upright, since the old flip-on-readback is gone.

/** Top half `top`, bottom half `bottom`, so orientation is observable. */
async function splitBitmap(
  top: [number, number, number],
  bottom: [number, number, number],
  size = 64,
): Promise<ImageBitmap> {
  const canvas = new OffscreenCanvas(size, size);
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = `rgb(${top.join(",")})`;
  ctx.fillRect(0, 0, size, size / 2);
  ctx.fillStyle = `rgb(${bottom.join(",")})`;
  ctx.fillRect(0, size / 2, size, size / 2);
  return createImageBitmap(canvas);
}

async function solidBitmap(v: number, size = 64): Promise<ImageBitmap> {
  const canvas = new OffscreenCanvas(size, size);
  const ctx = canvas.getContext("2d")!;
  ctx.fillStyle = `rgb(${v},${v},${v})`;
  ctx.fillRect(0, 0, size, size);
  return createImageBitmap(canvas);
}

/** drawImage(webglCanvas) into a 2D canvas — the exact production read path. */
function readback(src: OffscreenCanvas): (x: number, y: number) => number[] {
  const canvas = new OffscreenCanvas(src.width, src.height);
  const ctx = canvas.getContext("2d")!;
  ctx.drawImage(src, 0, 0);
  const data = ctx.getImageData(0, 0, src.width, src.height);
  return (x, y) => {
    const i = (y * src.width + x) * 4;
    return [data.data[i], data.data[i + 1], data.data[i + 2]];
  };
}

function near(actual: number[], expected: number[], tol = 12): boolean {
  return expected.every((c, i) => Math.abs(actual[i] - c) <= tol);
}

describe.skipIf(!webgl2Available())("DevelopEngine", () => {
  it("develops upright: an identity render keeps top on top", async () => {
    const engine = DevelopEngine.create(await splitBitmap([200, 0, 0], [0, 0, 200]));
    const out = engine.render(DEFAULT_STUDIO_ADJUSTMENTS, 64, 64);
    expect([out.width, out.height]).toEqual([64, 64]);
    const at = readback(out);
    expect(near(at(32, 10), [200, 0, 0])).toBe(true); // top stays red
    expect(near(at(32, 54), [0, 0, 200])).toBe(true); // bottom stays blue
    engine.dispose();
  });

  it("brightens with positive exposure and reports its size", async () => {
    const engine = DevelopEngine.create(await solidBitmap(100));
    const identity = readback(engine.render(DEFAULT_STUDIO_ADJUSTMENTS, 64, 64))(32, 32);
    const brighter = readback(engine.render({ ...DEFAULT_STUDIO_ADJUSTMENTS, exposure: 1 }, 64, 64))(
      32,
      32,
    );
    expect(near(identity, [100, 100, 100], 14)).toBe(true);
    expect(brighter[0]).toBeGreaterThan(identity[0] + 20);
    engine.dispose();
  });

  it("renders at the requested downscaled size", async () => {
    const engine = DevelopEngine.create(await solidBitmap(128));
    const out = engine.render(DEFAULT_STUDIO_ADJUSTMENTS, 32, 24);
    expect([out.width, out.height]).toEqual([32, 24]);
    engine.dispose();
  });

  it("exposes the GPU texture limit for the export guardrail", async () => {
    const engine = DevelopEngine.create(await solidBitmap(16));
    expect(engine.maxTextureSize).toBeGreaterThanOrEqual(2048);
    engine.dispose();
  });
});
