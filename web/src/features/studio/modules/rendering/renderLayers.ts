/**
 * The one layer renderer. Text and logos, drawn onto any 2d context.
 *
 * WORKER-SAFE. No DOM. Fonts must already be loaded — see
 * `fonts/loadStudioFonts` — because an unloaded face measures as a fallback and
 * silently misplaces every line.
 *
 * A layer's `x`/`y` is its VISUAL CENTER, and `align` only controls how lines
 * sit relative to each other inside a multi-line block. Anchoring text flush to
 * a margin is the template expander's job: it measures, converts an edge anchor
 * into a center, and emits an ordinary layer.
 *
 * That split is deliberate. AfterFrame keeps `align` as a render-time concern,
 * so `drawLayers` offsets by half the measured width, `buildFrameLayers`
 * pre-compensates in the other direction, and `generatePresetLayers` then
 * re-derives the center from a second, DOM-based measurement because the first
 * one disagreed. Three places doing width math to place one line. Here the
 * renderer never asks where a layer "should" be — it is told.
 */

import {
  displayText,
  type Layer,
  type LayerFill,
  type LayerShadow,
  type LogoLayer,
  type TextLayer,
} from "../../model/layers";
import { resolveFontFamily, resolveFontWeight } from "../../model/fonts";
import { angledLinearGradient, clamp, withOpacity } from "./canvasUtils";
import { cssFontShorthand } from "./fonts/loadStudioFonts";

/** Identifies a rasterized logo. Tinting happens before rasterization. */
export function logoKey(brand: string, variant: string, color: string | null): string {
  return `${brand}:${variant}:${color ?? "original"}`;
}

export function logoKeyForLayer(layer: LogoLayer): string {
  return logoKey(layer.brand, layer.variant, layer.color);
}

export type LogoImages = ReadonlyMap<string, ImageBitmap>;

type Ctx = OffscreenCanvasRenderingContext2D;

/** Resolved text geometry in output pixels. */
export type TextMetrics = {
  lines: string[];
  lineWidths: number[];
  /** Widest line. */
  width: number;
  /** Full block height (line box times line count). */
  height: number;
  fontPx: number;
  lineHeightPx: number;
};

function applyTextFont(ctx: Ctx, layer: TextLayer, outWidth: number): number {
  const text = displayText(layer);
  const family = resolveFontFamily(layer.font.family, text);
  const weight = resolveFontWeight(family, layer.font.weight);
  const fontPx = layer.font.size * outWidth;
  ctx.font = cssFontShorthand(family, weight, layer.font.italic, fontPx);
  // measureText accounts for letterSpacing, so setting it before measuring
  // keeps measurement and drawing on the same basis.
  ctx.letterSpacing = `${layer.font.tracking * fontPx}px`;
  return fontPx;
}

/**
 * Measure a text layer against `ctx`. The caller must use the SAME context it
 * will draw with — that identity is the whole reason this is not a standalone
 * utility.
 */
export function measureTextLayer(ctx: Ctx, layer: TextLayer, outWidth: number): TextMetrics {
  ctx.save();
  const fontPx = applyTextFont(ctx, layer, outWidth);
  const lines = displayText(layer).split("\n");
  const lineWidths = lines.map((line) => (line ? ctx.measureText(line).width : 0));
  ctx.restore();

  const lineHeightPx = fontPx * layer.font.lineHeight;
  return {
    lines,
    lineWidths,
    width: lineWidths.length ? Math.max(...lineWidths) : 0,
    height: lineHeightPx * lines.length,
    fontPx,
    lineHeightPx,
  };
}

function fillStyleFor(
  ctx: Ctx,
  fill: LayerFill,
  x: number,
  y: number,
  width: number,
  height: number,
): string | CanvasGradient {
  if (fill.kind === "gradient") {
    const gradient = angledLinearGradient(ctx, fill.angle, x, y, width, height);
    gradient.addColorStop(0, withOpacity(fill.from, fill.fromOpacity));
    gradient.addColorStop(1, withOpacity(fill.to, fill.toOpacity));
    return gradient;
  }
  return withOpacity(fill.color, fill.opacity);
}

function applyShadow(ctx: Ctx, shadow: LayerShadow | null, outWidth: number): void {
  if (!shadow) return;
  ctx.shadowColor = withOpacity(shadow.color, shadow.opacity);
  ctx.shadowBlur = shadow.blur * outWidth;
  ctx.shadowOffsetX = shadow.offsetX * outWidth;
  ctx.shadowOffsetY = shadow.offsetY * outWidth;
}

function clearShadow(ctx: Ctx): void {
  ctx.shadowColor = "transparent";
  ctx.shadowBlur = 0;
  ctx.shadowOffsetX = 0;
  ctx.shadowOffsetY = 0;
}

/** Horizontal offset of line `i` from the block's center, per `align`. */
function lineOffsetX(align: TextLayer["align"], blockWidth: number, lineWidth: number): number {
  if (align === "left") return -blockWidth / 2;
  if (align === "right") return blockWidth / 2 - lineWidth;
  return -lineWidth / 2;
}

function drawTextLayer(
  ctx: Ctx,
  layer: TextLayer,
  outWidth: number,
  outHeight: number,
): void {
  const metrics = measureTextLayer(ctx, layer, outWidth);
  if (!metrics.width && !layer.background) return;

  const centerX = layer.x * outWidth;
  const centerY = layer.y * outHeight;

  ctx.save();
  ctx.translate(centerX, centerY);
  if (layer.rotation) ctx.rotate((layer.rotation * Math.PI) / 180);
  ctx.globalAlpha = clamp(layer.opacity, 0, 1);

  applyTextFont(ctx, layer, outWidth);
  ctx.textAlign = "left";
  ctx.textBaseline = "middle";

  const { lines, lineWidths, width: blockWidth, lineHeightPx } = metrics;
  const blockHeight = metrics.height;
  // Baseline of line i, measured from the block's vertical center.
  const baselineY = (i: number) => (i - (lines.length - 1) / 2) * lineHeightPx;

  if (layer.background) {
    const pad = layer.background.padding;
    const fontPx = metrics.fontPx;
    const padTop = pad.top * fontPx;
    const padRight = pad.right * fontPx;
    const padBottom = pad.bottom * fontPx;
    const padLeft = pad.left * fontPx;
    const bgX = -blockWidth / 2 - padLeft;
    const bgY = -blockHeight / 2 - padTop;
    const bgW = blockWidth + padLeft + padRight;
    const bgH = blockHeight + padTop + padBottom;
    ctx.save();
    applyShadow(ctx, layer.shadow, outWidth);
    ctx.fillStyle = fillStyleFor(ctx, layer.background.fill, bgX, bgY, bgW, bgH);
    ctx.fillRect(bgX, bgY, bgW, bgH);
    ctx.restore();
  }

  // Stroke and fill are composited on a scratch canvas, then blitted once with
  // the shadow enabled. Drawing them directly would cast the shadow from the
  // stroke ring alone — a hollow silhouette — instead of from the merged glyph
  // shape, which is what CSS `text-shadow` with `paint-order` produces.
  const strokeWidthPx = layer.stroke ? layer.stroke.width * outWidth : 0;
  const glyphOverflow = metrics.fontPx * 0.6;
  const padX = Math.ceil(strokeWidthPx + 4);
  const padY = Math.ceil(strokeWidthPx + 4 + glyphOverflow);
  const scratchW = Math.ceil(blockWidth + padX * 2);
  const scratchH = Math.ceil((lines.length - 1) * lineHeightPx + padY * 2);

  const scratch = new OffscreenCanvas(Math.max(1, scratchW), Math.max(1, scratchH));
  const sctx = scratch.getContext("2d");
  if (!sctx) {
    ctx.restore();
    return;
  }
  applyTextFont(sctx, layer, outWidth);
  sctx.textAlign = "left";
  sctx.textBaseline = "middle";

  const scratchCenterX = scratchW / 2;
  const scratchCenterY = scratchH / 2;

  if (layer.stroke && strokeWidthPx > 0) {
    // Canvas strokes straddle the path, so half the width eats into the glyph.
    // Doubling keeps the visible outside weight equal to the requested width.
    sctx.strokeStyle = layer.stroke.color;
    sctx.lineWidth = strokeWidthPx * 2;
    sctx.lineJoin = "round";
    lines.forEach((line, i) => {
      if (!line) return;
      const x = scratchCenterX + lineOffsetX(layer.align, blockWidth, lineWidths[i]);
      sctx.strokeText(line, x, scratchCenterY + baselineY(i));
    });
  }

  sctx.fillStyle = fillStyleFor(
    sctx,
    layer.fill,
    scratchCenterX - blockWidth / 2,
    scratchCenterY - blockHeight / 2,
    blockWidth,
    blockHeight,
  );
  lines.forEach((line, i) => {
    if (!line) return;
    const x = scratchCenterX + lineOffsetX(layer.align, blockWidth, lineWidths[i]);
    sctx.fillText(line, x, scratchCenterY + baselineY(i));
  });

  // Canvas has no text-decoration; draw the rules with the same fill so a
  // gradient carries through them. Offsets mirror the CSS the preview uses.
  if (layer.underline || layer.strikethrough) {
    const thickness = Math.max(1, metrics.fontPx * 0.04);
    lineWidths.forEach((lineWidth, i) => {
      if (!lineWidth) return;
      const x = scratchCenterX + lineOffsetX(layer.align, blockWidth, lineWidth);
      const y = scratchCenterY + baselineY(i);
      if (layer.underline) {
        sctx.fillRect(x, y + metrics.fontPx * 0.31, lineWidth, thickness);
      }
      if (layer.strikethrough) {
        sctx.fillRect(x, y - metrics.fontPx * 0.05 - thickness / 2, lineWidth, thickness);
      }
    });
  }

  applyShadow(ctx, layer.shadow, outWidth);
  ctx.drawImage(scratch, -scratchCenterX, -scratchCenterY);
  clearShadow(ctx);
  ctx.restore();
}

function drawLogoLayer(
  ctx: Ctx,
  layer: LogoLayer,
  outWidth: number,
  outHeight: number,
  logos: LogoImages,
): void {
  const image = logos.get(logoKeyForLayer(layer));
  if (!image) return;

  const widthPx = layer.size * outWidth;
  const heightPx = image.height > 0 ? (widthPx * image.height) / image.width : widthPx;

  ctx.save();
  ctx.translate(layer.x * outWidth, layer.y * outHeight);
  if (layer.rotation) ctx.rotate((layer.rotation * Math.PI) / 180);
  ctx.globalAlpha = clamp(layer.opacity, 0, 1);
  applyShadow(ctx, layer.shadow, outWidth);
  ctx.drawImage(image, -widthPx / 2, -heightPx / 2, widthPx, heightPx);
  clearShadow(ctx);
  ctx.restore();
}

/** Draw layers in order; later layers sit on top. */
export function drawLayers(
  ctx: Ctx,
  outWidth: number,
  outHeight: number,
  layers: readonly Layer[],
  logos: LogoImages,
): void {
  ctx.save();
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  for (const layer of layers) {
    if (layer.type === "text") {
      drawTextLayer(ctx, layer, outWidth, outHeight);
    } else {
      drawLogoLayer(ctx, layer, outWidth, outHeight, logos);
    }
  }
  ctx.restore();
}
