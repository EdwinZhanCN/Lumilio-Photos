/**
 * EXIF preservation for Studio exports.
 *
 * A Studio export is re-encoded from a canvas, which drops all metadata. Nobody
 * wants an edit that strips the camera, lens, exposure, date and GPS off their
 * photo, so this copies the meaningful tags from the ORIGINAL file onto the
 * exported bytes — with two corrections: Orientation is reset to 1 (rotation is
 * already baked into the pixels, so a viewer must not rotate again) and the
 * pixel-dimension tags are set to the exported size.
 *
 * The heavy ExifTool WASM is imported dynamically inside {@link preserveExif}
 * so it lands in its own chunk (never the entry bundle) and is fetched only when
 * someone actually exports. {@link buildPreservedTags} is pure and holds the
 * policy — what to keep and what to override — so it is unit tested without the
 * WASM.
 *
 * PNG carries no EXIF; it is returned untouched. JPEG and WebP are handled.
 */

// The package loads its wasm with `new URL(<variable>, import.meta.url)`, which
// the bundler cannot statically emit — so we emit it ourselves with `?url` and
// redirect ExifTool's loader to it via the `fetch` option below. The hashed
// filename is pinned to the installed version; bump it if the dependency updates.
import zeroperlWasmUrl from "@colorhythm/exiftool-wasm/dist/esm/zeroperl-mqcadjqm.wasm?url";

export type ExifFormat = "image/jpeg" | "image/png" | "image/webp";

/** Serve ExifTool's wasm request from the asset we actually emitted. */
function wasmFetch(...args: unknown[]): Promise<Response> {
  const first = args[0];
  let url = "";
  if (typeof first === "string") url = first;
  else if (first instanceof URL) url = first.href;
  else if (first instanceof Request) url = first.url;
  if (url.includes("zeroperl") && url.endsWith(".wasm")) return fetch(zeroperlWasmUrl);
  return fetch(first as RequestInfo, args[1] as RequestInit | undefined);
}

export type ExifTagValue = string | number | boolean | Array<string | number | boolean>;
export type ExifTags = Record<string, ExifTagValue>;

/** Descriptive EXIF tags worth carrying onto an edited copy. */
const EXIF_KEYS = [
  "Make",
  "Model",
  "LensMake",
  "LensModel",
  "FNumber",
  "ExposureTime",
  "ISO",
  "FocalLength",
  "FocalLengthIn35mmFormat",
  "ExposureProgram",
  "ExposureCompensation",
  "MeteringMode",
  "Flash",
  "WhiteBalance",
  "DateTimeOriginal",
  "CreateDate",
  "ModifyDate",
  "OffsetTime",
  "OffsetTimeOriginal",
  "Artist",
  "Copyright",
  "ImageDescription",
  "Software",
] as const;

const GPS_KEYS = [
  "GPSLatitude",
  "GPSLongitude",
  "GPSAltitude",
  "GPSLatitudeRef",
  "GPSLongitudeRef",
  "GPSAltitudeRef",
  "GPSDateStamp",
  "GPSTimeStamp",
] as const;

function isTagValue(value: unknown): value is ExifTagValue {
  if (Array.isArray(value)) return true;
  const t = typeof value;
  return t === "string" || t === "number" || t === "boolean";
}

/**
 * Turn a parsed ExifTool tag object into the group-qualified tags to write onto
 * the export. Copies an allowlist of descriptive tags, then forces Orientation
 * to upright and the dimensions to the exported size.
 */
export function buildPreservedTags(
  source: Record<string, unknown>,
  width: number,
  height: number,
): ExifTags {
  const tags: ExifTags = {};
  for (const key of EXIF_KEYS) {
    const value = source[key];
    if (value != null && isTagValue(value)) tags[`EXIF:${key}`] = value;
  }
  for (const key of GPS_KEYS) {
    const value = source[key];
    if (value != null && isTagValue(value)) tags[`GPS:${key}`] = value;
  }
  // Rotation is baked into the pixels — the copy must read as upright.
  tags["EXIF:Orientation"] = 1;
  tags["EXIF:ExifImageWidth"] = width;
  tags["EXIF:ExifImageHeight"] = height;
  return tags;
}

export type PreserveExifOptions = {
  format: ExifFormat;
  width: number;
  height: number;
};

/**
 * Return `exportBlob` with the original's EXIF copied onto it. Best-effort:
 * PNG (which has no EXIF) and any failure return the export unchanged, because a
 * missing tag must never block a download.
 */
export async function preserveExif(
  exportBlob: Blob,
  originalBlob: Blob,
  options: PreserveExifOptions,
): Promise<Blob> {
  if (options.format === "image/png") return exportBlob;

  try {
    const { parseMetadata, writeMetadata } = await import("@colorhythm/exiftool-wasm");

    const parsed = await parseMetadata(
      { name: "source", data: originalBlob },
      { args: ["-json", "-n"], fetch: wasmFetch },
    );
    if (!parsed.success) return exportBlob;

    const rows = JSON.parse(parsed.data) as Array<Record<string, unknown>>;
    const source = Array.isArray(rows) ? rows[0] : null;
    if (!source) return exportBlob;

    const tags = buildPreservedTags(source, options.width, options.height);
    const name = options.format === "image/webp" ? "export.webp" : "export.jpg";
    // No -overwrite_original: it makes ExifTool erase the input in the wasm
    // virtual FS (which fails); writeMetadata returns the modified bytes anyway.
    const written = await writeMetadata({ name, data: exportBlob }, tags, {
      args: ["-n", "-m"],
      fetch: wasmFetch,
    });
    if (!written.success) return exportBlob;

    return new Blob([written.data], { type: options.format });
  } catch (error) {
    console.warn("[studio] EXIF preservation failed; exporting without it", error);
    return exportBlob;
  }
}
