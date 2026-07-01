/**
 * OffscreenCanvas renderers for the non-EXIF border styles
 * (COLORED / VIGNETTE / FROSTED). These replace the former Rust/wasm
 * (`border-wasm`) implementations 1:1 so the whole border tool runs on one
 * canvas pipeline with no wasm dependency.
 *
 * WORKER-SAFE: only OffscreenCanvas / createImageBitmap. Runs in tool.worker.
 */

import { hexToRgb } from "./types";
import {
  canvasToPngBytes,
  clamp,
  clampByte,
  isOffscreenCanvasSupported,
  roundRectPath,
} from "./canvasUtils";

export type BasicBorderMode = "COLORED" | "FROSTED" | "VIGNETTE";

export interface BasicBorderStyle {
  borderWidth: number;
  colorHex: string;
  blurSigma: number;
  brightness: number;
  cornerRadius: number;
  strength: number;
}

const FROSTED_FOREGROUND_SCALE = 0.75; // matches the former wasm implementation
const VIGNETTE_GRADIENT_STOPS = 16;

// ---------------------------------------------------------------------------
// COLORED: solid-color frame of `borderWidth` px around the image.
// ---------------------------------------------------------------------------
function renderColored(photo: ImageBitmap, borderWidth: number, colorHex: string): OffscreenCanvas {
  const bw = Math.max(0, Math.round(borderWidth));
  const W = photo.width;
  const H = photo.height;
  const canvas = new OffscreenCanvas(W + bw * 2, H + bw * 2);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to acquire OffscreenCanvas 2d context");

  const { r, g, b } = hexToRgb(colorHex);
  ctx.fillStyle = `rgb(${r},${g},${b})`;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.drawImage(photo, bw, bw, W, H);
  return canvas;
}

// ---------------------------------------------------------------------------
// VIGNETTE: radial darkening. Faithful to the wasm formula
//   factor = clamp(1 - (dist/cornerDist)^2 * strength, 0, 1); pixel *= factor
// implemented as a multiply-blended radial gradient (GPU, no per-pixel loop).
// ---------------------------------------------------------------------------
function renderVignette(photo: ImageBitmap, strength: number): OffscreenCanvas {
  const W = photo.width;
  const H = photo.height;
  const canvas = new OffscreenCanvas(W, H);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to acquire OffscreenCanvas 2d context");

  ctx.drawImage(photo, 0, 0, W, H);

  const s = clamp(strength, 0, 1); // wasm clamps strength to [0,1] internally
  const cx = W / 2;
  const cy = H / 2;
  const maxDist = Math.hypot(cx, cy); // centre -> corner
  const gradient = ctx.createRadialGradient(cx, cy, 0, cx, cy, maxDist);
  for (let i = 0; i <= VIGNETTE_GRADIENT_STOPS; i += 1) {
    const t = i / VIGNETTE_GRADIENT_STOPS;
    const factor = clamp(1 - t * t * s, 0, 1);
    const v = Math.round(factor * 255);
    gradient.addColorStop(t, `rgb(${v},${v},${v})`);
  }

  ctx.globalCompositeOperation = "multiply";
  ctx.fillStyle = gradient;
  ctx.fillRect(0, 0, W, H);
  ctx.globalCompositeOperation = "source-over";
  return canvas;
}

/** Keep only the rounded-rectangle interior; clear the corners to transparent. */
function clipOuterRoundedCorners(
  ctx: OffscreenCanvasRenderingContext2D,
  W: number,
  H: number,
  radius: number,
): void {
  if (radius <= 0) return;
  ctx.save();
  ctx.globalCompositeOperation = "destination-in";
  roundRectPath(ctx, 0, 0, W, H, radius);
  ctx.fillStyle = "#000";
  ctx.fill();
  ctx.restore();
}

// ---------------------------------------------------------------------------
// FROSTED: blurred + brightened full-frame background with rounded OUTER
// corners, then the image overlaid centered at 0.75 scale (no inner rounding).
// 1:1 with the former wasm `create_frosted_border`.
// ---------------------------------------------------------------------------
function renderFrosted(
  photo: ImageBitmap,
  blurSigma: number,
  brightness: number,
  cornerRadius: number,
): OffscreenCanvas {
  const W = photo.width;
  const H = photo.height;
  const canvas = new OffscreenCanvas(W, H);
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Failed to acquire OffscreenCanvas 2d context");

  // Background: full-frame blur (GPU; canvas blur replaces the wasm downscale
  // -> blur -> upscale speed hack).
  const sigma = Math.max(0, blurSigma);
  ctx.save();
  ctx.filter = sigma > 0.01 ? `blur(${sigma}px)` : "none";
  ctx.drawImage(photo, 0, 0, W, H);
  ctx.restore();

  // Brightness: wasm uses additive brighten() on the background. Replicate
  // exactly with a single pixel pass over the (background-only) canvas.
  const b = Math.round(brightness);
  if (b !== 0) {
    const image = ctx.getImageData(0, 0, W, H);
    const data = image.data;
    for (let i = 0; i < data.length; i += 4) {
      data[i] = clampByte(data[i] + b);
      data[i + 1] = clampByte(data[i + 1] + b);
      data[i + 2] = clampByte(data[i + 2] + b);
    }
    ctx.putImageData(image, 0, 0);
  }

  // Rounded OUTER corners (clamped like the wasm version).
  const radius = Math.min(clamp(cornerRadius, 0, Math.min(W, H) / 2), W / 2, H / 2);
  clipOuterRoundedCorners(ctx, W, H, radius);

  // Foreground: image scaled to 0.75, centered, opaque (no rounding).
  const fgW = Math.max(1, Math.round(W * FROSTED_FOREGROUND_SCALE));
  const fgH = Math.max(1, Math.round(H * FROSTED_FOREGROUND_SCALE));
  const offX = Math.round((W - fgW) / 2);
  const offY = Math.round((H - fgH) / 2);
  ctx.imageSmoothingEnabled = true;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(photo, offX, offY, fgW, fgH);
  return canvas;
}

/**
 * Render a non-EXIF border to PNG bytes. Throws if OffscreenCanvas is
 * unavailable.
 */
export async function renderBasicBorder(
  source: Blob,
  mode: BasicBorderMode,
  style: BasicBorderStyle,
  signal: AbortSignal,
): Promise<Uint8Array> {
  if (!isOffscreenCanvasSupported()) {
    throw new Error("The border tool needs OffscreenCanvas, which is unavailable in this runtime");
  }
  if (signal.aborted) throw new Error("Operation aborted");

  const photo = await createImageBitmap(source);
  try {
    if (signal.aborted) throw new Error("Operation aborted");
    let canvas: OffscreenCanvas;
    if (mode === "COLORED") {
      canvas = renderColored(photo, style.borderWidth, style.colorHex);
    } else if (mode === "VIGNETTE") {
      canvas = renderVignette(photo, style.strength);
    } else {
      canvas = renderFrosted(photo, style.blurSigma, style.brightness, style.cornerRadius);
    }
    if (signal.aborted) throw new Error("Operation aborted");
    return await canvasToPngBytes(canvas);
  } finally {
    photo.close?.();
  }
}
