export type BorderMode = "COLORED" | "FROSTED" | "VIGNETTE";

export type BorderParams = {
  mode: BorderMode;
  border_width: number;
  color_hex: string;
  blur_sigma: number;
  brightness_adjustment: number;
  corner_radius: number;
  strength: number;
  jpeg_quality: number;
};

export const DEFAULT_PARAMS: BorderParams = {
  mode: "COLORED",
  border_width: 20,
  color_hex: "#ffffff",
  blur_sigma: 15,
  brightness_adjustment: -40,
  corner_radius: 30,
  strength: 0.7,
  jpeg_quality: 90,
};

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

function toNumber(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function toMode(value: unknown): BorderMode {
  if (value === "FROSTED" || value === "VIGNETTE" || value === "COLORED") {
    return value;
  }
  return DEFAULT_PARAMS.mode;
}

function toHexColor(value: unknown): string {
  if (typeof value !== "string") return DEFAULT_PARAMS.color_hex;
  return /^#[0-9a-fA-F]{6}$/.test(value) ? value : DEFAULT_PARAMS.color_hex;
}

export function normalizeParams(raw: Record<string, unknown>): BorderParams {
  const mode = toMode(raw.mode);
  return {
    mode,
    border_width: clamp(toNumber(raw.border_width, DEFAULT_PARAMS.border_width), 1, 200),
    color_hex: toHexColor(raw.color_hex),
    blur_sigma: clamp(toNumber(raw.blur_sigma, DEFAULT_PARAMS.blur_sigma), 1, 50),
    brightness_adjustment: clamp(
      toNumber(raw.brightness_adjustment, DEFAULT_PARAMS.brightness_adjustment),
      -100,
      100,
    ),
    corner_radius: clamp(toNumber(raw.corner_radius, DEFAULT_PARAMS.corner_radius), 0, 100),
    strength: clamp(toNumber(raw.strength, DEFAULT_PARAMS.strength), 0.1, 2.0),
    jpeg_quality: clamp(toNumber(raw.jpeg_quality, DEFAULT_PARAMS.jpeg_quality), 1, 100),
  };
}

export function hexToRgb(hex: string): { r: number; g: number; b: number } {
  return {
    r: parseInt(hex.slice(1, 3), 16),
    g: parseInt(hex.slice(3, 5), 16),
    b: parseInt(hex.slice(5, 7), 16),
  };
}
