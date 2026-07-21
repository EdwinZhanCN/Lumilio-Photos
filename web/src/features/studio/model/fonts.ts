/**
 * The fonts Studio composition can use.
 *
 * This module names families and the weights each one actually ships; it holds
 * no URLs and loads nothing, so it stays usable from panels, templates, and the
 * render worker alike. Loading lives in `modules/rendering/fonts`.
 *
 * Every family is bundled locally. Nothing resolves to a system font and
 * nothing is fetched from a CDN: a composition must render identically on any
 * machine, and the deployment has to work offline.
 */

export type StudioFontFamily =
  | "Outfit"
  | "Plus Jakarta Sans"
  | "Inter"
  | "Playfair Display"
  | "Space Mono"
  | "Caveat"
  | "Noto Sans SC";

export type StudioFontDefinition = {
  family: StudioFontFamily;
  /** Weights with a real bundled face. Asking for others snaps to the nearest. */
  weights: number[];
  hasItalic: boolean;
  /** Covers Han glyphs — the only family that can render Chinese text. */
  cjk: boolean;
};

/**
 * Every weight listed here MUST have a bundled face in
 * `modules/rendering/fonts/fontFaces.ts`. `resolveFontWeight` snaps to this
 * list, so a weight with no face behind it makes the engine synthesize one —
 * and a synthesized face measures differently than it draws.
 */
export const STUDIO_FONTS: readonly StudioFontDefinition[] = [
  { family: "Outfit", weights: [300, 400, 500, 700], hasItalic: false, cjk: false },
  {
    family: "Plus Jakarta Sans",
    weights: [300, 400, 500, 700, 800],
    hasItalic: true,
    cjk: false,
  },
  { family: "Inter", weights: [400, 500, 700], hasItalic: false, cjk: false },
  { family: "Playfair Display", weights: [400, 700, 900], hasItalic: true, cjk: false },
  { family: "Space Mono", weights: [400, 700], hasItalic: true, cjk: false },
  { family: "Caveat", weights: [400, 700], hasItalic: false, cjk: false },
  { family: "Noto Sans SC", weights: [400, 700, 900], hasItalic: false, cjk: true },
] as const;

/** Logical role -> family, so a template asks for "grotesk" and not a product name. */
export const FRAME_FONT_ROLES = {
  grotesk: "Outfit",
  sans: "Plus Jakarta Sans",
  mono: "Space Mono",
  serif: "Playfair Display",
  hand: "Caveat",
  cjk: "Noto Sans SC",
} as const satisfies Record<string, StudioFontFamily>;

export type FrameFontRole = keyof typeof FRAME_FONT_ROLES;

export const DEFAULT_FONT_FAMILY: StudioFontFamily = "Plus Jakarta Sans";

const BY_FAMILY = new Map<string, StudioFontDefinition>(
  STUDIO_FONTS.map((font) => [font.family, font]),
);

export function findFont(family: string): StudioFontDefinition | null {
  return BY_FAMILY.get(family) ?? null;
}

/**
 * Snap a requested weight to one this family actually ships.
 *
 * Canvas will happily accept `font-weight: 600` for a family with no 600 face
 * and synthesize it inconsistently across engines, which then measures
 * differently than it draws. Resolving up front keeps measurement and rendering
 * on the same real face.
 */
export function resolveFontWeight(family: string, weight: number): number {
  const font = BY_FAMILY.get(family);
  if (!font) return weight;
  let best = font.weights[0];
  let bestDistance = Math.abs(best - weight);
  for (const candidate of font.weights) {
    const distance = Math.abs(candidate - weight);
    if (distance < bestDistance) {
      best = candidate;
      bestDistance = distance;
    }
  }
  return best;
}

const HAN = /[㐀-䶿一-鿿豈-﫿]/;

export function containsHan(text: string): boolean {
  return HAN.test(text);
}

/**
 * The family to actually render `text` in.
 *
 * A Latin family has no Han glyphs, so Chinese set in one falls back per-glyph
 * to whatever the engine picks — which differs between the preview canvas and a
 * worker, and between machines. Routing Han text to the bundled CJK family
 * keeps it deterministic.
 */
export function resolveFontFamily(requested: string, text: string): StudioFontFamily {
  const font = BY_FAMILY.get(requested);
  if (!font) return DEFAULT_FONT_FAMILY;
  if (!font.cjk && containsHan(text)) return FRAME_FONT_ROLES.cjk;
  return font.family;
}
