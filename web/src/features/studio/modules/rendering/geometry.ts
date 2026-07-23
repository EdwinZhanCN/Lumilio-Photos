/**
 * Crop, rotate and flip — the geometric half of composition, applied to the
 * developed photo before the border and layers are drawn.
 *
 * WORKER-SAFE. No DOM.
 *
 * The develop engine renders the whole un-rotated frame; this step takes the
 * (scaled) crop sub-rectangle out of it and orients it. Doing geometry here
 * rather than before develop keeps the GPU pipeline working on a stable,
 * full-frame texture — a crop or a 90° turn changes nothing the color math
 * depends on, so it must not force a texture re-upload.
 *
 * The output is always a fresh (or reused) OffscreenCanvas, upright and sized to
 * the final composed geometry, ready for {@link composeStudioImage}.
 */

import type { SourceRect } from "./coordinateSystem";
import { context2d, createCanvas } from "./canvasUtils";

export type GeometrySource = ImageBitmap | OffscreenCanvas;

export type GeometryOptions = {
  /** Region of the source that survives, in SOURCE pixels, or null for the whole frame. */
  crop: SourceRect | null;
  rotation: number;
  flipHorizontal: boolean;
  flipVertical: boolean;
  /** developedPixels / sourcePixels — maps the source-space crop into the developed frame. */
  scale: number;
};

/** True when the options would not move a single pixel. */
export function isGeometryIdentity(options: GeometryOptions): boolean {
  const angle = ((options.rotation % 360) + 360) % 360;
  return angle === 0 && !options.flipHorizontal && !options.flipVertical && options.crop === null;
}

/**
 * Apply `options` to `developed`, drawing into `reuse` when its size already
 * matches so a dragging slider does not allocate a canvas every frame.
 */
export function applyGeometry(
  developed: GeometrySource,
  options: GeometryOptions,
  reuse?: OffscreenCanvas | null,
): OffscreenCanvas {
  const angle = ((options.rotation % 360) + 360) % 360;
  const quarter = angle === 90 || angle === 270;

  // Crop, mapped from source pixels into the developed frame and clamped to it.
  const sx = options.crop ? Math.max(0, Math.round(options.crop.x * options.scale)) : 0;
  const sy = options.crop ? Math.max(0, Math.round(options.crop.y * options.scale)) : 0;
  const sw = options.crop
    ? Math.min(developed.width - sx, Math.max(1, Math.round(options.crop.width * options.scale)))
    : developed.width;
  const sh = options.crop
    ? Math.min(developed.height - sy, Math.max(1, Math.round(options.crop.height * options.scale)))
    : developed.height;

  const outWidth = quarter ? sh : sw;
  const outHeight = quarter ? sw : sh;

  const canvas =
    reuse && reuse.width === outWidth && reuse.height === outHeight
      ? reuse
      : createCanvas(outWidth, outHeight);
  const ctx = context2d(canvas);
  ctx.clearRect(0, 0, outWidth, outHeight);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";

  ctx.save();
  ctx.translate(outWidth / 2, outHeight / 2);
  ctx.rotate((angle * Math.PI) / 180);
  ctx.scale(options.flipHorizontal ? -1 : 1, options.flipVertical ? -1 : 1);
  ctx.drawImage(developed, sx, sy, sw, sh, -sw / 2, -sh / 2, sw, sh);
  ctx.restore();

  return canvas;
}
