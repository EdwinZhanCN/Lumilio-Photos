/**
 * EXIF extraction + formatting for the EXIF-driven border styles
 * (Frosted Info / Info Strip).
 *
 * Pure module: no DOM, no canvas. Safe to import from both the main thread
 * (Studio editor / panel) and the tool worker (border renderer).
 *
 * Input is the raw exiftool JSON object stored on the asset
 * (`GET /api/v1/assets/{id}/exif` -> `exif_raw`). exiftool emits
 * human-readable strings by default (e.g. FocalLength "16.1 mm",
 * ExposureTime "1/1000"), but we also tolerate numeric values.
 */

export interface BorderExif {
  /** Camera maker, e.g. "OLYMPUS IMAGING CORP." */
  make?: string;
  /** Camera model, e.g. "STYLUS1" / "Canon EOS R7" */
  model?: string;
  /** Lens model (optional, currently unused in the layouts) */
  lensModel?: string;
  /** Formatted focal length, e.g. "400mm" */
  focalLength?: string;
  /** Formatted aperture, e.g. "f/5.6" */
  aperture?: string;
  /** Formatted shutter speed, e.g. "1/640s" */
  shutter?: string;
  /** Formatted ISO, e.g. "ISO 1250" */
  iso?: string;
  /** Formatted capture time, e.g. "2014-01-23 14:57:18" */
  dateTime?: string;
}

type RawExif = Record<string, unknown> | null | undefined;

function firstValue(exif: RawExif, keys: string[]): unknown {
  if (!exif) return undefined;
  for (const key of keys) {
    const value = exif[key];
    if (value === null || value === undefined || value === "") continue;
    return value;
  }
  return undefined;
}

function scalarToString(value: unknown): string | undefined {
  if (typeof value === "string") return value.trim() || undefined;
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return undefined;
}

function firstString(exif: RawExif, keys: string[]): string | undefined {
  const value = firstValue(exif, keys);
  if (value === undefined) return undefined;
  if (Array.isArray(value)) {
    const joined = value
      .map((v) => scalarToString(v) ?? "")
      .filter(Boolean)
      .join(", ");
    return joined || undefined;
  }
  return scalarToString(value);
}

/** Parse the first number out of a string/number ("16.1 mm" -> 16.1). */
function toNumber(value: unknown): number | undefined {
  if (typeof value === "number") return Number.isFinite(value) ? value : undefined;
  if (typeof value === "string") {
    const match = value.match(/-?\d+(\.\d+)?/);
    if (match) {
      const n = Number(match[0]);
      return Number.isFinite(n) ? n : undefined;
    }
  }
  return undefined;
}

/** Trim a trailing ".0" so "400.0" reads as "400" but "16.1" stays. */
function trimDecimal(n: number, maxFractionDigits = 1): string {
  const rounded = Math.round(n * 10 ** maxFractionDigits) / 10 ** maxFractionDigits;
  return Number.isInteger(rounded) ? String(rounded) : String(rounded);
}

function formatFocalLength(value: unknown): string | undefined {
  const n = toNumber(value);
  if (n === undefined || n <= 0) return undefined;
  return `${trimDecimal(n)}mm`;
}

function formatAperture(value: unknown): string | undefined {
  const n = toNumber(value);
  if (n === undefined || n <= 0) return undefined;
  return `f/${trimDecimal(n, 1)}`;
}

function formatShutter(value: unknown): string | undefined {
  if (typeof value === "string") {
    const trimmed = value.trim();
    // exiftool typically gives "1/640" or "0.5" (seconds).
    if (/^\d+\/\d+$/.test(trimmed)) return `${trimmed}s`;
    const n = toNumber(trimmed);
    if (n === undefined || n <= 0) return undefined;
    return secondsToShutter(n);
  }
  const n = toNumber(value);
  if (n === undefined || n <= 0) return undefined;
  return secondsToShutter(n);
}

function secondsToShutter(seconds: number): string {
  if (seconds >= 1) return `${trimDecimal(seconds, 1)}s`;
  const denom = Math.round(1 / seconds);
  return `1/${denom}s`;
}

function formatIso(value: unknown): string | undefined {
  const n = toNumber(value);
  if (n === undefined || n <= 0) return undefined;
  return `ISO ${Math.round(n)}`;
}

function formatDateTime(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  // exiftool: "2014:01:23 14:57:18" (optionally with subseconds / timezone).
  const match = value
    .trim()
    .match(/^(\d{4}):(\d{2}):(\d{2})[ T](\d{2}:\d{2}:\d{2})/);
  if (!match) return value.trim() || undefined;
  return `${match[1]}-${match[2]}-${match[3]} ${match[4]}`;
}

/**
 * Build the structured, display-formatted EXIF the border layouts consume.
 */
export function extractBorderExif(exif: RawExif): BorderExif {
  return {
    make: firstString(exif, ["Make", "EXIF:Make", "Exif.Image.Make"]),
    model: firstString(exif, [
      "Model",
      "CameraModelName",
      "EXIF:Model",
      "Exif.Image.Model",
    ]),
    lensModel: firstString(exif, [
      "LensModel",
      "Lens",
      "EXIF:LensModel",
      "Exif.Photo.LensModel",
    ]),
    focalLength: formatFocalLength(
      firstValue(exif, ["FocalLength", "EXIF:FocalLength", "Exif.Photo.FocalLength"]),
    ),
    aperture: formatAperture(
      firstValue(exif, ["FNumber", "Aperture", "EXIF:FNumber", "Exif.Photo.FNumber"]),
    ),
    shutter: formatShutter(
      firstValue(exif, [
        "ExposureTime",
        "ShutterSpeed",
        "ShutterSpeedValue",
        "EXIF:ExposureTime",
        "Exif.Photo.ExposureTime",
      ]),
    ),
    iso: formatIso(
      firstValue(exif, [
        "ISO",
        "ISOSpeed",
        "ISOSpeedRatings",
        "EXIF:ISO",
        "Exif.Photo.ISOSpeedRatings",
      ]),
    ),
    dateTime: formatDateTime(
      firstValue(exif, [
        "DateTimeOriginal",
        "CreateDate",
        "EXIF:DateTimeOriginal",
        "Exif.Photo.DateTimeOriginal",
      ]),
    ),
  };
}

/** The four shooting parameters, in display order. */
export function shootingParams(exif: BorderExif): string[] {
  return [exif.focalLength, exif.aperture, exif.shutter, exif.iso].filter(
    (v): v is string => Boolean(v),
  );
}

/** The camera identity used as the headline label. */
export function cameraLabel(exif: BorderExif): string | undefined {
  return exif.model ?? exif.make;
}

/**
 * Whether there is enough EXIF to render an EXIF-driven border.
 *
 * Threshold: a camera identity (model or make) AND at least one shooting
 * parameter. Anything less is rejected so we never produce an empty card.
 */
export function hasSufficientExif(exif: BorderExif): boolean {
  return Boolean(cameraLabel(exif)) && shootingParams(exif).length >= 1;
}
