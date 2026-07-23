/**
 * Composition layers — the content placed on a framed photo.
 *
 * A layer is either text or a brand logo. Both share geometry, opacity, and an
 * optional shadow; everything else is specific to the kind. Frame templates
 * expand into these, and once expanded a template-produced layer is an ordinary
 * editable layer with no special status.
 *
 * Units are deliberately uniform: every position, size, and offset is a
 * fraction of the COMPOSED OUTPUT (the photo plus its canvas padding), and
 * every opacity is 0..1. A layer therefore renders identically against a
 * 400 px preview and a 6000 px export, and no caller needs a scale factor.
 *
 * This is a departure from the AfterFrame original, which mixed three bases:
 * font sizes in "px at a 1920 px-wide reference", shadow offsets in the same
 * reference, sticker scale as a width fraction, and opacity as 0..100 in some
 * fields and 0..1 in others. Every conversion between them was a place to get
 * it wrong.
 */

/** Offsets and blur are fractions of the composed output's width. */
export type LayerShadow = {
  color: string;
  opacity: number;
  blur: number;
  offsetX: number;
  offsetY: number;
};

export type LayerFill =
  | { kind: "solid"; color: string; opacity: number }
  | {
      kind: "gradient";
      from: string;
      to: string;
      angle: number;
      fromOpacity: number;
      toOpacity: number;
    };

export type TextCase = "none" | "upper" | "lower" | "title";
export type TextAlign = "left" | "center" | "right";

export type TextFont = {
  family: string;
  weight: number;
  italic: boolean;
  /** Cap-height-ish size as a fraction of the composed output's width. */
  size: number;
  /** Letter spacing as a fraction of the font size (em). */
  tracking: number;
  /** Line box as a multiple of the font size. */
  lineHeight: number;
};

/** Padding around a text background block, as fractions of the font size. */
export type TextBackgroundPadding = {
  top: number;
  right: number;
  bottom: number;
  left: number;
};

type LayerBase = {
  id: string;
  /** Position of the layer's visual CENTER, as fractions of the composed output. */
  x: number;
  y: number;
  /** Clockwise degrees. */
  rotation: number;
  opacity: number;
  shadow: LayerShadow | null;
  /**
   * Depth plane in 0..1 for scene occlusion: 1 = always in front (no
   * occlusion), lower sits deeper so nearer scene pixels hide the layer. Applied
   * only when a depth field has been computed; the default 1 is a no-op.
   */
  zPosition: number;
  /**
   * Set when a frame template produced this layer. Purely informational — it
   * lets the UI offer "reset to preset" and does not restrict editing.
   */
  fromTemplate: boolean;
};

export type TextLayer = LayerBase & {
  type: "text";
  text: string;
  font: TextFont;
  textCase: TextCase;
  align: TextAlign;
  fill: LayerFill;
  stroke: { color: string; width: number } | null;
  background: { fill: LayerFill; padding: TextBackgroundPadding } | null;
  underline: boolean;
  strikethrough: boolean;
};

export type LogoLayer = LayerBase & {
  type: "logo";
  brand: string;
  variant: string;
  /** `null` keeps the mark's own colors (required for color-locked brands). */
  color: string | null;
  /** Width as a fraction of the composed output's width. */
  size: number;
};

export type Layer = TextLayer | LogoLayer;

export const DEFAULT_SHADOW: LayerShadow = {
  color: "#000000",
  opacity: 0.6,
  blur: 0.006,
  offsetX: 0,
  offsetY: 0.003,
};

export const DEFAULT_TEXT_FONT: TextFont = {
  family: "Plus Jakarta Sans",
  weight: 400,
  italic: false,
  size: 0.06,
  tracking: 0,
  lineHeight: 1.2,
};

export const DEFAULT_TEXT_BACKGROUND_PADDING: TextBackgroundPadding = {
  top: 0.15,
  right: 0.25,
  bottom: 0.15,
  left: 0.25,
};

let layerSeq = 0;

function nextId(prefix: string): string {
  layerSeq += 1;
  return `${prefix}-${Date.now().toString(36)}-${layerSeq}`;
}

export function createTextLayer(overrides: Partial<Omit<TextLayer, "type">> = {}): TextLayer {
  return {
    id: nextId("text"),
    type: "text",
    text: "",
    font: { ...DEFAULT_TEXT_FONT, ...overrides.font },
    textCase: "none",
    align: "center",
    fill: { kind: "solid", color: "#ffffff", opacity: 1 },
    stroke: null,
    background: null,
    underline: false,
    strikethrough: false,
    x: 0.5,
    y: 0.5,
    rotation: 0,
    opacity: 1,
    shadow: null,
    zPosition: 1,
    fromTemplate: false,
    ...overrides,
  };
}

export function createLogoLayer(
  brand: string,
  variant: string,
  overrides: Partial<Omit<LogoLayer, "type" | "brand" | "variant">> = {},
): LogoLayer {
  return {
    id: nextId("logo"),
    type: "logo",
    brand,
    variant,
    color: null,
    size: 0.12,
    x: 0.5,
    y: 0.5,
    rotation: 0,
    opacity: 1,
    shadow: null,
    zPosition: 1,
    fromTemplate: false,
    ...overrides,
  };
}

export function isTextLayer(layer: Layer): layer is TextLayer {
  return layer.type === "text";
}

export function isLogoLayer(layer: Layer): layer is LogoLayer {
  return layer.type === "logo";
}

/** Apply a layer's case transform. Done here so preview and render share one string. */
export function displayText(layer: TextLayer): string {
  const value = layer.text ?? "";
  switch (layer.textCase) {
    case "upper":
      return value.toUpperCase();
    case "lower":
      return value.toLowerCase();
    case "title":
      return value.replace(/\S+/g, (word) => word.charAt(0).toUpperCase() + word.slice(1));
    default:
      return value;
  }
}

// --- normalization (sidecar input is untrusted) ------------------------------

const HEX_COLOR = /^#[0-9a-fA-F]{6}$/;
const CSS_COLOR = /^(#[0-9a-fA-F]{3,8}|rgba?\([\d.,\s%/]+\)|transparent)$/;

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function num(value: unknown, fallback: number, min: number, max: number): number {
  return typeof value === "number" && Number.isFinite(value) ? clamp(value, min, max) : fallback;
}

function bool(value: unknown): boolean {
  return value === true;
}

function color(value: unknown, fallback: string): string {
  if (typeof value !== "string") return fallback;
  const trimmed = value.trim();
  return HEX_COLOR.test(trimmed) || CSS_COLOR.test(trimmed) ? trimmed : fallback;
}

function normalizeFill(input: unknown, fallbackColor: string): LayerFill {
  const raw = (input ?? {}) as Record<string, unknown>;
  if (raw.kind === "gradient") {
    return {
      kind: "gradient",
      from: color(raw.from, fallbackColor),
      to: color(raw.to, "#000000"),
      angle: num(raw.angle, 90, 0, 360),
      fromOpacity: num(raw.fromOpacity, 1, 0, 1),
      toOpacity: num(raw.toOpacity, 1, 0, 1),
    };
  }
  return {
    kind: "solid",
    color: color(raw.color, fallbackColor),
    opacity: num(raw.opacity, 1, 0, 1),
  };
}

function normalizeShadow(input: unknown): LayerShadow | null {
  if (!input || typeof input !== "object") return null;
  const raw = input as Record<string, unknown>;
  return {
    color: color(raw.color, DEFAULT_SHADOW.color),
    opacity: num(raw.opacity, DEFAULT_SHADOW.opacity, 0, 1),
    blur: num(raw.blur, DEFAULT_SHADOW.blur, 0, 0.5),
    offsetX: num(raw.offsetX, 0, -0.5, 0.5),
    offsetY: num(raw.offsetY, 0, -0.5, 0.5),
  };
}

function normalizeTextCase(value: unknown): TextCase {
  return value === "upper" || value === "lower" || value === "title" ? value : "none";
}

function normalizeAlign(value: unknown): TextAlign {
  return value === "left" || value === "right" ? value : "center";
}

function normalizeFont(input: unknown): TextFont {
  const raw = (input ?? {}) as Record<string, unknown>;
  return {
    family: typeof raw.family === "string" && raw.family ? raw.family : DEFAULT_TEXT_FONT.family,
    weight: num(raw.weight, DEFAULT_TEXT_FONT.weight, 100, 900),
    italic: bool(raw.italic),
    size: num(raw.size, DEFAULT_TEXT_FONT.size, 0.001, 2),
    tracking: num(raw.tracking, 0, -0.5, 2),
    lineHeight: num(raw.lineHeight, DEFAULT_TEXT_FONT.lineHeight, 0.5, 4),
  };
}

function normalizeLayer(input: unknown): Layer | null {
  if (!input || typeof input !== "object") return null;
  const raw = input as Record<string, unknown>;

  const base = {
    id: typeof raw.id === "string" && raw.id ? raw.id : nextId("layer"),
    x: num(raw.x, 0.5, -1, 2),
    y: num(raw.y, 0.5, -1, 2),
    rotation: num(raw.rotation, 0, -360, 360),
    opacity: num(raw.opacity, 1, 0, 1),
    shadow: normalizeShadow(raw.shadow),
    zPosition: num(raw.zPosition, 1, 0, 1),
    fromTemplate: bool(raw.fromTemplate),
  };

  if (raw.type === "logo") {
    if (typeof raw.brand !== "string" || !raw.brand) return null;
    return {
      ...base,
      type: "logo",
      brand: raw.brand,
      variant: typeof raw.variant === "string" && raw.variant ? raw.variant : "wordmark",
      color: typeof raw.color === "string" ? color(raw.color, "#000000") : null,
      size: num(raw.size, 0.12, 0.001, 2),
    };
  }

  if (raw.type !== "text") return null;
  const backgroundRaw = raw.background as Record<string, unknown> | null | undefined;
  const strokeRaw = raw.stroke as Record<string, unknown> | null | undefined;

  return {
    ...base,
    type: "text",
    text: typeof raw.text === "string" ? raw.text : "",
    font: normalizeFont(raw.font),
    textCase: normalizeTextCase(raw.textCase),
    align: normalizeAlign(raw.align),
    fill: normalizeFill(raw.fill, "#ffffff"),
    stroke:
      strokeRaw && typeof strokeRaw === "object"
        ? { color: color(strokeRaw.color, "#000000"), width: num(strokeRaw.width, 0.002, 0, 0.2) }
        : null,
    background:
      backgroundRaw && typeof backgroundRaw === "object"
        ? {
            fill: normalizeFill(backgroundRaw.fill, "#000000"),
            padding: {
              top: num(
                (backgroundRaw.padding as Record<string, unknown>)?.top,
                DEFAULT_TEXT_BACKGROUND_PADDING.top,
                0,
                4,
              ),
              right: num(
                (backgroundRaw.padding as Record<string, unknown>)?.right,
                DEFAULT_TEXT_BACKGROUND_PADDING.right,
                0,
                4,
              ),
              bottom: num(
                (backgroundRaw.padding as Record<string, unknown>)?.bottom,
                DEFAULT_TEXT_BACKGROUND_PADDING.bottom,
                0,
                4,
              ),
              left: num(
                (backgroundRaw.padding as Record<string, unknown>)?.left,
                DEFAULT_TEXT_BACKGROUND_PADDING.left,
                0,
                4,
              ),
            },
          }
        : null,
    underline: bool(raw.underline),
    strikethrough: bool(raw.strikethrough),
  };
}

/** Drops entries that cannot be understood rather than rendering something wrong. */
export function normalizeLayers(input: unknown): Layer[] {
  if (!Array.isArray(input)) return [];
  const layers: Layer[] = [];
  for (const entry of input) {
    const layer = normalizeLayer(entry);
    if (layer) layers.push(layer);
  }
  return layers;
}
