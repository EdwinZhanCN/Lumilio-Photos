/**
 * Registers Studio's bundled faces into whichever font set the current context
 * has — `document.fonts` on the main thread, `self.fonts` inside a worker.
 *
 * The worker path is the one that matters. AfterFrame measures text with a
 * hidden DOM node and draws it on a canvas, and carries a long comment about
 * alignment drifting proportionally to line width whenever the two disagree
 * about which face is active (`useFrameTool.js:20-44`). Loading the same faces
 * into the worker's own font set lets us measure with the very context we draw
 * with, so there is no second measurement to disagree with.
 *
 * Callers must await this before any `measureText` or `fillText`. A face that
 * has not finished loading silently measures as a fallback.
 */

import { FONT_FACE_SOURCES } from "./fontFaces";

type AnyFontSet = {
  add(font: FontFace): void;
  ready?: Promise<unknown>;
};

/**
 * `FontFaceSet` is exposed as `document.fonts` in a window and `self.fonts` in
 * a worker. Both are optional at runtime: worker-side support is recent enough
 * that we degrade explicitly instead of rendering wrong text.
 */
function resolveFontSet(): AnyFontSet | null {
  if (typeof document !== "undefined" && document.fonts) {
    return document.fonts as unknown as AnyFontSet;
  }
  const scope = self as unknown as { fonts?: AnyFontSet };
  return scope.fonts ?? null;
}

export class StudioFontsUnavailableError extends Error {
  constructor() {
    super("This runtime has no FontFaceSet, so Studio text cannot be rendered accurately");
    this.name = "StudioFontsUnavailableError";
  }
}

let loadPromise: Promise<void> | null = null;

async function loadAll(): Promise<void> {
  const fontSet = resolveFontSet();
  if (!fontSet || typeof FontFace === "undefined") {
    throw new StudioFontsUnavailableError();
  }

  const results = await Promise.allSettled(
    FONT_FACE_SOURCES.map(async (source) => {
      const face = new FontFace(source.family, `url(${source.url}) format('woff2')`, {
        weight: String(source.weight),
        style: source.style,
        display: "block",
      });
      await face.load();
      fontSet.add(face);
    }),
  );

  // One family failing to decode must not deny the others: a composition using
  // Outfit should still render if the Chinese subset failed to fetch. Report
  // what was lost rather than failing the whole editor.
  const failed = results
    .map((result, index) => ({ result, source: FONT_FACE_SOURCES[index] }))
    .filter(({ result }) => result.status === "rejected");

  if (failed.length === FONT_FACE_SOURCES.length) {
    throw new Error("No Studio fonts could be loaded");
  }
  if (failed.length > 0) {
    const names = failed.map(({ source }) => `${source.family} ${source.weight} ${source.style}`);
    console.warn(`[studio] ${failed.length} font face(s) failed to load: ${names.join(", ")}`);
  }
}

/**
 * Idempotent: concurrent callers share one load, and a completed load resolves
 * immediately. A failed load is not cached, so a transient fetch error can be
 * retried by calling again.
 */
export function ensureStudioFontsLoaded(): Promise<void> {
  if (!loadPromise) {
    loadPromise = loadAll().catch((error: unknown) => {
      loadPromise = null;
      throw error;
    });
  }
  return loadPromise;
}

/** Build a canvas `font` shorthand. Callers must resolve family/weight first. */
export function cssFontShorthand(
  family: string,
  weight: number,
  italic: boolean,
  sizePx: number,
): string {
  return `${italic ? "italic " : ""}${weight} ${sizePx}px "${family}"`;
}
