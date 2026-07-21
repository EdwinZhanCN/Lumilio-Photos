/**
 * Worker-safe OffscreenCanvas helpers shared by the canvas and layer
 * renderers. No DOM: everything here must run inside the render worker.
 */

export function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

export function isOffscreenCanvasSupported(): boolean {
  return typeof OffscreenCanvas !== "undefined" && typeof createImageBitmap === "function";
}

/** Trace a rounded-rectangle path (manual; avoids relying on ctx.roundRect). */
export function roundRectPath(
  ctx: OffscreenCanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number,
): void {
  const radius = Math.max(0, Math.min(r, w / 2, h / 2));
  ctx.beginPath();
  ctx.moveTo(x + radius, y);
  ctx.arcTo(x + w, y, x + w, y + h, radius);
  ctx.arcTo(x + w, y + h, x, y + h, radius);
  ctx.arcTo(x, y + h, x, y, radius);
  ctx.arcTo(x, y, x + w, y, radius);
  ctx.closePath();
}

export async function canvasToPngBytes(canvas: OffscreenCanvas): Promise<Uint8Array> {
  const blob = await canvas.convertToBlob({ type: "image/png" });
  return new Uint8Array(await blob.arrayBuffer());
}

export function createCanvas(width: number, height: number): OffscreenCanvas {
  return new OffscreenCanvas(Math.max(1, Math.round(width)), Math.max(1, Math.round(height)));
}

export function context2d(
  canvas: OffscreenCanvas,
  options?: CanvasRenderingContext2DSettings,
): OffscreenCanvasRenderingContext2D {
  const ctx = canvas.getContext("2d", options);
  if (!ctx) throw new Error("Failed to acquire an OffscreenCanvas 2d context");
  return ctx;
}

/**
 * A CSS linear-gradient with CSS angle semantics: 0deg points up, 90deg
 * points right. Canvas gradients are defined by two points, so the angle is
 * converted to a vector spanning the box's projected extent.
 */
export function angledLinearGradient(
  ctx: OffscreenCanvasRenderingContext2D,
  angleDeg: number,
  x: number,
  y: number,
  width: number,
  height: number,
): CanvasGradient {
  const radians = (angleDeg * Math.PI) / 180;
  const dx = Math.sin(radians);
  const dy = -Math.cos(radians);
  const halfSpan = (Math.abs(dx) * width + Math.abs(dy) * height) / 2;
  const cx = x + width / 2;
  const cy = y + height / 2;
  return ctx.createLinearGradient(
    cx - dx * halfSpan,
    cy - dy * halfSpan,
    cx + dx * halfSpan,
    cy + dy * halfSpan,
  );
}

/** Apply `color` at `opacity`, accepting both hex and existing rgba() strings. */
export function withOpacity(color: string, opacity: number): string {
  const alpha = clamp(opacity, 0, 1);
  if (alpha >= 1) return color;
  const hex = /^#([0-9a-fA-F]{6})$/.exec(color.trim());
  if (hex) {
    const value = parseInt(hex[1], 16);
    const r = (value >> 16) & 255;
    const g = (value >> 8) & 255;
    const b = value & 255;
    return `rgba(${r},${g},${b},${alpha})`;
  }
  // rgb()/rgba()/transparent — let the engine parse it and scale via globalAlpha
  // at the call site instead of rewriting the string.
  return color;
}
