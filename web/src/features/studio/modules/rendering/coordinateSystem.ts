/**
 * The one place Studio converts between its three coordinate spaces.
 *
 * WORKER-SAFE. No DOM, no canvas — pure geometry so it can be unit tested and
 * shared by the worker (which renders) and the main thread (which draws crop and
 * text overlays over the preview).
 *
 * Three spaces, one photo:
 *
 *   - **source**  — the photo's real pixels (e.g. 6000×4000). Crop rectangles
 *     are authored and stored here so a saved edit is resolution independent.
 *   - **preview** — what the viewport shows: the source developed and shrunk to
 *     fit a small on-screen canvas.
 *   - **export**  — the output resolution, which may be the full source or a
 *     guardrail-reduced size.
 *
 * The develop engine renders the *un-rotated* photo at a chosen scale; rotation,
 * flip and crop are geometry applied afterwards (see `./geometry`). So the
 * numbers here describe two sizes per render: the un-rotated developed size the
 * engine targets, and the final composed-geometry size the viewport presents.
 */

/** A rectangle in source pixels. The crop half of {@link StudioEditAdjustments}. */
export type SourceRect = {
  x: number;
  y: number;
  width: number;
  height: number;
};

export type RenderSize = {
  /**
   * Un-rotated size the develop engine renders the WHOLE frame to. Geometry
   * later takes the (scaled) crop sub-rectangle out of this.
   */
  developWidth: number;
  developHeight: number;
  /** Final size after crop + rotation/flip — what the viewport canvas presents. */
  outWidth: number;
  outHeight: number;
  /** Normalized rotation in [0, 360). */
  angle: number;
  /** developSize / sourceSize — also the factor mapping a source crop into developed pixels. */
  scale: number;
};

function normalizeAngle(rotation: number): number {
  return ((rotation % 360) + 360) % 360;
}

function isQuarterTurn(angle: number): boolean {
  return angle === 90 || angle === 270;
}

/** Downscale factor that fits `width`×`height` inside a `maxSize` square. */
export function fitScale(width: number, height: number, maxSize: number): number {
  const longest = Math.max(width, height);
  if (longest <= 0 || longest <= maxSize) return 1;
  return maxSize / longest;
}

/**
 * Resolve the developed and composed sizes for a render.
 *
 * `crop`, when present, is the region of the source that survives; scale is
 * derived from its size so the cropped result fills the preview budget rather
 * than being shrunk with the whole frame. `maxSize` caps the longest edge of the
 * *final rotated* result, matching the viewport's fit budget.
 */
export function deriveRenderSize(
  sourceWidth: number,
  sourceHeight: number,
  rotation: number,
  crop: SourceRect | null,
  maxSize: number,
): RenderSize {
  const cropW = crop ? crop.width : sourceWidth;
  const cropH = crop ? crop.height : sourceHeight;
  const angle = normalizeAngle(rotation);
  const quarter = isQuarterTurn(angle);

  // Scale so the final rotated crop fills the preview budget.
  const rotatedW = quarter ? cropH : cropW;
  const rotatedH = quarter ? cropW : cropH;
  const scale = fitScale(rotatedW, rotatedH, maxSize);

  // The engine renders the whole frame; geometry crops the scaled sub-rect.
  const developWidth = Math.max(1, Math.round(sourceWidth * scale));
  const developHeight = Math.max(1, Math.round(sourceHeight * scale));

  const cropDevW = Math.max(1, Math.round(cropW * scale));
  const cropDevH = Math.max(1, Math.round(cropH * scale));
  const outWidth = quarter ? cropDevH : cropDevW;
  const outHeight = quarter ? cropDevW : cropDevH;

  return { developWidth, developHeight, outWidth, outHeight, angle, scale };
}

// --- Crop coordinate mapping -------------------------------------------------
//
// The crop overlay is dragged on the DISPLAYED frame (rotation + flip applied,
// crop not yet), but `adjustments.crop` is stored in SOURCE pixels because the
// pipeline crops before it rotates (see geometry.ts). These map between the two
// for 90° rotations + flips, mirroring exactly what applyGeometry draws.

/** Frame size the user sees and drags on, after rotation. */
export function displayedFrameSize(
  sourceWidth: number,
  sourceHeight: number,
  rotation: number,
): { width: number; height: number } {
  const angle = normalizeAngle(rotation);
  return isQuarterTurn(angle)
    ? { width: sourceHeight, height: sourceWidth }
    : { width: sourceWidth, height: sourceHeight };
}

function rotatePoint(x: number, y: number, angle: number): { x: number; y: number } {
  switch (normalizeAngle(angle)) {
    case 90:
      return { x: -y, y: x };
    case 180:
      return { x: -x, y: -y };
    case 270:
      return { x: y, y: -x };
    default:
      return { x, y };
  }
}

function rectFromCorners(ax: number, ay: number, bx: number, by: number): SourceRect {
  return {
    x: Math.min(ax, bx),
    y: Math.min(ay, by),
    width: Math.abs(ax - bx),
    height: Math.abs(ay - by),
  };
}

/** Map a rect on the displayed frame back to source pixels (for storage). */
export function mapRectDisplayedToSource(
  rect: SourceRect,
  sourceWidth: number,
  sourceHeight: number,
  rotation: number,
  flipHorizontal: boolean,
  flipVertical: boolean,
): SourceRect {
  const out = displayedFrameSize(sourceWidth, sourceHeight, rotation);
  const toSource = (dx: number, dy: number) => {
    const centered = rotatePoint(dx - out.width / 2, dy - out.height / 2, -rotation);
    return {
      x: centered.x * (flipHorizontal ? -1 : 1) + sourceWidth / 2,
      y: centered.y * (flipVertical ? -1 : 1) + sourceHeight / 2,
    };
  };
  const a = toSource(rect.x, rect.y);
  const b = toSource(rect.x + rect.width, rect.y + rect.height);
  return rectFromCorners(a.x, a.y, b.x, b.y);
}

/** Map a source-pixel rect onto the displayed frame (for seeding the overlay). */
export function mapRectSourceToDisplayed(
  rect: SourceRect,
  sourceWidth: number,
  sourceHeight: number,
  rotation: number,
  flipHorizontal: boolean,
  flipVertical: boolean,
): SourceRect {
  const out = displayedFrameSize(sourceWidth, sourceHeight, rotation);
  const toDisplayed = (sx: number, sy: number) => {
    const flipped = {
      x: (sx - sourceWidth / 2) * (flipHorizontal ? -1 : 1),
      y: (sy - sourceHeight / 2) * (flipVertical ? -1 : 1),
    };
    const rotated = rotatePoint(flipped.x, flipped.y, rotation);
    return { x: rotated.x + out.width / 2, y: rotated.y + out.height / 2 };
  };
  const a = toDisplayed(rect.x, rect.y);
  const b = toDisplayed(rect.x + rect.width, rect.y + rect.height);
  return rectFromCorners(a.x, a.y, b.x, b.y);
}

/** How the user chose the export resolution (the Pixelmator Quick Export subset). */
export type ExportSizeMode =
  | { kind: "original" }
  | { kind: "percent"; percent: number }
  | { kind: "longEdge"; longEdge: number };

export type ExportSizePlan = {
  /** Longest-edge budget to pass to {@link deriveRenderSize} for the export render. */
  maxSize: number;
  /** The full-resolution long edge the source can supply (after any source guardrail). */
  nativeLongEdge: number;
  /** True when the guardrail (or an unupscalable request) reduced the ask. */
  downscaled: boolean;
};

/**
 * Resolve the export render budget, applying the guardrail.
 *
 * `sourceWidth`/`sourceHeight` are the *effective* source (already clamped to
 * what the GPU could upload); `maxDimension` is the hard ceiling (GPU texture
 * limit). The result never upscales past the native long edge and never exceeds
 * the ceiling — `downscaled` says whether either bound bit, so the UI can tell
 * the user their full-resolution ask was reduced.
 */
export function resolveExportSize(
  sourceWidth: number,
  sourceHeight: number,
  crop: SourceRect | null,
  mode: ExportSizeMode,
  maxDimension: number,
): ExportSizePlan {
  const cropW = crop ? crop.width : sourceWidth;
  const cropH = crop ? crop.height : sourceHeight;
  const nativeLongEdge = Math.max(1, Math.round(Math.max(cropW, cropH)));

  let requested: number;
  if (mode.kind === "percent") {
    const percent = Math.min(100, Math.max(1, mode.percent));
    requested = (nativeLongEdge * percent) / 100;
  } else if (mode.kind === "longEdge") {
    requested = mode.longEdge;
  } else {
    requested = nativeLongEdge;
  }
  requested = Math.max(1, Math.round(requested));

  const ceiling = Math.max(1, Math.min(Math.round(maxDimension), nativeLongEdge));
  const maxSize = Math.min(requested, ceiling);
  return { maxSize, nativeLongEdge, downscaled: maxSize < requested };
}

/**
 * Fit a source into a texture the GPU can actually upload.
 *
 * Returns the source unchanged when it already fits `maxDimension`
 * (`gl.MAX_TEXTURE_SIZE`), otherwise the largest same-aspect size that does.
 */
export function clampToTexture(
  width: number,
  height: number,
  maxDimension: number,
): { width: number; height: number; clamped: boolean } {
  const scale = fitScale(width, height, maxDimension);
  if (scale >= 1) return { width, height, clamped: false };
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
    clamped: true,
  };
}
