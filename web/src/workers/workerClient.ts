/**
 * A unified client to manage and interact with all web workers in the application.
 * This class provides a clean, promise-based API for computationally expensive tasks,
 * abstracting away the underlying `postMessage` communication.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */
export class AppWorkerClient {
  private generateThumbnailworker: Worker;
  private hashAssetsworker: Worker;
  private generateBorderworker: Worker;
  private exportWorker: Worker;
  private extractExifWorker: Worker;

  private eventTarget: EventTarget;

  constructor() {
    this.generateThumbnailworker = new Worker(
      new URL("./thumbnail.worker.ts", import.meta.url),
      { type: "module" },
    );
    this.hashAssetsworker = new Worker(
      new URL("./hash.worker.ts", import.meta.url),
      { type: "module" },
    );
    this.generateBorderworker = new Worker(
      new URL("./border.worker.ts", import.meta.url),
      { type: "module" },
    );
    this.exportWorker = new Worker(
      new URL("./export.worker.ts", import.meta.url),
      { type: "module" },
    );
    this.extractExifWorker = new Worker(
      new URL("./exif.worker.ts", import.meta.url),
      { type: "module" },
    );

    this.eventTarget = new EventTarget();
  }

  /**
   * Adds a progress listener that can be used by any worker task.
   * @param callback - Function to handle progress events.
   * @returns A function to remove the event listener.
   */
  addProgressListener(callback: (detail: any) => void): () => void {
    const handler = (e: CustomEvent) => callback(e.detail);
    this.eventTarget.addEventListener("progress", handler as EventListener);
    return () =>
      this.eventTarget.removeEventListener(
        "progress",
        handler as EventListener,
      );
  }

  // --- Thumbnail Generation ---
  async generateThumbnail(data: {
    files: FileList | File[];
    batchIndex: number;
    startIndex: number;
  }): Promise<{ batchIndex: number; results: any[]; status: string }> {
    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "BATCH_COMPLETE":
            resolve({
              batchIndex: e.data.payload.batchIndex,
              results: e.data.payload.results,
              status: "complete",
            });
            this.generateThumbnailworker.removeEventListener(
              "message",
              handler,
            );
            break;
          case "ERROR":
            const error = new Error(e.data.payload.error);
            error.name = e.data.payload.errorName;
            error.stack = e.data.payload.errorStack;
            this.generateThumbnailworker.removeEventListener(
              "message",
              handler,
            );
            reject(error);
            break;
          case "PROGRESS":
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };
      this.generateThumbnailworker.addEventListener("message", handler);
      this.generateThumbnailworker.postMessage({
        type: "GENERATE_THUMBNAIL",
        data,
      });
    });
  }

  abortGenerateThumbnail() {
    this.generateThumbnailworker.postMessage({ type: "ABORT" });
  }

  // --- Hash Generation ---
  async generateHash(data: FileList | File[]): Promise<{
    hashResults: Array<{ index: number; hash: string }>;
    status: string;
  }> {
    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "HASH_COMPLETE":
            resolve({
              hashResults: e.data.hashResult,
              status: "complete",
            });
            this.hashAssetsworker.removeEventListener("message", handler);
            break;
          case "ERROR":
            this.hashAssetsworker.removeEventListener("message", handler);
            reject(
              new Error(e.data.payload?.error || "WASM initialization failed"),
            );
            break;
          case "PROGRESS":
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };
      this.hashAssetsworker.addEventListener("message", handler);
      const filesArray = Array.isArray(data) ? data : Array.from(data);
      this.hashAssetsworker.postMessage({
        type: "GENERATE_HASH",
        data: filesArray,
      });
    });
  }

  abortGenerateHash() {
    this.hashAssetsworker.postMessage({ type: "ABORT" });
  }

  // --- Border Generation ---
  async generateBorders(
    files: File[],
    option: "COLORED" | "FROSTED" | "VIGNETTE",
    param: object,
  ): Promise<{
    [uuid: string]: {
      originalFileName: string;
      borderedFileURL?: string;
      error?: string;
    };
  }> {
    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "GENERATE_BORDER_COMPLETE":
            this.generateBorderworker.removeEventListener("message", handler);
            resolve(e.data.data);
            break;
          case "ERROR":
            this.generateBorderworker.removeEventListener("message", handler);
            reject(new Error(e.data.payload.error));
            break;
        }
      };
      this.generateBorderworker.addEventListener("message", handler);
      this.generateBorderworker.postMessage({
        type: "GENERATE_BORDER",
        data: { files },
        option,
        param,
      });
    });
  }

  abortGenerateBorders() {
    this.generateBorderworker.postMessage({ type: "ABORT" });
  }

  // --- Image Export ---
  async exportImage(
    imageUrl: string,
    options: {
      format: "jpeg" | "png" | "webp" | "original";
      quality: number;
      maxWidth?: number;
      maxHeight?: number;
      filename?: string;
    },
  ): Promise<{
    status: "complete" | "error";
    blob?: Blob;
    filename?: string;
    error?: string;
  }> {
    return new Promise((resolve) => {
      const handleMessage = (event: MessageEvent) => {
        const { type, result, error, payload } = event.data;
        if (type === "EXPORT_COMPLETE") {
          this.exportWorker.removeEventListener("message", handleMessage);
          resolve({
            status: "complete",
            blob: result.blob,
            filename: result.filename,
          });
        } else if (type === "ERROR") {
          this.exportWorker.removeEventListener("message", handleMessage);
          resolve({ status: "error", error: error || "Export failed" });
        } else if (type === "PROGRESS") {
          this.eventTarget.dispatchEvent(
            new CustomEvent("progress", {
              detail: { processed: payload?.processed || 0 },
            }),
          );
        }
      };
      this.exportWorker.addEventListener("message", handleMessage);
      this.exportWorker.postMessage({
        type: "EXPORT_IMAGE",
        data: { imageUrl, options },
      });
    });
  }

  abortExportImage() {
    this.exportWorker.postMessage({ type: "ABORT" });
  }

  // --- EXIF Extraction ---
  async extractExif(files: FileList | File[]): Promise<{
    exifResults: Array<{ index: number; exifData: Record<string, any> }>;
    status: string;
  }> {
    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "EXIF_COMPLETE":
            this.extractExifWorker.removeEventListener("message", handler);
            resolve({
              exifResults: e.data.payload.results,
              status: "complete",
            });
            break;
          case "ERROR":
            this.extractExifWorker.removeEventListener("message", handler);
            reject(new Error(e.data.payload.error));
            break;
          case "PROGRESS":
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };
      this.extractExifWorker.addEventListener("message", handler);
      const filesArray = Array.isArray(files) ? files : Array.from(files);
      this.extractExifWorker.postMessage({
        type: "EXTRACT_EXIF",
        data: { files: filesArray },
      });
    });
  }

  abortExtractExif() {
    this.extractExifWorker.postMessage({ type: "ABORT" });
  }

  // --- Lifecycle Management ---
  /**
   * Terminates all active workers to clean up resources.
   * This should be called when the application is unmounting.
   */
  terminateAllWorkers(): void {
    this.generateThumbnailworker.terminate();
    this.hashAssetsworker.terminate();
    this.generateBorderworker.terminate();
    this.exportWorker.terminate();
    this.extractExifWorker.terminate();
    console.log("All workers terminated.");
  }
}
