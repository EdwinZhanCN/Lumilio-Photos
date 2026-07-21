/**
 * Brand-logo registry: matching a camera to a brand, and picking a variant.
 *
 * Pure and DOM-free — rasterization lives in `logoRaster.ts`, which is main
 * thread only. This module is just the manifest plus lookup rules, so it can be
 * used to decide what a template needs before anything is drawn.
 */

import manifest from "@/assets/logos/manifest.json";

export type LogoVariant = {
  id: string;
  kind: "symbol" | "wordmark";
  file: string;
  /** width / height, measured from the SVG viewBox. */
  aspect: number;
  /**
   * Height multiplier that normalizes variant shapes against a template's
   * single `size`. A tall square symbol renders at 1.0 and a short wide
   * wordmark at ~0.45, so one template gives every brand the same visual
   * weight.
   */
  h: number;
  /** An iconic color this variant keeps when the template's ink is neutral. */
  color?: string;
  /** Multi-color mark that must never be re-tinted. */
  colorLocked?: boolean;
};

export type LogoBrand = {
  id: string;
  name: string;
  accent: string;
  variants: LogoVariant[];
};

const BRANDS: LogoBrand[] = (manifest.logos as LogoBrand[]).map((brand) => ({
  ...brand,
  variants: brand.variants.map((variant) => ({ ...variant })),
}));

const BY_ID = new Map<string, LogoBrand>(BRANDS.map((brand) => [brand.id, brand]));

/**
 * Substring needle -> brand id. Order matters and is preserved from the
 * manifest: Pentax is tested before Ricoh because Ricoh-built Pentax bodies
 * report Make "RICOH IMAGING COMPANY, LTD." with a "PENTAX ..." model.
 */
const MATCH_RULES: Array<[needle: string, brandId: string]> = Object.entries(
  manifest.match as Record<string, string>,
);

export function allBrands(): readonly LogoBrand[] {
  return BRANDS;
}

export function findBrand(brandId: string): LogoBrand | null {
  return BY_ID.get(brandId) ?? null;
}

/**
 * Resolve a camera to a brand. Both Make and Model are searched because some
 * bodies omit the brand from Make and others omit it from Model.
 */
export function matchBrand(make: string | undefined, model: string | undefined): LogoBrand | null {
  const haystack = `${make ?? ""} ${model ?? ""}`.toLowerCase();
  if (!haystack.trim()) return null;
  for (const [needle, brandId] of MATCH_RULES) {
    if (haystack.includes(needle)) {
      const brand = BY_ID.get(brandId);
      if (brand) return brand;
    }
  }
  return null;
}

export type VariantQuery = {
  /** Preferred variant id, e.g. "symbol" or "wordmark". */
  variantId?: string;
  kind?: LogoVariant["kind"];
  /**
   * Return null instead of falling back when the request cannot be satisfied.
   * Dual-logo templates use this so a brand with only one mark skips the second
   * slot rather than printing the same mark twice.
   */
  strict?: boolean;
};

export function pickVariant(brand: LogoBrand | null, query: VariantQuery = {}): LogoVariant | null {
  if (!brand?.variants.length) return null;

  if (query.variantId) {
    const exact = brand.variants.find((variant) => variant.id === query.variantId);
    if (exact) return exact;
    if (query.strict) return null;
  }
  if (query.kind) {
    const byKind = brand.variants.find((variant) => variant.kind === query.kind);
    if (byKind) return byKind;
    if (query.strict) return null;
  }
  return query.strict ? null : brand.variants[0];
}

/**
 * Colors treated as "the template did not express an opinion", so a variant's
 * own iconic color (Canon red, Sony alpha orange) wins over them.
 */
const NEUTRAL_INK = new Set(["#141414", "#1a1a1a", "#f4f4f4", "#ffffff", "#000000"]);

/**
 * Final color for a mark, or null to keep its original colors.
 *
 * Precedence: a color-locked variant always keeps its own colors; an explicit
 * user override wins next; then the variant's iconic color if the template ink
 * is neutral; then the template ink.
 */
export function resolveLogoColor(
  variant: LogoVariant,
  templateInk: string | null,
  userOverride: string | null,
): string | null {
  if (variant.colorLocked) return null;
  if (userOverride) return userOverride;
  if (variant.color && (!templateInk || NEUTRAL_INK.has(templateInk.toLowerCase()))) {
    return variant.color;
  }
  return templateInk;
}
