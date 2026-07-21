/**
 * Bundled font binaries for Studio composition, as build-resolved URLs.
 *
 * The static font imports are resolved as asset URLs so the bundler can
 * fingerprint and emit each face. This module is only ever reached through a
 * dynamic import (see
 * `loadStudioFonts`), so the ~3.7 MB of font data lands in its own chunk and
 * costs nothing until someone opens the editor.
 *
 * The weight set here is the contract: it must stay in sync with
 * `model/fonts.ts`, because `resolveFontWeight` snaps a requested weight to
 * what that module claims exists, and a claim with no face behind it means the
 * engine synthesizes one — measured differently than drawn.
 *
 * Chinese uses the `chinese-simplified` build, which is ONE face covering the
 * whole subset. The default `@fontsource/noto-sans-sc` entry splits into ~100
 * `unicode-range` slices, and a browser only fetches a slice when a matching
 * character appears in the DOM. Canvas `fillText` never triggers that, so the
 * sliced build renders Chinese in a fallback face or as tofu.
 */

import outfit300 from "@fontsource/outfit/files/outfit-latin-300-normal.woff2";
import outfit400 from "@fontsource/outfit/files/outfit-latin-400-normal.woff2";
import outfit500 from "@fontsource/outfit/files/outfit-latin-500-normal.woff2";
import outfit700 from "@fontsource/outfit/files/outfit-latin-700-normal.woff2";

import jakarta300 from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-300-normal.woff2";
import jakarta400 from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-400-normal.woff2";
import jakarta500 from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-500-normal.woff2";
import jakarta700 from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-700-normal.woff2";
import jakarta800 from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-800-normal.woff2";
import jakarta400i from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-400-italic.woff2";
import jakarta700i from "@fontsource/plus-jakarta-sans/files/plus-jakarta-sans-latin-700-italic.woff2";

import inter400 from "@fontsource/inter/files/inter-latin-400-normal.woff2";
import inter500 from "@fontsource/inter/files/inter-latin-500-normal.woff2";
import inter700 from "@fontsource/inter/files/inter-latin-700-normal.woff2";

import playfair400 from "@fontsource/playfair-display/files/playfair-display-latin-400-normal.woff2";
import playfair700 from "@fontsource/playfair-display/files/playfair-display-latin-700-normal.woff2";
import playfair900 from "@fontsource/playfair-display/files/playfair-display-latin-900-normal.woff2";
import playfair400i from "@fontsource/playfair-display/files/playfair-display-latin-400-italic.woff2";
import playfair700i from "@fontsource/playfair-display/files/playfair-display-latin-700-italic.woff2";

import spaceMono400 from "@fontsource/space-mono/files/space-mono-latin-400-normal.woff2";
import spaceMono700 from "@fontsource/space-mono/files/space-mono-latin-700-normal.woff2";
import spaceMono400i from "@fontsource/space-mono/files/space-mono-latin-400-italic.woff2";

import caveat400 from "@fontsource/caveat/files/caveat-latin-400-normal.woff2";
import caveat700 from "@fontsource/caveat/files/caveat-latin-700-normal.woff2";

import notoSc400 from "@fontsource/noto-sans-sc/files/noto-sans-sc-chinese-simplified-400-normal.woff2";
import notoSc700 from "@fontsource/noto-sans-sc/files/noto-sans-sc-chinese-simplified-700-normal.woff2";
import notoSc900 from "@fontsource/noto-sans-sc/files/noto-sans-sc-chinese-simplified-900-normal.woff2";

import type { StudioFontFamily } from "../../../model/fonts";

export type FontFaceSource = {
  family: StudioFontFamily;
  weight: number;
  style: "normal" | "italic";
  url: string;
};

export const FONT_FACE_SOURCES: readonly FontFaceSource[] = [
  { family: "Outfit", weight: 300, style: "normal", url: outfit300 },
  { family: "Outfit", weight: 400, style: "normal", url: outfit400 },
  { family: "Outfit", weight: 500, style: "normal", url: outfit500 },
  { family: "Outfit", weight: 700, style: "normal", url: outfit700 },

  { family: "Plus Jakarta Sans", weight: 300, style: "normal", url: jakarta300 },
  { family: "Plus Jakarta Sans", weight: 400, style: "normal", url: jakarta400 },
  { family: "Plus Jakarta Sans", weight: 500, style: "normal", url: jakarta500 },
  { family: "Plus Jakarta Sans", weight: 700, style: "normal", url: jakarta700 },
  { family: "Plus Jakarta Sans", weight: 800, style: "normal", url: jakarta800 },
  { family: "Plus Jakarta Sans", weight: 400, style: "italic", url: jakarta400i },
  { family: "Plus Jakarta Sans", weight: 700, style: "italic", url: jakarta700i },

  { family: "Inter", weight: 400, style: "normal", url: inter400 },
  { family: "Inter", weight: 500, style: "normal", url: inter500 },
  { family: "Inter", weight: 700, style: "normal", url: inter700 },

  { family: "Playfair Display", weight: 400, style: "normal", url: playfair400 },
  { family: "Playfair Display", weight: 700, style: "normal", url: playfair700 },
  { family: "Playfair Display", weight: 900, style: "normal", url: playfair900 },
  { family: "Playfair Display", weight: 400, style: "italic", url: playfair400i },
  { family: "Playfair Display", weight: 700, style: "italic", url: playfair700i },

  { family: "Space Mono", weight: 400, style: "normal", url: spaceMono400 },
  { family: "Space Mono", weight: 700, style: "normal", url: spaceMono700 },
  { family: "Space Mono", weight: 400, style: "italic", url: spaceMono400i },

  { family: "Caveat", weight: 400, style: "normal", url: caveat400 },
  { family: "Caveat", weight: 700, style: "normal", url: caveat700 },

  { family: "Noto Sans SC", weight: 400, style: "normal", url: notoSc400 },
  { family: "Noto Sans SC", weight: 700, style: "normal", url: notoSc700 },
  { family: "Noto Sans SC", weight: 900, style: "normal", url: notoSc900 },
] as const;
