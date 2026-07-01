/**
 * OffscreenCanvas renderers for the two EXIF-driven border styles.
 *
 * WORKER-SAFE: uses only `OffscreenCanvas`, `createImageBitmap`, and the
 * formatted EXIF / pre-rasterized logo passed in via tool params. No DOM, no
 * SVG parsing, no wasm. Runs inside `tool.worker.ts`.
 *
 * - FROSTED_INFO: existing frosted look (blurred cover background + centered
 *   rounded photo) with a centered EXIF caption in the bottom margin. The
 *   caption color adapts to the (variable) frosted background luminance.
 * - INFO_STRIP:  a clean white strip below the photo — camera model on the
 *   left, brand logo (or text fallback) + divider + params/date on the right.
 */

import { cameraLabel, shootingParams, type BorderExif } from "./exifInfo";
import { canvasToPngBytes, clamp, isOffscreenCanvasSupported, roundRectPath } from "./canvasUtils";

export type ExifBorderMode = "FROSTED_INFO" | "INFO_STRIP";

export interface ExifBorderStyle {
  blurSigma: number;
  brightness: number;
  cornerRadius: number;
}

export interface ExifBorderInput {
  mode: ExifBorderMode;
  style: ExifBorderStyle;
  exif: BorderExif;
  /** Pre-rasterized brand logo (Info Strip), or null to use brandText. */
  logo: ImageBitmap | null;
  /** Brand wordmark, used by Frosted Info and as the Info Strip fallback. */
  brandText: string | null;
}

const FONT_STACK =
  "-apple-system, system-ui, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif";

/** Contain `src` within `box`, preserving aspect ratio. */
function containSize(
  srcW: number,
  srcH: number,
  boxW: number,
  boxH: number,
): { w: number; h: number } {
  const scale = Math.min(boxW / srcW, boxH / srcH);
  return { w: Math.max(1, Math.round(srcW * scale)), h: Math.max(1, Math.round(srcH * scale)) };
}

/** Mean perceptual luminance (0..1) of a canvas region, coarsely sampled. */
function regionLuminance(
  ctx: OffscreenCanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
): number {
  const sx = Math.max(0, Math.floor(x));
  const sy = Math.max(0, Math.floor(y));
  const sw = Math.max(1, Math.floor(w));
  const sh = Math.max(1, Math.floor(h));
  const data = ctx.getImageData(sx, sy, sw, sh).data;
  // Step across pixels so large bands stay cheap.
  const stepX = Math.max(1, Math.floor(sw / 64));
  const stepY = Math.max(1, Math.floor(sh / 64));
  let sum = 0;
  let count = 0;
  for (let py = 0; py < sh; py += stepY) {
    for (let px = 0; px < sw; px += stepX) {
      const i = (py * sw + px) * 4;
      const r = data[i] / 255;
      const g = data[i + 1] / 255;
      const b = data[i + 2] / 255;
      sum += 0.2126 * r + 0.7152 * g + 0.0722 * b;
      count += 1;
    }
  }
  return count > 0 ? sum / count : 0.5;
}

/** Draw `text` left-anchored, truncating with an ellipsis past `maxWidth`. */
function fillTextTruncated(
  ctx: OffscreenCanvasRenderingContext2D,
  text: string,
  x: number,
  y: number,
  maxWidth: number,
): void {
  if (ctx.measureText(text).width <= maxWidth) {
    ctx.fillText(text, x, y);
    return;
  }
  const ellipsis = "…";
  let lo = 0;
  let hi = text.length;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    const candidate = text.slice(0, mid) + ellipsis;
    if (ctx.measureText(candidate).width <= maxWidth) lo = mid;
    else hi = mid - 1;
  }
  ctx.fillText(text.slice(0, lo) + ellipsis, x, y);
}

// ---------------------------------------------------------------------------
// FROSTED_INFO
// ---------------------------------------------------------------------------
function renderFrostedInfo(
  photo: ImageBitmap,
  style: ExifBorderStyle,
  exif: BorderExif,
  brandText: string | null,
): OffscreenCanvas {
  const W = photo.width;
  const H = photo.height;
  const canvas = new OffscreenCanvas(W, H);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to acquire OffscreenCanvas 2d context");

  // --- Frosted cover background (blurred, slightly overscanned) ---
  const sigma = clamp(style.blurSigma, 0, 100);
  ctx.save();
  ctx.filter = sigma > 0.01 ? `blur(${sigma}px)` : "none";
  const coverScale = Math.max(W / photo.width, H / photo.height) * 1.08;
  const coverW = photo.width * coverScale;
  const coverH = photo.height * coverScale;
  ctx.drawImage(photo, (W - coverW) / 2, (H - coverH) / 2, coverW, coverH);
  ctx.restore();

  // --- Brightness wash (also lifts caption contrast) ---
  const b = clamp(style.brightness, -100, 100);
  if (b < 0) {
    ctx.fillStyle = `rgba(0,0,0,${Math.min(0.78, (-b / 100) * 0.8)})`;
    ctx.fillRect(0, 0, W, H);
  } else if (b > 0) {
    ctx.fillStyle = `rgba(255,255,255,${Math.min(0.6, (b / 100) * 0.6)})`;
    ctx.fillRect(0, 0, W, H);
  }

  // --- Foreground photo card ---
  const textBand = Math.round(H * 0.16);
  const marginX = Math.round(W * 0.07);
  const marginTop = Math.round(H * 0.06);
  const boxW = W - 2 * marginX;
  const boxH = H - marginTop - textBand;
  const fg = containSize(photo.width, photo.height, boxW, boxH);
  const fgX = Math.round((W - fg.w) / 2);
  const fgY = marginTop + Math.round((boxH - fg.h) / 2);
  const radius = (clamp(style.cornerRadius, 0, 100) / 100) * Math.min(fg.w, fg.h) * 0.16;

  // Soft drop shadow behind the card.
  ctx.save();
  ctx.shadowColor = "rgba(0,0,0,0.35)";
  ctx.shadowBlur = Math.round(W * 0.012);
  ctx.shadowOffsetY = Math.round(H * 0.004);
  roundRectPath(ctx, fgX, fgY, fg.w, fg.h, radius);
  ctx.fillStyle = "#000";
  ctx.fill();
  ctx.restore();

  ctx.save();
  roundRectPath(ctx, fgX, fgY, fg.w, fg.h, radius);
  ctx.clip();
  ctx.drawImage(photo, fgX, fgY, fg.w, fg.h);
  ctx.restore();

  // --- Caption (centered in the bottom band) ---
  const bandTop = H - textBand;
  const luminance = regionLuminance(ctx, 0, bandTop, W, textBand);
  const dark = luminance > 0.55;
  const primary = dark ? "rgba(20,20,22,0.96)" : "rgba(248,248,250,0.97)";
  const secondary = dark ? "rgba(20,20,22,0.66)" : "rgba(244,244,248,0.82)";

  const brandSize = Math.round(W * 0.0265);
  const paramSize = Math.round(W * 0.0225);
  const lineGap = Math.round(brandSize * 0.55);
  const blockH = brandSize + lineGap + paramSize;
  const blockTop = bandTop + Math.round((textBand - blockH) / 2);
  const cx = W / 2;

  ctx.save();
  ctx.shadowColor = dark ? "rgba(255,255,255,0.25)" : "rgba(0,0,0,0.45)";
  ctx.shadowBlur = Math.round(W * 0.004);
  ctx.textBaseline = "alphabetic";
  ctx.textAlign = "left";

  // Line 1: brand (bold italic) + model (regular).
  const label = cameraLabel(exif);
  const segments: Array<{ text: string; font: string; color: string }> = [];
  if (brandText) {
    segments.push({
      text: brandText,
      font: `italic 700 ${brandSize}px ${FONT_STACK}`,
      color: primary,
    });
  }
  if (label) {
    segments.push({
      text: (brandText ? "  " : "") + label,
      font: `500 ${brandSize}px ${FONT_STACK}`,
      color: primary,
    });
  }
  if (segments.length > 0) {
    let total = 0;
    for (const s of segments) {
      ctx.font = s.font;
      total += ctx.measureText(s.text).width;
    }
    let x = cx - total / 2;
    const y1 = blockTop + brandSize;
    for (const s of segments) {
      ctx.font = s.font;
      ctx.fillStyle = s.color;
      ctx.fillText(s.text, x, y1);
      x += ctx.measureText(s.text).width;
    }
  }

  // Line 2: shooting parameters.
  const paramLine = shootingParams(exif).join("   ");
  if (paramLine) {
    ctx.font = `500 ${paramSize}px ${FONT_STACK}`;
    ctx.fillStyle = secondary;
    ctx.textAlign = "center";
    const y2 = blockTop + brandSize + lineGap + paramSize;
    ctx.fillText(paramLine, cx, y2);
  }
  ctx.restore();

  return canvas;
}

// ---------------------------------------------------------------------------
// INFO_STRIP
// ---------------------------------------------------------------------------
function renderInfoStrip(
  photo: ImageBitmap,
  exif: BorderExif,
  logo: ImageBitmap | null,
  brandText: string | null,
): OffscreenCanvas {
  const W = photo.width;
  const H = photo.height;
  const stripH = Math.round(clamp(W * 0.086, 84, H * 0.3));
  const canvas = new OffscreenCanvas(W, H + stripH);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to acquire OffscreenCanvas 2d context");

  ctx.drawImage(photo, 0, 0, W, H);

  // White strip + hairline separator.
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, H, W, stripH);
  ctx.fillStyle = "rgba(0,0,0,0.06)";
  ctx.fillRect(0, H, W, Math.max(1, Math.round(stripH * 0.012)));

  const padX = Math.round(stripH * 0.5);
  const cy = H + stripH / 2;

  // --- Right block: [logo|brand] | params / date ---
  const paramSize = Math.round(stripH * 0.27);
  const dateSize = Math.round(stripH * 0.2);
  const paramLine = shootingParams(exif).join("   ");
  const dateLine = exif.dateTime ?? "";

  ctx.font = `600 ${paramSize}px ${FONT_STACK}`;
  const paramW = ctx.measureText(paramLine).width;
  ctx.font = `400 ${dateSize}px ${FONT_STACK}`;
  const dateW = dateLine ? ctx.measureText(dateLine).width : 0;
  const textBlockW = Math.max(paramW, dateW);
  const rightEdge = W - padX;
  const textLeft = rightEdge - textBlockW;

  ctx.textAlign = "left";
  ctx.textBaseline = "middle";
  if (dateLine) {
    const half = Math.round(stripH * 0.16);
    ctx.font = `600 ${paramSize}px ${FONT_STACK}`;
    ctx.fillStyle = "#26262a";
    ctx.fillText(paramLine, textLeft, cy - half);
    ctx.font = `400 ${dateSize}px ${FONT_STACK}`;
    ctx.fillStyle = "#76767c";
    ctx.fillText(dateLine, textLeft, cy + half);
  } else {
    ctx.font = `600 ${paramSize}px ${FONT_STACK}`;
    ctx.fillStyle = "#26262a";
    ctx.fillText(paramLine, textLeft, cy);
  }

  // Divider.
  const dividerGap = Math.round(stripH * 0.24);
  const dividerX = Math.round(textLeft - dividerGap);
  const dividerH = Math.round(stripH * 0.5);
  const dividerW = Math.max(1, Math.round(stripH * 0.012));
  ctx.fillStyle = "rgba(0,0,0,0.14)";
  ctx.fillRect(dividerX, Math.round(cy - dividerH / 2), dividerW, dividerH);

  // Logo (preferred) or brand wordmark, right-anchored to the divider.
  const logoRight = dividerX - dividerGap;
  let brandLeft = logoRight; // left edge of whatever we draw, for collision math
  if (logo) {
    const logoH = Math.round(stripH * 0.46);
    const logoW = Math.round(logoH * (logo.width / logo.height));
    brandLeft = logoRight - logoW;
    ctx.drawImage(logo, brandLeft, Math.round(cy - logoH / 2), logoW, logoH);
  } else if (brandText) {
    const brandSize = Math.round(stripH * 0.3);
    ctx.font = `700 ${brandSize}px ${FONT_STACK}`;
    ctx.fillStyle = "#1a1a1d";
    ctx.textAlign = "right";
    ctx.fillText(brandText, logoRight, cy);
    brandLeft = logoRight - ctx.measureText(brandText).width;
    ctx.textAlign = "left";
  }

  // --- Left block: camera model headline ---
  const label = cameraLabel(exif) ?? brandText ?? "";
  if (label) {
    const modelSize = Math.round(stripH * 0.34);
    const maxLabelW = Math.max(0, brandLeft - padX - Math.round(stripH * 0.3));
    ctx.font = `700 ${modelSize}px ${FONT_STACK}`;
    ctx.fillStyle = "#1a1a1d";
    ctx.textAlign = "left";
    ctx.textBaseline = "middle";
    fillTextTruncated(ctx, label, padX, cy, maxLabelW);
  }

  return canvas;
}

/**
 * Render an EXIF-driven border to PNG bytes. Throws if OffscreenCanvas is
 * unavailable (these modes have no wasm fallback).
 */
export async function renderExifBorder(
  source: Blob,
  input: ExifBorderInput,
  signal: AbortSignal,
): Promise<Uint8Array> {
  if (!isOffscreenCanvasSupported()) {
    throw new Error(
      "This border style needs OffscreenCanvas, which is unavailable in this runtime",
    );
  }
  if (signal.aborted) throw new Error("Operation aborted");

  const photo = await createImageBitmap(source);
  try {
    if (signal.aborted) throw new Error("Operation aborted");
    const canvas =
      input.mode === "FROSTED_INFO"
        ? renderFrostedInfo(photo, input.style, input.exif, input.brandText)
        : renderInfoStrip(photo, input.exif, input.logo, input.brandText);
    if (signal.aborted) throw new Error("Operation aborted");
    return await canvasToPngBytes(canvas);
  } finally {
    photo.close?.();
  }
}
