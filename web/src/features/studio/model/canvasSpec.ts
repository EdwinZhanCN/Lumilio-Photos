/**
 * The canvas treatment — Studio's border layer.
 *
 * A canvas describes how the photo is framed: how much room is added around it,
 * what fills that room, how the corners are cut, and what is laid over the photo
 * to keep overlaid content legible. It says nothing about text or logos; those
 * are layers (see `./layers`), and a frame template is just a preset that
 * references a canvas and a set of layers.
 *
 * Keeping frosted here rather than in the template is the point: "frosted with
 * a caption" and "frosted on its own" are one canvas with different layers, not
 * two modes. The former Border tool had them as two renderers that disagreed on
 * what every shared parameter meant.
 *
 * All geometry is fractional so a spec renders identically at preview size and
 * at full resolution. Padding is a fraction of the photo's SHORT edge, so the
 * same spec gives a landscape and a portrait frame the same visual weight —
 * a width basis would make portrait margins look thin.
 */

/** Padding around the photo, each side a fraction of the photo's short edge. */
export type CanvasPad = {
  top: number;
  right: number;
  bottom: number;
  left: number;
};

/**
 * What fills the padded area.
 *
 * `frosted` reproduces the photo itself, scaled up past the canvas bounds
 * (`overscan`) and blurred. The overscan matters: blurring an image drawn at
 * exact canvas size samples past its edge and leaves a visibly lighter rim.
 * The old `FROSTED` renderer omitted it and had that rim; `FROSTED_INFO`
 * included it. This is the corrected behaviour for both.
 */
export type CanvasBackground =
  | { kind: "solid"; color: string }
  | { kind: "gradient"; from: string; to: string; angle: number }
  | { kind: "frosted"; blur: number; brightness: number; overscan: number };

/**
 * A one-sided gradient wash over the photo, so text placed on the image stays
 * readable no matter how bright that edge is. `height` is a fraction of the
 * photo's height.
 */
export type CanvasScrim = {
  edge: "top" | "bottom";
  from: string;
  to: string;
  height: number;
};

export type CanvasSpec = {
  pad: CanvasPad;
  background: CanvasBackground;
  /** Corner radius of the whole composed output, as a fraction of its short edge. */
  outerRadius: number;
  /** Corner radius of the photo within the frame, as a fraction of ITS short edge. */
  innerRadius: number;
  scrim: CanvasScrim | null;
  /** Radial darkening toward the corners. 0 disables it. */
  vignette: number;
};

export const NO_PAD: CanvasPad = { top: 0, right: 0, bottom: 0, left: 0 };

export const DEFAULT_CANVAS: CanvasSpec = {
  pad: NO_PAD,
  background: { kind: "solid", color: "#ffffff" },
  outerRadius: 0,
  innerRadius: 0,
  scrim: null,
  vignette: 0,
};

/**
 * The frosted look, tuned to match what the old `FROSTED_INFO` renderer
 * produced — the better-behaved of the two implementations.
 */
export const DEFAULT_FROSTED_BACKGROUND: Extract<CanvasBackground, { kind: "frosted" }> = {
  kind: "frosted",
  blur: 0.06,
  brightness: -0.16,
  overscan: 1.12,
};

const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;
const CSS_COLOR = /^(#[0-9a-fA-F]{3,8}|rgba?\([\d.,\s%/]+\)|transparent)$/;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function num(value: unknown, fallback: number, min: number, max: number): number {
  return typeof value === "number" && Number.isFinite(value)
    ? clamp(value, min, max)
    : fallback;
}

function hex(value: unknown, fallback: string): string {
  return typeof value === "string" && HEX_COLOR.test(value) ? value : fallback;
}

/** Scrim stops are authored as rgba(), so they take the wider CSS color test. */
function cssColor(value: unknown, fallback: string): string {
  return typeof value === "string" && CSS_COLOR.test(value.trim()) ? value.trim() : fallback;
}

export function normalizeCanvasPad(input: unknown): CanvasPad {
  const raw = (input ?? {}) as Partial<Record<keyof CanvasPad, unknown>>;
  return {
    top: num(raw.top, 0, 0, 2),
    right: num(raw.right, 0, 0, 2),
    bottom: num(raw.bottom, 0, 0, 2),
    left: num(raw.left, 0, 0, 2),
  };
}

export function normalizeCanvasBackground(input: unknown): CanvasBackground {
  const raw = (input ?? {}) as Record<string, unknown>;
  if (raw.kind === "frosted") {
    return {
      kind: "frosted",
      blur: num(raw.blur, DEFAULT_FROSTED_BACKGROUND.blur, 0, 0.5),
      brightness: num(raw.brightness, DEFAULT_FROSTED_BACKGROUND.brightness, -1, 1),
      overscan: num(raw.overscan, DEFAULT_FROSTED_BACKGROUND.overscan, 1, 2),
    };
  }
  if (raw.kind === "gradient") {
    return {
      kind: "gradient",
      from: hex(raw.from, "#ffffff"),
      to: hex(raw.to, "#d4d4d4"),
      angle: num(raw.angle, 180, 0, 360),
    };
  }
  return { kind: "solid", color: hex(raw.color, "#ffffff") };
}

export function normalizeCanvasScrim(input: unknown): CanvasScrim | null {
  if (!input || typeof input !== "object") return null;
  const raw = input as Record<string, unknown>;
  return {
    edge: raw.edge === "top" ? "top" : "bottom",
    from: cssColor(raw.from, "rgba(0,0,0,0)"),
    to: cssColor(raw.to, "rgba(0,0,0,0.5)"),
    height: num(raw.height, 0.32, 0, 1),
  };
}

export function normalizeCanvasSpec(input: unknown): CanvasSpec {
  if (!input || typeof input !== "object") return { ...DEFAULT_CANVAS, pad: { ...NO_PAD } };
  const raw = input as Record<string, unknown>;
  return {
    pad: normalizeCanvasPad(raw.pad),
    background: normalizeCanvasBackground(raw.background),
    outerRadius: num(raw.outerRadius, 0, 0, 0.5),
    innerRadius: num(raw.innerRadius, 0, 0, 0.5),
    scrim: normalizeCanvasScrim(raw.scrim),
    vignette: num(raw.vignette, 0, 0, 1),
  };
}

/** True when the spec would change a single pixel of the source photo. */
export function isCanvasActive(canvas: CanvasSpec | null): boolean {
  if (!canvas) return false;
  const { pad } = canvas;
  return (
    pad.top > 0 ||
    pad.right > 0 ||
    pad.bottom > 0 ||
    pad.left > 0 ||
    canvas.outerRadius > 0 ||
    canvas.innerRadius > 0 ||
    canvas.vignette > 0 ||
    canvas.scrim !== null
  );
}

/**
 * Composed output size for a photo framed by `canvas`, in pixels.
 *
 * Padding resolves against the short edge, so callers must not re-derive it
 * from width — this function is the single place that conversion happens.
 */
export function resolveCanvasGeometry(
  photoWidth: number,
  photoHeight: number,
  canvas: CanvasSpec,
): {
  outWidth: number;
  outHeight: number;
  padPx: CanvasPad;
  photoX: number;
  photoY: number;
} {
  const shortEdge = Math.min(photoWidth, photoHeight);
  const padPx: CanvasPad = {
    top: Math.round(canvas.pad.top * shortEdge),
    right: Math.round(canvas.pad.right * shortEdge),
    bottom: Math.round(canvas.pad.bottom * shortEdge),
    left: Math.round(canvas.pad.left * shortEdge),
  };
  return {
    outWidth: photoWidth + padPx.left + padPx.right,
    outHeight: photoHeight + padPx.top + padPx.bottom,
    padPx,
    photoX: padPx.left,
    photoY: padPx.top,
  };
}
