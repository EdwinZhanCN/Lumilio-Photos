/**
 * Depth-based occlusion mask — the "layers sit inside the scene" effect, ported
 * from AfterFrame's `buildDepthAlphaMask`.
 *
 * WORKER-SAFE. Pure over typed arrays, no DOM.
 *
 * A depth field is a grayscale image where 0 = far and 255 = near. A layer has a
 * `zPosition` in 0..1: 1 means always in front (no occlusion), lower sits deeper
 * in the scene. Where the scene depth is NEARER than the layer's plane, the
 * layer is hidden. `feather` softens the transition so the occlusion edge is not
 * a hard cut.
 *
 * The alpha is: 1 (layer shows) where depth ≤ z − f/2, 0 (hidden) where
 * depth ≥ z + f/2, linear between — clamped so the ramp never runs past [0,1].
 */

/** Occlusion alpha (0..1) for one depth sample against a layer plane. */
export function depthAlpha(depth01: number, zPosition: number, feather: number): number {
  const z = Math.max(0, Math.min(1, zPosition));
  const f = Math.max(0, Math.min(0.5, feather));
  const lo = Math.max(0, z - f / 2);
  const hi = Math.min(1, z + f / 2);
  if (depth01 <= lo) return 1;
  if (depth01 >= hi) return 0;
  return 1 - (depth01 - lo) / (hi - lo);
}

/**
 * Build a white RGBA mask (length width*height*4) whose alpha is the occlusion
 * for `zPosition`. `depthRGBA` is a grayscale depth field's pixel bytes
 * (R=G=B=depth). A `zPosition >= 1` needs no mask and returns null.
 */
export function buildDepthAlphaMask(
  depthRGBA: Uint8ClampedArray | Uint8Array,
  width: number,
  height: number,
  zPosition: number,
  feather: number,
): Uint8ClampedArray | null {
  if (zPosition >= 1) return null;
  const out = new Uint8ClampedArray(width * height * 4);
  for (let i = 0; i < width * height; i += 1) {
    const depth = depthRGBA[i * 4] / 255;
    const alpha = depthAlpha(depth, zPosition, feather);
    const p = i * 4;
    out[p] = 255;
    out[p + 1] = 255;
    out[p + 2] = 255;
    out[p + 3] = Math.round(alpha * 255);
  }
  return out;
}
