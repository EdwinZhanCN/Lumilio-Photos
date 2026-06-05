/**
 * Brand-logo matching and rasterization for the Info Strip border.
 *
 * MAIN-THREAD ONLY: rasterization uses `Image` + `<canvas>` + `document`.
 * Do NOT import this from the tool worker. The worker receives an already
 * rasterized `ImageBitmap` (or a fallback brand string) through the tool
 * params, so it never touches the DOM or the SVG sources.
 *
 * Logo sources live in `web/src/assets/logos/*.svg`. The list of files there
 * is the source of truth for "supported" brands; an unsupported camera Make
 * falls back to a text wordmark (see `brandDisplayName`).
 */

// Vite glob: raw SVG markup keyed by absolute module path. `?raw` avoids an
// extra network fetch and lets us read the viewBox to rasterize crisply.
const LOGO_SOURCES = import.meta.glob<string>(
  "../../../../assets/logos/*.svg",
  { eager: true, query: "?raw", import: "default" },
);

/** Map basename ("canon") -> raw SVG string, built once at module load. */
const LOGO_BY_KEY: Record<string, string> = Object.fromEntries(
  Object.entries(LOGO_SOURCES).map(([path, svg]) => {
    const key = path.split("/").pop()!.replace(/\.svg$/i, "");
    return [key, svg];
  }),
);

export type BrandKey = string;

/**
 * Brand detection rules, evaluated in order against "<Make> <Model>".
 * Order matters: Pentax is checked before Ricoh because Ricoh-built Pentax
 * bodies report Make "RICOH IMAGING COMPANY, LTD." with a "PENTAX ..." model.
 */
const BRAND_RULES: Array<{ key: BrandKey; test: RegExp; display: string }> = [
  { key: "apple", test: /apple/i, display: "Apple" },
  { key: "canon", test: /canon/i, display: "Canon" },
  { key: "dji", test: /\bdji\b|da-?jiang/i, display: "DJI" },
  { key: "fujifilm", test: /fuji/i, display: "FUJIFILM" },
  { key: "hasselblad", test: /hasselblad/i, display: "Hasselblad" },
  { key: "leica", test: /leica/i, display: "Leica" },
  { key: "nikon", test: /nikon/i, display: "Nikon" },
  { key: "olympus", test: /olympus|om digital|om-?system/i, display: "OLYMPUS" },
  { key: "panasonic", test: /panasonic|lumix/i, display: "Panasonic" },
  { key: "pentax", test: /pentax/i, display: "PENTAX" },
  { key: "ricoh", test: /ricoh/i, display: "RICOH" },
  { key: "sigma", test: /sigma/i, display: "SIGMA" },
  { key: "sony", test: /sony/i, display: "SONY" },
  { key: "zeiss", test: /zeiss|carl zeiss/i, display: "ZEISS" },
];

/**
 * Match a camera Make/Model to a supported logo key, or null when the brand
 * has no bundled logo (caller should fall back to `brandDisplayName`).
 */
export function matchBrandKey(
  make: string | undefined,
  model: string | undefined,
): BrandKey | null {
  const haystack = `${make ?? ""} ${model ?? ""}`;
  for (const rule of BRAND_RULES) {
    if (rule.test.test(haystack) && LOGO_BY_KEY[rule.key]) {
      return rule.key;
    }
  }
  return null;
}

function titleCase(value: string): string {
  return value
    .toLowerCase()
    .replace(/\b\w/g, (c) => c.toUpperCase())
    .trim();
}

/**
 * A human-friendly brand wordmark, used as the Frosted Info brand text and as
 * the Info Strip fallback when no logo matches. Matched brands use a curated
 * display name; unmatched brands fall back to a title-cased Make.
 */
export function brandDisplayName(
  make: string | undefined,
  matchedKey: BrandKey | null,
): string | null {
  if (matchedKey) {
    const rule = BRAND_RULES.find((r) => r.key === matchedKey);
    if (rule) return rule.display;
  }
  if (make && make.trim()) {
    // Makes are often shouty ("OLYMPUS IMAGING CORP."); keep the first token
    // group tidy without inventing a brand we don't recognize.
    const cleaned = make.replace(/\b(corp(oration)?|imaging|company|co|ltd)\b\.?/gi, "").trim();
    return titleCase(cleaned || make);
  }
  return null;
}

/**
 * Parse an SVG's aspect ratio and, when it has no `viewBox`, a synthesized one
 * derived from its width/height. The synthesized viewBox is essential: if we
 * later override width/height without a viewBox, content authored in the
 * original coordinate space falls outside the new viewport and renders blank
 * (this is what hid the Apple/Panasonic logos, which ship without a viewBox).
 */
function parseSvgGeometry(svg: string): {
  aspect: number;
  synthViewBox: string | null;
} {
  // Restrict parsing to the root <svg> opening tag (may span multiple lines).
  const openTag = svg.match(/<svg\b[^>]*>/i)?.[0] ?? "";
  const hasViewBox = /viewBox\s*=\s*"/i.test(openTag);

  let aspect = 3; // sensible wide fallback for unknown wordmarks
  const viewBoxMatch = openTag.match(/viewBox\s*=\s*"([\d.\s,+-]+)"/i);
  if (viewBoxMatch) {
    const parts = viewBoxMatch[1].trim().split(/[\s,]+/).map(Number);
    if (parts.length === 4 && parts[2] > 0 && parts[3] > 0) {
      aspect = parts[2] / parts[3];
    }
  }

  const w = openTag.match(/\bwidth\s*=\s*"([\d.]+)/i);
  const h = openTag.match(/\bheight\s*=\s*"([\d.]+)/i);
  const wn = w ? Number(w[1]) : Number.NaN;
  const hn = h ? Number(h[1]) : Number.NaN;
  if (!hasViewBox && wn > 0 && hn > 0) {
    aspect = wn / hn;
    return { aspect, synthViewBox: `0 0 ${wn} ${hn}` };
  }
  return { aspect, synthViewBox: null };
}

const LOGO_RASTER_HEIGHT = 256; // crisp source; the worker scales it down

/**
 * Rasterize a bundled brand SVG to an `ImageBitmap` sized to its natural
 * aspect ratio, ready to transfer to the tool worker. Returns null if the
 * brand has no bundled logo or rasterization fails.
 */
export async function rasterizeBrandLogo(
  key: BrandKey,
): Promise<ImageBitmap | null> {
  const svg = LOGO_BY_KEY[key];
  if (!svg) return null;

  const { aspect, synthViewBox } = parseSvgGeometry(svg);
  const height = LOGO_RASTER_HEIGHT;
  const width = Math.max(1, Math.round(height * aspect));

  // Force an explicit pixel size on the root <svg> so the <img> has a concrete
  // intrinsic size, and ensure a viewBox exists so the content scales into it.
  const sized = svg.replace(/<svg\b([^>]*)>/i, (_match, attrs: string) => {
    const cleaned = attrs
      .replace(/\swidth\s*=\s*"[^"]*"/i, "")
      .replace(/\sheight\s*=\s*"[^"]*"/i, "");
    const viewBoxAttr =
      synthViewBox && !/viewBox\s*=/i.test(cleaned)
        ? ` viewBox="${synthViewBox}"`
        : "";
    return `<svg${cleaned}${viewBoxAttr} width="${width}" height="${height}">`;
  });

  const blob = new Blob([sized], { type: "image/svg+xml" });
  const url = URL.createObjectURL(blob);
  try {
    const img = new Image();
    img.decoding = "async";
    img.src = url;
    await img.decode();

    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d");
    if (!ctx) return null;
    ctx.drawImage(img, 0, 0, width, height);
    return await createImageBitmap(canvas);
  } catch {
    return null;
  } finally {
    URL.revokeObjectURL(url);
  }
}
