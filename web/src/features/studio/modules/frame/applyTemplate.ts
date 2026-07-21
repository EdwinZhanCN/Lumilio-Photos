/**
 * Expanding a template against a real photo, including adaptive ink.
 *
 * Overlay templates choose black or white text by what is behind it, which
 * needs the composed background to already exist — but the background comes
 * from the same expansion that produces the text. That circularity is resolved
 * in two passes:
 *
 *   1. Expand with no sampler to obtain the canvas spec. A template's canvas
 *      never depends on its text, so this pass is authoritative for it.
 *   2. Render that canvas, then expand again with a sampler pointed at the
 *      result, yielding layers whose ink matches what they sit on.
 *
 * Non-overlay templates skip the second pass: their ink is declared, so a
 * sample would change nothing.
 */

import { renderCanvasSpec } from "../rendering/renderCanvas";
import { context2d } from "../rendering/canvasUtils";
import { sampleRegionLuminance } from "../rendering/composeStudioImage";
import { expandTemplate, type ExpandedTemplate } from "./expandTemplate";
import type { FrameTemplate } from "./frameTemplate";
import type { FrameExif } from "./frameExif";

export type ApplyTemplateInput = {
  photo: ImageBitmap | OffscreenCanvas;
  exif: FrameExif;
  measureCtx: OffscreenCanvasRenderingContext2D;
  logoColor?: string | null;
};

/**
 * Expand `template` against `photo`, resolving adaptive ink where relevant.
 *
 * The photo is used only for its dimensions and, for overlay templates, its
 * pixels — the returned layers are resolution-independent and apply equally to
 * a preview and a full-resolution export.
 */
export function applyTemplate(
  template: FrameTemplate,
  { photo, exif, measureCtx, logoColor = null }: ApplyTemplateInput,
): ExpandedTemplate {
  const base = {
    photoWidth: photo.width,
    photoHeight: photo.height,
    exif,
    measureCtx,
    logoColor,
  };

  const firstPass = expandTemplate(template, base);
  if (template.family !== "overlay") return firstPass;

  // Render the background once, then sample it for every overlay element.
  const surface = renderCanvasSpec(photo, firstPass.canvas);
  const surfaceCtx = context2d(surface.canvas, { willReadFrequently: true });

  return expandTemplate(template, {
    ...base,
    sampleLuminance: (x, y, width, height) =>
      sampleRegionLuminance(surfaceCtx, x, y, width, height),
  });
}
