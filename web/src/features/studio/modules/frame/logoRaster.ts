/**
 * Rasterizes and tints brand logos to `ImageBitmap`s for the render worker.
 *
 * MAIN THREAD ONLY. Decoding an SVG needs `Image`/`document`, which a worker
 * does not have, so the worker is handed finished bitmaps through the render
 * message and never touches SVG. Do not import this from worker code.
 */

import { logoKey } from "../rendering/renderLayers";
import { findBrand, pickVariant, type LogoVariant } from "./logoRegistry";

// Raw SVG markup keyed by module path. `?raw` keeps it inline so there is no
// second network round-trip and the viewBox can be read before decoding.
const LOGO_SOURCES = import.meta.glob<string>("../../../../assets/logos/*/*.svg", {
  eager: true,
  query: "?raw",
  import: "default",
});

/** "canon/wordmark.svg" -> raw SVG. Keys mirror the manifest's `file` values. */
const SVG_BY_FILE: Record<string, string> = Object.fromEntries(
  Object.entries(LOGO_SOURCES).map(([path, svg]) => {
    const parts = path.split("/");
    return [`${parts[parts.length - 2]}/${parts[parts.length - 1]}`, svg];
  }),
);

/**
 * Rasterize at a generous fixed height and let the renderer scale down. The
 * cache is keyed by brand/variant/color and not by size, so one bitmap has to
 * stay crisp in both a 260 px template thumbnail and a full-resolution export.
 */
const RASTER_HEIGHT = 512;

/**
 * Force explicit pixel dimensions on the root `<svg>`, and synthesize a
 * viewBox when the file lacks one.
 *
 * The synthesized viewBox is not optional: overriding width/height on an SVG
 * authored without one leaves its content in the original coordinate space,
 * outside the new viewport, and it renders blank. Apple's mark is the file in
 * this set that ships without a viewBox.
 */
function sizeSvg(svg: string, width: number, height: number): string {
  return svg.replace(/<svg\b([^>]*)>/i, (_match, attrs: string) => {
    const hasViewBox = /viewBox\s*=/i.test(attrs);
    const widthAttr = /\bwidth\s*=\s*"([\d.]+)/i.exec(attrs);
    const heightAttr = /\bheight\s*=\s*"([\d.]+)/i.exec(attrs);

    const cleaned = attrs
      .replace(/\swidth\s*=\s*"[^"]*"/i, "")
      .replace(/\sheight\s*=\s*"[^"]*"/i, "");

    let viewBox = "";
    if (!hasViewBox && widthAttr && heightAttr) {
      viewBox = ` viewBox="0 0 ${widthAttr[1]} ${heightAttr[1]}"`;
    }
    return `<svg${cleaned}${viewBox} width="${width}" height="${height}">`;
  });
}

/** Decode sized SVG markup. Dimensions must already be baked in by `sizeSvg`. */
async function decodeSvg(svg: string): Promise<HTMLImageElement> {
  const blob = new Blob([svg], { type: "image/svg+xml" });
  const url = URL.createObjectURL(blob);
  try {
    const image = new Image();
    image.decoding = "async";
    image.src = url;
    await image.decode();
    return image;
  } finally {
    // Safe once decode() has resolved — the bitmap is already in memory.
    URL.revokeObjectURL(url);
  }
}

/**
 * Rasterize and tint a variant onto a canvas.
 *
 * The canvas, not an `ImageBitmap`, is what gets cached. Bitmaps handed to the
 * worker are transferred, which detaches them on this side, so a cached bitmap
 * would be an empty husk on the second request. Keeping the canvas lets each
 * request mint a fresh transferable copy from the same decoded pixels.
 */
async function rasterizeVariant(
  variant: LogoVariant,
  color: string | null,
): Promise<HTMLCanvasElement | null> {
  const svg = SVG_BY_FILE[variant.file];
  if (!svg) return null;

  const height = RASTER_HEIGHT;
  const width = Math.max(1, Math.round(height * variant.aspect));

  try {
    const image = await decodeSvg(sizeSvg(svg, width, height));
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return null;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(image, 0, 0, width, height);

    // Recolor the silhouette while preserving anti-aliased alpha edges. Skipped
    // for color-locked marks, which resolve `color` to null.
    if (color) {
      ctx.globalCompositeOperation = "source-in";
      ctx.fillStyle = color;
      ctx.fillRect(0, 0, width, height);
      ctx.globalCompositeOperation = "source-over";
    }
    return canvas;
  } catch {
    return null;
  }
}

export type LogoRequest = {
  brand: string;
  variant: string;
  color: string | null;
};

/** Decoded, tinted canvases keyed by brand/variant/color. */
const cache = new Map<string, Promise<HTMLCanvasElement | null>>();

/**
 * Rasterize every requested mark, keyed the way `renderLayers` looks them up.
 *
 * Each call returns freshly minted bitmaps that the caller may transfer to a
 * worker. Unmatched brands and failed decodes are simply absent from the map;
 * `renderLayers` skips a logo layer with no image rather than drawing a
 * placeholder.
 */
export async function rasterizeLogos(
  requests: readonly LogoRequest[],
): Promise<Map<string, ImageBitmap>> {
  const wanted = new Map<string, LogoRequest>();
  for (const request of requests) {
    wanted.set(logoKey(request.brand, request.variant, request.color), request);
  }

  const entries = await Promise.all(
    Array.from(wanted, async ([key, request]) => {
      let pending = cache.get(key);
      if (!pending) {
        const brand = findBrand(request.brand);
        const variant = pickVariant(brand, { variantId: request.variant, strict: true });
        pending = variant ? rasterizeVariant(variant, request.color) : Promise.resolve(null);
        cache.set(key, pending);
      }
      const canvas = await pending;
      return [key, canvas ? await createImageBitmap(canvas) : null] as const;
    }),
  );

  const images = new Map<string, ImageBitmap>();
  for (const [key, bitmap] of entries) {
    if (bitmap) images.set(key, bitmap);
  }
  return images;
}
