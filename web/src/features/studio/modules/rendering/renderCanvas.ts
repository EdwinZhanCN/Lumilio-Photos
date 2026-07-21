/**
 * Renders a `CanvasSpec` — the border layer — into an OffscreenCanvas, leaving
 * a surface for layers to be drawn on top of.
 *
 * WORKER-SAFE. No DOM.
 *
 * Draw order is fixed and meaningful:
 *   background -> photo (inner radius) -> scrim -> vignette -> outer radius
 *
 * The scrim sits above the photo but below layers, because its whole job is to
 * darken what text will sit on. The vignette covers the composed frame, not
 * just the photo, so a vignette over a padded canvas dims the margin too.
 *
 * This replaces two disagreeing frosted implementations. The old `FROSTED`
 * blurred the photo at exact canvas size, which samples past the edge and
 * leaves a lighter rim, and hardcoded a 0.75 foreground scale with no inner
 * rounding; the old `FROSTED_INFO` overscanned, used `fitContain`, and rounded
 * the inner corners. `corner_radius` even meant pixels in one and percent in
 * the other. Only the second behaviour survives, and blur is now a fraction of
 * the short edge instead of a pixel count — a fixed pixel blur is a completely
 * different look at preview size than at export size.
 */

import {
  resolveCanvasGeometry,
  type CanvasBackground,
  type CanvasScrim,
  type CanvasSpec,
} from "../../model/canvasSpec";
import {
  angledLinearGradient,
  clamp,
  context2d,
  createCanvas,
  roundRectPath,
} from "./canvasUtils";

export type PhotoSource = ImageBitmap | OffscreenCanvas;

function photoSize(photo: PhotoSource): { width: number; height: number } {
  return { width: photo.width, height: photo.height };
}

function fillBackground(
  ctx: OffscreenCanvasRenderingContext2D,
  background: CanvasBackground,
  photo: PhotoSource,
  outWidth: number,
  outHeight: number,
): void {
  if (background.kind === "frosted") {
    drawFrosted(ctx, background, photo, outWidth, outHeight);
    return;
  }

  if (background.kind === "gradient") {
    const gradient = angledLinearGradient(ctx, background.angle, 0, 0, outWidth, outHeight);
    gradient.addColorStop(0, background.from);
    gradient.addColorStop(1, background.to);
    ctx.fillStyle = gradient;
  } else {
    ctx.fillStyle = background.color;
  }
  ctx.fillRect(0, 0, outWidth, outHeight);
}

/**
 * The photo itself, scaled to cover the output past its bounds, then blurred.
 *
 * `overscan` is not cosmetic. A blur kernel near an edge samples outside the
 * source and the engine treats that as transparent, so a blurred image drawn
 * at exact size fades toward its border and leaves a visible lighter rim.
 * Drawing it larger than the canvas keeps the kernel fed everywhere visible.
 */
function drawFrosted(
  ctx: OffscreenCanvasRenderingContext2D,
  background: Extract<CanvasBackground, { kind: "frosted" }>,
  photo: PhotoSource,
  outWidth: number,
  outHeight: number,
): void {
  const { width: srcW, height: srcH } = photoSize(photo);
  const shortEdge = Math.min(outWidth, outHeight);
  const blurPx = background.blur * shortEdge;

  // Cover the output, then overscan past it.
  const coverScale = Math.max(outWidth / srcW, outHeight / srcH) * background.overscan;
  const drawW = srcW * coverScale;
  const drawH = srcH * coverScale;

  ctx.save();
  ctx.filter = blurPx > 0.5 ? `blur(${blurPx}px)` : "none";
  ctx.drawImage(photo, (outWidth - drawW) / 2, (outHeight - drawH) / 2, drawW, drawH);
  ctx.restore();

  // Brightness as a composite pass rather than a per-pixel loop: the old
  // implementation read the whole frame back with getImageData and added a
  // constant to every channel, which is slower and forces the canvas out of
  // GPU memory.
  //
  // Darkening also becomes multiplicative instead of additive. Subtracting a
  // constant crushes shadows to flat black before it meaningfully dims the
  // highlights; scaling preserves the tonal relationships that make the blur
  // read as an out-of-focus photo rather than a grey wash.
  const brightness = clamp(background.brightness, -1, 1);
  if (brightness !== 0) {
    const level = brightness > 0 ? brightness : 1 + brightness;
    const channel = Math.round(clamp(level, 0, 1) * 255);
    ctx.save();
    ctx.globalCompositeOperation = brightness > 0 ? "lighter" : "multiply";
    ctx.fillStyle = `rgb(${channel},${channel},${channel})`;
    ctx.fillRect(0, 0, outWidth, outHeight);
    ctx.restore();
  }
}

function drawScrim(
  ctx: OffscreenCanvasRenderingContext2D,
  scrim: CanvasScrim,
  x: number,
  y: number,
  width: number,
  height: number,
): void {
  const bandHeight = clamp(scrim.height, 0, 1) * height;
  if (bandHeight <= 0) return;

  const fromTop = scrim.edge === "top";
  const bandY = fromTop ? y : y + height - bandHeight;
  const gradient = ctx.createLinearGradient(
    0,
    fromTop ? bandY + bandHeight : bandY,
    0,
    fromTop ? bandY : bandY + bandHeight,
  );
  gradient.addColorStop(0, scrim.from);
  gradient.addColorStop(1, scrim.to);

  ctx.save();
  ctx.fillStyle = gradient;
  ctx.fillRect(x, bandY, width, bandHeight);
  ctx.restore();
}

/**
 * Radial darkening toward the corners, as a multiply-blended gradient rather
 * than a per-pixel pass. Stops approximate `1 - (d/dMax)^2 * strength`.
 */
function drawVignette(
  ctx: OffscreenCanvasRenderingContext2D,
  strength: number,
  width: number,
  height: number,
): void {
  const amount = clamp(strength, 0, 1);
  if (amount <= 0) return;

  const cx = width / 2;
  const cy = height / 2;
  const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, Math.hypot(cx, cy));
  const STOPS = 16;
  for (let i = 0; i <= STOPS; i += 1) {
    const t = i / STOPS;
    const factor = clamp(1 - t * t * amount, 0, 1);
    const channel = Math.round(factor * 255);
    gradient.addColorStop(t, `rgb(${channel},${channel},${channel})`);
  }

  ctx.save();
  ctx.globalCompositeOperation = "multiply";
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, width, height);
  ctx.restore();
}

/** Keep only the rounded interior, clearing the corners to transparent. */
function clipOuterCorners(
  ctx: OffscreenCanvasRenderingContext2D,
  width: number,
  height: number,
  radius: number,
): void {
  if (radius <= 0) return;
  ctx.save();
  ctx.globalCompositeOperation = "destination-in";
  roundRectPath(ctx, 0, 0, width, height, radius);
  ctx.fillStyle = "#000";
  ctx.fill();
  ctx.restore();
}

export type RenderedCanvas = {
  canvas: OffscreenCanvas;
  width: number;
  height: number;
};

/**
 * Compose `photo` inside `spec`. The result is ready for layers to be drawn on.
 * Callers own the returned canvas.
 */
export function renderCanvasSpec(photo: PhotoSource, spec: CanvasSpec): RenderedCanvas {
  const { width: photoW, height: photoH } = photoSize(photo);
  const geometry = resolveCanvasGeometry(photoW, photoH, spec);
  const { outWidth, outHeight, photoX, photoY } = geometry;

  const canvas = createCanvas(outWidth, outHeight);
  const ctx = context2d(canvas);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";

  fillBackground(ctx, spec.background, photo, outWidth, outHeight);

  const innerRadius = spec.innerRadius * Math.min(photoW, photoH);
  if (innerRadius > 0) {
    ctx.save();
    roundRectPath(ctx, photoX, photoY, photoW, photoH, innerRadius);
    ctx.clip();
    ctx.drawImage(photo, photoX, photoY, photoW, photoH);
    ctx.restore();
  } else {
    ctx.drawImage(photo, photoX, photoY, photoW, photoH);
  }

  if (spec.scrim) {
    drawScrim(ctx, spec.scrim, photoX, photoY, photoW, photoH);
  }

  drawVignette(ctx, spec.vignette, outWidth, outHeight);
  clipOuterCorners(ctx, outWidth, outHeight, spec.outerRadius * Math.min(outWidth, outHeight));

  return { canvas, width: outWidth, height: outHeight };
}
