/// <reference lib="webworker" />

import init, { ImageProcessor } from "@/wasm/export_wasm";
import type { ExportOptions } from "@/types/Export.d.ts";

const initializationPromise: Promise<void> | null = null;
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
  return new Promise((resolve, reject) => {
    init()
      .then(() => {
        self.postMessage({ type: "WASM_READY" });
        resolve();
      })
      .catch((error: unknown) => {
        const errMsg = (error as Error).message ?? "Unknown worker error";
        console.error("Error initializing export WebAssembly module:", error);
        self.postMessage({ type: "ERROR", payload: { error: errMsg } });
        reject(new Error(errMsg));
      });
  });
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

    if (
      options.format === "original" &&
      !options.maxWidth &&
      !options.maxHeight
    ) {
      const filename = options.filename || extractFilenameFromUrl(imageUrl);
      return { blob: imageBlob, filename };
    }

    if (signal.aborted) throw new Error("Operation aborted");
    self.postMessage({ type: "PROGRESS", payload: { processed: 50 } });

    const imageArrayBuffer = await imageBlob.arrayBuffer();
    const imageBytes = new Uint8Array(imageArrayBuffer);
    const processor = new ImageProcessor();

    const loadSuccess = processor.load_from_bytes(imageBytes);
    if (!loadSuccess) {
      throw new Error("Failed to load image in WASM module");
    }
    self.postMessage({ type: "PROGRESS", payload: { processed: 70 } });

    const wasmOptions = {
      format: options.format,
      quality: options.quality,
      max_width: options.maxWidth || null,
      max_height: options.maxHeight || null,
      filename: options.filename || null,
    };

    const result = processor.export_image(wasmOptions);
    self.postMessage({ type: "PROGRESS", payload: { processed: 90 } });

    if (!result.success) {
      throw new Error(result.error || "Export failed");
    }

    const mimeType = getMimeType(options.format);
    const blob = new Blob([new Uint8Array(result.data)], { type: mimeType });
    const filename = result.filename || generateFilename(options);

    self.postMessage({ type: "PROGRESS", payload: { processed: 100 } });
    return { blob, filename };
  } catch (error: unknown) {
    if (error instanceof Error && error.name === "AbortError") {
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
        if (!(err instanceof Error && err.name === "AbortError")) {
          console.error("Error exporting image:", err);
          self.postMessage({
            type: "ERROR",
            error: (err as Error).message,
          });
        }
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
