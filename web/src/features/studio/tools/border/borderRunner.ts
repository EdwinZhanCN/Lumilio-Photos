import { isExifBorderMode, normalizeParams } from "./types";
import { hasSufficientExif, type BorderExif } from "./exifInfo";
import {
  renderExifBorder,
  type ExifBorderInput,
  type ExifBorderMode,
} from "./exifBorderRenderer";
import {
  renderBasicBorder,
  type BasicBorderMode,
} from "./basicBorders";

type ProgressHelpers = {
  reportProgress?: (processed: number, total: number) => void;
};

function assertNotAborted(signal: AbortSignal): void {
  if (signal.aborted) {
    throw new Error("Operation aborted");
  }
}

function outputFileName(file: File): string {
  const base = file.name.replace(/\.[^.]+$/, "");
  return `${base}-border.png`;
}

async function runExifBorderTransform(
  file: File,
  mode: ExifBorderMode,
  params: ReturnType<typeof normalizeParams>,
  rawParams: Record<string, unknown>,
  signal: AbortSignal,
  helpers?: ProgressHelpers,
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  helpers?.reportProgress?.(1, 3);

  const exif = (rawParams.exif as BorderExif | undefined) ?? {};
  const brandText =
    typeof rawParams.brandText === "string" && rawParams.brandText
      ? rawParams.brandText
      : null;
  const logo =
    typeof ImageBitmap !== "undefined" && rawParams.logo instanceof ImageBitmap
      ? rawParams.logo
      : null;

  // Guard: the main thread pre-validates and disables Apply, but reject here
  // too so the worker never produces an empty card.
  if (!hasSufficientExif(exif)) {
    throw new Error(
      "This border needs camera EXIF (model plus at least one of focal length, aperture, shutter, or ISO), which this photo is missing.",
    );
  }

  const input: ExifBorderInput = {
    mode,
    style: {
      blurSigma: params.blur_sigma,
      brightness: params.brightness_adjustment,
      cornerRadius: params.corner_radius,
    },
    exif,
    logo,
    brandText,
  };

  assertNotAborted(signal);
  helpers?.reportProgress?.(2, 3);

  const outputBytes = await renderExifBorder(file, input, signal);

  assertNotAborted(signal);
  helpers?.reportProgress?.(3, 3);
  return {
    bytes: new Uint8Array(outputBytes),
    mimeType: "image/png",
    fileName: outputFileName(file),
  };
}

async function runBasicBorderTransform(
  file: File,
  mode: BasicBorderMode,
  params: ReturnType<typeof normalizeParams>,
  signal: AbortSignal,
  helpers?: ProgressHelpers,
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  helpers?.reportProgress?.(1, 3);
  assertNotAborted(signal);
  helpers?.reportProgress?.(2, 3);

  const outputBytes = await renderBasicBorder(
    file,
    mode,
    {
      borderWidth: params.border_width,
      colorHex: params.color_hex,
      blurSigma: params.blur_sigma,
      brightness: params.brightness_adjustment,
      cornerRadius: params.corner_radius,
      strength: params.strength,
    },
    signal,
  );

  assertNotAborted(signal);
  helpers?.reportProgress?.(3, 3);
  return {
    bytes: new Uint8Array(outputBytes),
    mimeType: "image/png",
    fileName: outputFileName(file),
  };
}

/**
 * Border tool entry point. Every mode renders on an OffscreenCanvas in the
 * tool worker (no wasm): COLORED / VIGNETTE / FROSTED via `basicBorders`, and
 * the EXIF-driven FROSTED_INFO / INFO_STRIP via `exifBorderRenderer`.
 */
export async function runBorderTransform(
  file: File,
  signal: AbortSignal,
  rawParams: Record<string, unknown>,
  helpers?: ProgressHelpers,
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  const params = normalizeParams(rawParams);

  if (isExifBorderMode(params.mode)) {
    return runExifBorderTransform(file, params.mode, params, rawParams, signal, helpers);
  }
  return runBasicBorderTransform(file, params.mode, params, signal, helpers);
}
