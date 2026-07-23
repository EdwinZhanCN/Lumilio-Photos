/**
 * Crop-rectangle geometry, ported from AfterFrame's `cropMath.js`.
 *
 * Pure functions, no DOM — the interaction math for a draggable crop box with
 * eight handles, free or aspect-locked. All rectangles live in one "bounds"
 * space (the caller decides which — Studio uses the displayed, rotated frame in
 * source pixels) and are kept inside those bounds with a minimum size.
 *
 * Deliberately NOT ported yet: straighten (free-angle rotation) and
 * centre-symmetric resize. Studio's crop lands first as the coordinate-system
 * proof; those are additive follow-ups.
 */

export const MIN_CROP_SIZE = 48;

export type CropRect = { x: number; y: number; width: number; height: number };
export type CropBounds = { width: number; height: number };
export type CropPoint = { x: number; y: number };
export type CropHandle = "n" | "s" | "e" | "w" | "nw" | "ne" | "sw" | "se";

export type AspectPreset = {
  key: string;
  label: string;
  /** number ratio, "original" (resolved against the source), or null for free. */
  aspect: number | "original" | null;
};

export const ASPECT_PRESETS: readonly AspectPreset[] = [
  { key: "free", label: "Free", aspect: null },
  { key: "original", label: "Original", aspect: "original" },
  { key: "1:1", label: "1:1", aspect: 1 },
  { key: "3:2", label: "3:2", aspect: 3 / 2 },
  { key: "2:3", label: "2:3", aspect: 2 / 3 },
  { key: "4:3", label: "4:3", aspect: 4 / 3 },
  { key: "3:4", label: "3:4", aspect: 3 / 4 },
  { key: "5:4", label: "5:4", aspect: 5 / 4 },
  { key: "4:5", label: "4:5", aspect: 4 / 5 },
  { key: "16:9", label: "16:9", aspect: 16 / 9 },
  { key: "9:16", label: "9:16", aspect: 9 / 16 },
  { key: "2.35:1", label: "2.35:1", aspect: 2.35 },
];

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function minDimensionsForAspect(aspect: number | null): { width: number; height: number } {
  if (!aspect) return { width: MIN_CROP_SIZE, height: MIN_CROP_SIZE };
  if (aspect >= 1) return { width: MIN_CROP_SIZE * aspect, height: MIN_CROP_SIZE };
  return { width: MIN_CROP_SIZE, height: MIN_CROP_SIZE / aspect };
}

function normalizeRect(rect: CropRect, bounds: CropBounds): CropRect {
  const width = clamp(rect.width, Math.min(MIN_CROP_SIZE, bounds.width), bounds.width);
  const height = clamp(rect.height, Math.min(MIN_CROP_SIZE, bounds.height), bounds.height);
  return {
    x: clamp(rect.x, 0, bounds.width - width),
    y: clamp(rect.y, 0, bounds.height - height),
    width,
    height,
  };
}

/** Resolve an aspect-preset key to a numeric ratio for the current source. */
export function getAspectRatio(aspectKey: string, originalAspect: number): number | null {
  const preset = ASPECT_PRESETS.find((entry) => entry.key === aspectKey);
  if (!preset) return null;
  if (preset.aspect === "original") return originalAspect || null;
  return preset.aspect;
}

/** The largest centered crop of `aspect` (or the whole frame) inside `bounds`. */
export function createDefaultCropRect(bounds: CropBounds, aspect: number | null): CropRect {
  if (!aspect) {
    return normalizeRect({ x: 0, y: 0, width: bounds.width, height: bounds.height }, bounds);
  }
  let width = bounds.width;
  let height = width / aspect;
  if (height > bounds.height) {
    height = bounds.height;
    width = height * aspect;
  }
  return normalizeRect(
    { x: (bounds.width - width) / 2, y: (bounds.height - height) / 2, width, height },
    bounds,
  );
}

export function moveCropRect(
  rect: CropRect,
  bounds: CropBounds,
  deltaX: number,
  deltaY: number,
): CropRect {
  return normalizeRect({ ...rect, x: rect.x + deltaX, y: rect.y + deltaY }, bounds);
}

function resizeFreeCropRect(
  rect: CropRect,
  handle: CropHandle,
  point: CropPoint,
  bounds: CropBounds,
): CropRect {
  let left = rect.x;
  let top = rect.y;
  let right = rect.x + rect.width;
  let bottom = rect.y + rect.height;

  if (handle.includes("w")) left = clamp(point.x, 0, right - MIN_CROP_SIZE);
  if (handle.includes("e")) right = clamp(point.x, left + MIN_CROP_SIZE, bounds.width);
  if (handle.includes("n")) top = clamp(point.y, 0, bottom - MIN_CROP_SIZE);
  if (handle.includes("s")) bottom = clamp(point.y, top + MIN_CROP_SIZE, bounds.height);

  return { x: left, y: top, width: right - left, height: bottom - top };
}

function fitCornerRect(
  anchor: CropPoint,
  point: CropPoint,
  bounds: CropBounds,
  aspect: number,
  directionX: number,
  directionY: number,
): CropRect {
  const minSize = minDimensionsForAspect(aspect);
  const maxWidth = directionX > 0 ? bounds.width - anchor.x : anchor.x;
  const maxHeight = directionY > 0 ? bounds.height - anchor.y : anchor.y;

  let width = Math.max(minSize.width, directionX > 0 ? point.x - anchor.x : anchor.x - point.x);
  let height = width / aspect;
  const pointerHeight = Math.max(
    minSize.height,
    directionY > 0 ? point.y - anchor.y : anchor.y - point.y,
  );
  if (height > pointerHeight) {
    height = pointerHeight;
    width = height * aspect;
  }
  if (width > maxWidth) {
    width = maxWidth;
    height = width / aspect;
  }
  if (height > maxHeight) {
    height = maxHeight;
    width = height * aspect;
  }
  width = Math.max(minSize.width, width);
  height = Math.max(minSize.height, height);

  const x = directionX > 0 ? anchor.x : anchor.x - width;
  const y = directionY > 0 ? anchor.y : anchor.y - height;
  return normalizeRect({ x, y, width, height }, bounds);
}

function fitVerticalEdgeRect(
  rect: CropRect,
  point: CropPoint,
  bounds: CropBounds,
  aspect: number,
  edge: "n" | "s",
): CropRect {
  const minSize = minDimensionsForAspect(aspect);
  const centerX = rect.x + rect.width / 2;
  const anchorY = edge === "n" ? rect.y + rect.height : rect.y;
  const maxWidth = Math.min(centerX, bounds.width - centerX) * 2;
  const maxHeight = edge === "n" ? anchorY : bounds.height - anchorY;

  let height = Math.max(minSize.height, edge === "n" ? anchorY - point.y : point.y - anchorY);
  let width = height * aspect;
  if (width > maxWidth) {
    width = maxWidth;
    height = width / aspect;
  }
  if (height > maxHeight) {
    height = maxHeight;
    width = height * aspect;
  }
  width = Math.max(minSize.width, width);
  height = Math.max(minSize.height, height);

  return normalizeRect(
    { x: centerX - width / 2, y: edge === "n" ? anchorY - height : anchorY, width, height },
    bounds,
  );
}

function fitHorizontalEdgeRect(
  rect: CropRect,
  point: CropPoint,
  bounds: CropBounds,
  aspect: number,
  edge: "w" | "e",
): CropRect {
  const minSize = minDimensionsForAspect(aspect);
  const centerY = rect.y + rect.height / 2;
  const anchorX = edge === "w" ? rect.x + rect.width : rect.x;
  const maxHeight = Math.min(centerY, bounds.height - centerY) * 2;
  const maxWidth = edge === "w" ? anchorX : bounds.width - anchorX;

  let width = Math.max(minSize.width, edge === "w" ? anchorX - point.x : point.x - anchorX);
  let height = width / aspect;
  if (height > maxHeight) {
    height = maxHeight;
    width = height * aspect;
  }
  if (width > maxWidth) {
    width = maxWidth;
    height = width / aspect;
  }
  width = Math.max(minSize.width, width);
  height = Math.max(minSize.height, height);

  return normalizeRect(
    { x: edge === "w" ? anchorX - width : anchorX, y: centerY - height / 2, width, height },
    bounds,
  );
}

function resizeFixedCropRect(
  rect: CropRect,
  handle: CropHandle,
  point: CropPoint,
  bounds: CropBounds,
  aspect: number,
): CropRect {
  if (handle === "n" || handle === "s") return fitVerticalEdgeRect(rect, point, bounds, aspect, handle);
  if (handle === "w" || handle === "e") return fitHorizontalEdgeRect(rect, point, bounds, aspect, handle);

  const anchors: Record<string, { x: number; y: number; dx: number; dy: number }> = {
    nw: { x: rect.x + rect.width, y: rect.y + rect.height, dx: -1, dy: -1 },
    ne: { x: rect.x, y: rect.y + rect.height, dx: 1, dy: -1 },
    sw: { x: rect.x + rect.width, y: rect.y, dx: -1, dy: 1 },
    se: { x: rect.x, y: rect.y, dx: 1, dy: 1 },
  };
  const anchor = anchors[handle];
  if (!anchor) return rect;
  return fitCornerRect({ x: anchor.x, y: anchor.y }, point, bounds, aspect, anchor.dx, anchor.dy);
}

/** Resize `rect` by dragging `handle` toward `point`, free or aspect-locked. */
export function resizeCropRect(
  rect: CropRect,
  handle: CropHandle,
  point: CropPoint,
  bounds: CropBounds,
  aspect: number | null,
): CropRect {
  if (!aspect) return resizeFreeCropRect(rect, handle, point, bounds);
  return resizeFixedCropRect(rect, handle, point, bounds, aspect);
}
