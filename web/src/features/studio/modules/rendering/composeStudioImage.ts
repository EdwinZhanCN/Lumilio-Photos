/**
 * The composition stage: canvas treatment plus layers, applied to a developed
 * photo.
 *
 * WORKER-SAFE. No DOM.
 *
 * This runs AFTER adjustments, on their result. Composition is never baked
 * into the develop pipeline, which is what keeps both halves independently
 * editable: changing exposure re-renders the photo without disturbing a
 * caption's position, and moving a caption does not re-run the GPU pipeline.
 *
 * The old Border tool worked the other way round — it exported the developed
 * image, ran a one-shot tool over it, and held the result as an opaque blob, so
 * any adjustment change had to discard the border entirely.
 */

import { isCanvasActive, type CanvasSpec } from "../../model/canvasSpec";
import type { Layer } from "../../model/layers";
import { context2d } from "./canvasUtils";
import { renderCanvasSpec } from "./renderCanvas";
import { drawLayers, type LogoImages } from "./renderLayers";

export type Composition = {
  canvas: CanvasSpec | null;
  layers: readonly Layer[];
};

/**
 * Compose `photo` and return the result.
 *
 * Returns the input untouched when there is nothing to do, so an edit with no
 * composition costs no extra canvas allocation or copy.
 */
export function composeStudioImage(
  photo: OffscreenCanvas,
  composition: Composition,
  logos: LogoImages,
): OffscreenCanvas {
  const hasCanvas = isCanvasActive(composition.canvas);
  const hasLayers = composition.layers.length > 0;
  if (!hasCanvas && !hasLayers) return photo;

  if (!composition.canvas) {
    // Layers with no canvas treatment draw straight onto the photo.
    drawLayers(context2d(photo), photo.width, photo.height, composition.layers, logos);
    return photo;
  }

  const framed = renderCanvasSpec(photo, composition.canvas);
  if (hasLayers) {
    drawLayers(
      context2d(framed.canvas),
      framed.width,
      framed.height,
      composition.layers,
      logos,
    );
  }
  return framed.canvas;
}

/**
 * Average luminance (0..1) of a region, for choosing legible ink over a photo.
 *
 * Samples every fourth pixel — enough to pick black or white, and cheap enough
 * to call once per overlay text element while expanding a template.
 */
export function sampleRegionLuminance(
  ctx: OffscreenCanvasRenderingContext2D,
  x: number,
  y: number,
  width: number,
  height: number,
): number {
  const left = Math.max(0, Math.round(x));
  const top = Math.max(0, Math.round(y));
  const right = Math.min(ctx.canvas.width, Math.round(x + width));
  const bottom = Math.min(ctx.canvas.height, Math.round(y + height));
  const w = right - left;
  const h = bottom - top;
  if (w <= 0 || h <= 0) return 0.3;

  let data: Uint8ClampedArray;
  try {
    data = ctx.getImageData(left, top, w, h).data;
  } catch {
    // A tainted or zero-sized surface: assume mid-dark, which is what the
    // scrim under overlay text is there to guarantee anyway.
    return 0.3;
  }

  let sum = 0;
  let count = 0;
  for (let i = 0; i < data.length; i += 16) {
    sum += 0.299 * data[i] + 0.587 * data[i + 1] + 0.114 * data[i + 2];
    count += 1;
  }
  return count ? sum / count / 255 : 0.3;
}
