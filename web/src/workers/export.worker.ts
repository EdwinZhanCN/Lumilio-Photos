/// <reference lib="webworker" />

import init, { ImageProcessor } from "../wasm/export/export_wasm";
import type { ExportOptions } from "@/types/Export.d.ts";

let initializationPromise: Promise<void> | null = null;
let abortController = new AbortController();

interface WorkerMessage {
  type: "ABORT" | "EXPORT_IMAGE";
  data?: {
    imageUrl: string;
    options: ExportOptions;
  };
}

interface WorkerExportResult {
  blob: Blob;
  filename: string;
  error?: string;
}

function initialize(): Promise<void> {
  if (initializationPromise) {
    return initializationPromise;
  }

  initializationPromise = new Promise((resolve, reject) => {
    init().then(resolve).catch(reject);
  })
    .then(() => {
      self.postMessage({ type: "WASM_READY" });
    })
    .catch((error: unknown) => {
      const errMsg = (error as Error).message ?? "Unknown worker error";
      console.error("Error initializing export WebAssembly module:", error);
      self.postMessage({ type: "ERROR", payload: { error: errMsg } });
      initializationPromise = null;
      throw new Error(errMsg);
    });

  return initializationPromise;
}

function isAbortError(error: unknown): boolean {
  return error instanceof Error && error.name === "AbortError";
}

function isOperationAbortedError(error: unknown): boolean {
  return (
    isAbortError(error) ||
    (error instanceof Error && error.message.toLowerCase().includes("aborted"))
  );
}

function getOriginalFilename(imageUrl: string, options: ExportOptions): string {
  if (options.filename) {
    return options.filename;
  }

  return extractFilenameFromUrl(imageUrl);
}

function buildWasmOptions(options: ExportOptions) {
  // Runtime supports only lossless WebP currently.
  const quality = options.format === "webp" ? 1 : options.quality;

  return {
    format: options.format,
    quality,
    max_width: options.maxWidth || null,
    max_height: options.maxHeight || null,
    filename: options.filename || null,
  };
}

function getResultBytes(resultData: unknown): Uint8Array<ArrayBuffer> {
  if (resultData instanceof Uint8Array) {
    return Uint8Array.from(resultData);
  }
  if (resultData instanceof ArrayBuffer) {
    return new Uint8Array(resultData);
  }
  if (ArrayBuffer.isView(resultData)) {
    const view = new Uint8Array(
      resultData.buffer.slice(
        resultData.byteOffset,
        resultData.byteOffset + resultData.byteLength,
      ),
    );
    return Uint8Array.from(view);
  }
  throw new Error("Invalid export result bytes");
}

async function processWithWasm(
  imageBlob: Blob,
  options: ExportOptions,
  signal: AbortSignal,
): Promise<WorkerExportResult> {
  if (signal.aborted) {
    throw new Error("Operation aborted");
  }

  const imageArrayBuffer = await imageBlob.arrayBuffer();
  const imageBytes = new Uint8Array(imageArrayBuffer);

  const processor = new ImageProcessor();
  try {
    const loadSuccess = processor.load_from_bytes(imageBytes);
    if (!loadSuccess) {
      throw new Error("Failed to load image in WASM module");
    }
    self.postMessage({ type: "PROGRESS", payload: { processed: 70 } });

    const result = processor.export_image(buildWasmOptions(options));
    self.postMessage({ type: "PROGRESS", payload: { processed: 90 } });

    if (!result.success) {
      throw new Error(result.error || "Export failed");
    }
    if (!result.data) {
      throw new Error("Export produced no data");
    }

    const mimeType = getMimeType(options.format);
    const outputBytes = getResultBytes(result.data);
    const blob = new Blob([outputBytes], { type: mimeType });
    const filename = result.filename || generateFilename(options);

    self.postMessage({ type: "PROGRESS", payload: { processed: 100 } });
    return { blob, filename };
  } finally {
    processor.free();
  }
}

async function exportImage(
  imageUrl: string,
  options: ExportOptions,
  signal: AbortSignal,
): Promise<WorkerExportResult> {
  try {
    self.postMessage({ type: "PROGRESS", payload: { processed: 10 } });

    const response = await fetch(imageUrl, { signal });
    if (signal.aborted) throw new Error("Operation aborted");
    if (!response.ok) {
      throw new Error(`Failed to fetch image: ${response.statusText}`);
    }

    const imageBlob = await response.blob();
    self.postMessage({ type: "PROGRESS", payload: { processed: 30 } });

    if (options.format === "original") {
      return {
        blob: imageBlob,
        filename: getOriginalFilename(imageUrl, options),
      };
    }

    if (signal.aborted) throw new Error("Operation aborted");
    self.postMessage({ type: "PROGRESS", payload: { processed: 50 } });

    return processWithWasm(imageBlob, options, signal);
  } catch (error: unknown) {
    if (isAbortError(error)) {
      console.log("Export fetch aborted");
    } else {
      console.error(`Error exporting image from ${imageUrl}`, error);
    }
    throw error;
  }
}

function getMimeType(format: ExportOptions["format"]): string {
  switch (format) {
    case "jpeg":
      return "image/jpeg";
    case "png":
      return "image/png";
    case "webp":
      return "image/webp";
    default:
      return "image/jpeg";
  }
}

function getFileExtension(format: ExportOptions["format"]): string {
  switch (format) {
    case "jpeg":
      return "jpg";
    case "png":
      return "png";
    case "webp":
      return "webp";
    default:
      return "jpg";
  }
}

function generateFilename(options: ExportOptions): string {
  if (options.filename) {
    return options.filename;
  }
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  const extension = getFileExtension(options.format);
  return `lumilio-export-${timestamp}.${extension}`;
}

function extractFilenameFromUrl(url: string): string {
  try {
    const urlObj = new URL(url);
    const pathname = urlObj.pathname;
    const filename = pathname.split("/").pop();
    return filename || "download";
  } catch {
    return "download";
  }
}

self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
  const { type, data } = e.data;

  switch (type) {
    case "ABORT":
      abortController.abort();
      abortController = new AbortController();
      break;

    case "EXPORT_IMAGE":
      abortController = new AbortController();
      try {
        await initialize();
        if (!data) {
          throw new Error("No export data provided");
        }
        const result = await exportImage(
          data.imageUrl,
          data.options,
          abortController.signal,
        );
        self.postMessage({ type: "EXPORT_COMPLETE", result });
      } catch (err: unknown) {
        if (!isOperationAbortedError(err)) {
          console.error("Error exporting image:", err);
        }
        self.postMessage({
          type: "ERROR",
          error: isOperationAbortedError(err)
            ? "Operation aborted"
            : (err as Error).message,
        });
      }
      break;

    default:
      self.postMessage({
        type: "ERROR",
        error: `Unknown message type: ${type}`,
      });
      break;
  }
};
