/**
 * Renders a small preview of every template against the current photo.
 *
 * All templates share ONE downscaled base bitmap, drawn once. AfterFrame
 * rebuilds a full-resolution base per template, re-renders, and JPEG-encodes
 * each one, behind a 200 ms debounce to survive the cost — the debounce treats
 * the symptom. At preview size the work is small enough to simply do.
 *
 * Runs on the main thread but touches no DOM: `OffscreenCanvas` throughout, and
 * logos arrive already rasterized.
 */

import { renderCanvasSpec } from "../rendering/renderCanvas";
import { drawLayers, type LogoImages } from "../rendering/renderLayers";
import { context2d, createCanvas } from "../rendering/canvasUtils";
import { applyTemplate } from "./applyTemplate";
import { expandTemplate } from "./expandTemplate";
import type { FrameTemplate } from "./frameTemplate";
import type { FrameExif } from "./frameExif";
import type { LogoRequest } from "./logoRaster";

/** Wide enough to judge a layout, small enough that 21 of them are cheap. */
const PREVIEW_WIDTH = 240;

function downscale(photo: ImageBitmap, width: number): OffscreenCanvas {
  const height = Math.max(1, Math.round((photo.height * width) / photo.width));
  const canvas = createCanvas(width, height);
  const ctx = context2d(canvas);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(photo, 0, 0, width, height);
  return canvas;
}

/**
 * Every mark these templates need, without rendering anything.
 *
 * Called before {@link renderTemplatePreviews} so the caller can rasterize the
 * logos first — expansion needs them to size logo layers correctly.
 */
export function collectTemplateLogos(
  templates: readonly FrameTemplate[],
  exif: FrameExif,
  photoWidth: number,
  photoHeight: number,
): LogoRequest[] {
  const measureCtx = context2d(createCanvas(1, 1));
  const requests: LogoRequest[] = [];
  for (const template of templates) {
    requests.push(
      ...expandTemplate(template, { photoWidth, photoHeight, exif, measureCtx }).logoRequests,
    );
  }
  return requests;
}

export type TemplatePreviewInput = {
  photo: ImageBitmap;
  exif: FrameExif;
  templates: readonly FrameTemplate[];
  logos: LogoImages;
};

/**
 * Preview object URLs keyed by template id.
 *
 * The caller owns the URLs and must revoke them when replacing or discarding
 * the set.
 */
export async function renderTemplatePreviews({
  photo,
  exif,
  templates,
  logos,
}: TemplatePreviewInput): Promise<Map<string, string>> {
  const base = downscale(photo, PREVIEW_WIDTH);
  const measureCtx = context2d(createCanvas(1, 1));
  const urls = new Map<string, string>();

  for (const template of templates) {
    const expanded = applyTemplate(template, { photo: base, exif, measureCtx });
    const framed = renderCanvasSpec(base, expanded.canvas);
    drawLayers(context2d(framed.canvas), framed.width, framed.height, expanded.layers, logos);

    const blob = await framed.canvas.convertToBlob({ type: "image/jpeg", quality: 0.82 });
    urls.set(template.id, URL.createObjectURL(blob));
  }

  return urls;
}
