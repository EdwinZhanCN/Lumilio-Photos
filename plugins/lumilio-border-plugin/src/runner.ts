import { hexToRgb, normalizeParams, type BorderMode } from "./types";

type BorderWasmModule = {
  default: (input?: string | URL | Request | Response) => Promise<unknown>;
  add_colored_border: (
    image: Uint8Array,
    borderWidth: number,
    r: number,
    g: number,
    b: number,
    qualityHint: number,
  ) => Uint8Array;
  create_frosted_border: (
    image: Uint8Array,
    blurSigma: number,
    brightnessAdjustment: number,
    cornerRadius: number,
    qualityHint: number,
  ) => Uint8Array;
  add_vignette_border: (
    image: Uint8Array,
    strength: number,
    qualityHint: number,
  ) => Uint8Array;
};

const PNG_SIGNATURE = [0x89, 0x50, 0x4e, 0x47];
const MODE_PREPROCESS_LIMITS: Record<BorderMode, { maxSide: number; maxPixels: number }> = {
  COLORED: { maxSide: Number.MAX_SAFE_INTEGER, maxPixels: Number.MAX_SAFE_INTEGER },
  // Frosted mode has full-frame blur, so use a lower threshold to keep latency predictable.
  FROSTED: { maxSide: 3072, maxPixels: 9_000_000 },
  VIGNETTE: { maxSide: 4096, maxPixels: 16_000_000 },
};
const OUTPUT_QUALITY_HINT = 90;
const WASM_NATIVE_INPUT_MIME = new Set(["image/png", "image/jpeg", "image/webp"]);

let wasmPromise: Promise<BorderWasmModule> | null = null;

function assertNotAborted(signal: AbortSignal): void {
  if (signal.aborted) {
    throw new Error("Operation aborted");
  }
}

function isLikelyDecodeError(message: string): boolean {
  const lower = message.toLowerCase();
  return (
    lower.includes("could not guess format") ||
    lower.includes("failed to load image") ||
    lower.includes("decode") ||
    lower.includes("unsupported")
  );
}

function normalizeMimeType(mimeType: string): string {
  return mimeType.trim().toLowerCase().split(";")[0] ?? "";
}

function shouldPreTranscodeForMime(file: File): boolean {
  const normalized = normalizeMimeType(file.type);
  if (!normalized) return false;
  return !WASM_NATIVE_INPUT_MIME.has(normalized);
}

function detectMimeType(bytes: Uint8Array): "image/png" | "application/octet-stream" {
  if (
    bytes.length >= 4 &&
    bytes[0] === PNG_SIGNATURE[0] &&
    bytes[1] === PNG_SIGNATURE[1] &&
    bytes[2] === PNG_SIGNATURE[2] &&
    bytes[3] === PNG_SIGNATURE[3]
  ) {
    return "image/png";
  }

  return "application/octet-stream";
}

function fileExtensionFromMime(mimeType: string): string {
  if (mimeType === "image/png") return "png";
  return "bin";
}

function canUseCanvasTranscode(): boolean {
  return typeof createImageBitmap === "function" && typeof OffscreenCanvas !== "undefined";
}

async function getBorderWasm(): Promise<BorderWasmModule> {
  if (!wasmPromise) {
    wasmPromise = (async () => {
      const mod = (await import("./vendor/border_wasm.js")) as BorderWasmModule;
      const wasmUrlCandidates = [
        new URL("./vendor/border_wasm_bg.wasm", import.meta.url),
        new URL("./border_wasm_bg.wasm", import.meta.url),
      ];

      let lastError: unknown = null;
      for (const wasmUrl of wasmUrlCandidates) {
        try {
          await mod.default(wasmUrl);
          return mod;
        } catch (error) {
          lastError = error;
        }
      }

      throw new Error(
        `Failed to initialize border wasm from all URL candidates: ${String(lastError)}`,
      );
    })();
  }
  return wasmPromise;
}

function runWasmTransform(
  wasm: BorderWasmModule,
  mode: BorderMode,
  inputBytes: Uint8Array,
  params: ReturnType<typeof normalizeParams>,
): Uint8Array {
  if (mode === "COLORED") {
    const rgb = hexToRgb(params.color_hex);
    return wasm.add_colored_border(
      inputBytes,
      params.border_width,
      rgb.r,
      rgb.g,
      rgb.b,
      OUTPUT_QUALITY_HINT,
    );
  }

  if (mode === "FROSTED") {
    return wasm.create_frosted_border(
      inputBytes,
      params.blur_sigma,
      params.brightness_adjustment,
      params.corner_radius,
      OUTPUT_QUALITY_HINT,
    );
  }

  return wasm.add_vignette_border(
    inputBytes,
    params.strength,
    OUTPUT_QUALITY_HINT,
  );
}

async function transcodeWithCanvas(
  fileOrBitmap: File | ImageBitmap,
  signal: AbortSignal,
  options: {
    mimeType: "image/png" | "image/jpeg";
    quality?: number;
    maxSide?: number;
  },
): Promise<Uint8Array> {
  if (!canUseCanvasTranscode()) {
    throw new Error("Canvas transcoding is not supported in this worker runtime");
  }

  assertNotAborted(signal);
  const ownsBitmap = fileOrBitmap instanceof File;
  const bitmap = ownsBitmap ? await createImageBitmap(fileOrBitmap) : fileOrBitmap;

  try {
    assertNotAborted(signal);
    const maxSide = options.maxSide ?? Math.max(bitmap.width, bitmap.height);
    const sourceMaxSide = Math.max(bitmap.width, bitmap.height);
    const scale = sourceMaxSide > maxSide ? maxSide / sourceMaxSide : 1;

    const targetWidth = Math.max(1, Math.round(bitmap.width * scale));
    const targetHeight = Math.max(1, Math.round(bitmap.height * scale));

    const canvas = new OffscreenCanvas(targetWidth, targetHeight);
    const ctx = canvas.getContext("2d", {
      alpha: options.mimeType === "image/png",
      desynchronized: true,
      willReadFrequently: false,
    });

    if (!ctx) {
      throw new Error("Failed to create 2d canvas context");
    }

    ctx.drawImage(bitmap, 0, 0, targetWidth, targetHeight);
    const blob = await canvas.convertToBlob({
      type: options.mimeType,
      quality: options.quality,
    });

    assertNotAborted(signal);
    return new Uint8Array(await blob.arrayBuffer());
  } finally {
    if (ownsBitmap && typeof bitmap.close === "function") {
      bitmap.close();
    }
  }
}

async function maybeDownscaleForHeavyMode(
  file: File,
  mode: BorderMode,
  signal: AbortSignal,
): Promise<{ bytes: Uint8Array; preTranscoded: boolean } | null> {
  const shouldInspectDimensions = mode !== "COLORED";
  const shouldPreTranscodeByMime = shouldPreTranscodeForMime(file);

  if (!shouldInspectDimensions && !shouldPreTranscodeByMime) {
    return null;
  }
  if (!canUseCanvasTranscode()) {
    return null;
  }

  assertNotAborted(signal);
  const bitmap = await createImageBitmap(file);
  try {
    const width = bitmap.width;
    const height = bitmap.height;
    const totalPixels = width * height;
    const maxSide = Math.max(width, height);
    const limits = MODE_PREPROCESS_LIMITS[mode];
    const needsDownscale =
      shouldInspectDimensions &&
      (totalPixels > limits.maxPixels || maxSide > limits.maxSide);

    if (!needsDownscale && !shouldPreTranscodeByMime) {
      return null;
    }

    const bytes = await transcodeWithCanvas(bitmap, signal, {
      // Pre-normalize to PNG for better wasm decode stability.
      mimeType: "image/png",
      maxSide: needsDownscale ? limits.maxSide : undefined,
    });
    return { bytes, preTranscoded: true };
  } finally {
    if (typeof bitmap.close === "function") {
      bitmap.close();
    }
  }
}

export async function run(
  ctx: { inputFile: File; signal: AbortSignal },
  rawParams: Record<string, unknown>,
  helpers?: { reportProgress?: (processed: number, total: number) => void },
): Promise<{ bytes: Uint8Array; mimeType: string; fileName: string }> {
  helpers?.reportProgress?.(1, 6);

  const wasm = await getBorderWasm();
  const params = normalizeParams(rawParams);

  assertNotAborted(ctx.signal);
  helpers?.reportProgress?.(2, 6);

  let usedPreTranscode = false;
  const preprocessed = await maybeDownscaleForHeavyMode(
    ctx.inputFile,
    params.mode,
    ctx.signal,
  );
  const inputBytes = preprocessed
    ? new Uint8Array(preprocessed.bytes)
    : new Uint8Array(await ctx.inputFile.arrayBuffer());
  usedPreTranscode = preprocessed?.preTranscoded ?? false;

  assertNotAborted(ctx.signal);
  helpers?.reportProgress?.(3, 6);

  let outputBytes: Uint8Array;
  try {
    outputBytes = runWasmTransform(wasm, params.mode, inputBytes, params);
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    if (!isLikelyDecodeError(message)) {
      throw error;
    }
    if (usedPreTranscode) {
      throw new Error(
        `Border plugin failed to decode even after canvas preprocessing: ${message}`,
      );
    }

    // Fallback: browser decode + transcode to PNG, then retry wasm.
    const fallbackBytes = await transcodeWithCanvas(ctx.inputFile, ctx.signal, {
      mimeType: "image/png",
    });
    assertNotAborted(ctx.signal);
    outputBytes = runWasmTransform(wasm, params.mode, fallbackBytes, params);
  }

  assertNotAborted(ctx.signal);
  helpers?.reportProgress?.(5, 6);

  const mimeType = detectMimeType(outputBytes);
  if (mimeType !== "image/png") {
    throw new Error("Border plugin expected PNG output from wasm encoder");
  }
  const base = ctx.inputFile.name.replace(/\.[^.]+$/, "");
  const ext = fileExtensionFromMime(mimeType);

  helpers?.reportProgress?.(6, 6);
  return {
    bytes: new Uint8Array(outputBytes),
    mimeType,
    fileName: `${base}-border.${ext}`,
  };
}

export default {
  run,
};
