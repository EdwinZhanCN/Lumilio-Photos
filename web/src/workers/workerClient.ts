/**
 * A unified client to manage and interact with all web workers in the application.
 * This class provides a clean, promise-based API for computationally expensive tasks,
 * abstracting away the underlying `postMessage` communication.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */

export type WorkerType = "thumbnail" | "hash" | "border" | "export" | "exif";

export interface WorkerClientOptions {
  preload?: WorkerType[];
}


export interface SingleHashResult {
  index: number;
  hash: string;
  file?: File; // 可选：把原始文件传回来方便后续处理
}

const THREADS = Math.max(1, navigator.hardwareConcurrency - 1);

export class AppWorkerClient {
  private generateThumbnailworker: Worker | null = null;
  private hashWorkers: Worker[] = [];
  private generateBorderworker: Worker | null = null;
  private exportWorker: Worker | null = null;
  private extractExifWorker: Worker | null = null;

  private eventTarget: EventTarget;

  constructor(options: WorkerClientOptions = {}) {
    this.eventTarget = new EventTarget();

    // Pre-load specified workers
    if (options.preload) {
      options.preload.forEach((workerType) => {
        this.getOrInitializeWorker(workerType);
      });
    }
  }

  /**
   * Get or initialize a worker on demand
   */
  private getOrInitializeWorker(type: WorkerType, index: number = 0): Worker {
    switch (type) {
      case "thumbnail":
        if (!this.generateThumbnailworker) {
          this.generateThumbnailworker = new Worker(
            new URL("./thumbnail.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.generateThumbnailworker;

      case "hash":
        if (!this.hashWorkers[index]) {
          this.hashWorkers[index] = new Worker(
            new URL("./hash.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.hashWorkers[index];

      case "border":
        if (!this.generateBorderworker) {
          this.generateBorderworker = new Worker(
            new URL("./border.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.generateBorderworker;

      case "export":
        if (!this.exportWorker) {
          this.exportWorker = new Worker(
            new URL("./export.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.exportWorker;

      case "exif":
        if (!this.extractExifWorker) {
          this.extractExifWorker = new Worker(
            new URL("./exif.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.extractExifWorker;

      default:
        throw new Error(`Unknown worker type: ${type}`);
    }
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
    const worker = this.getOrInitializeWorker("thumbnail");

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "BATCH_COMPLETE":
            resolve({
              batchIndex: e.data.payload.batchIndex,
              results: e.data.payload.results,
              status: "complete",
            });
            worker.removeEventListener("message", handler);
            break;
          case "ERROR": {
            const error = new Error(e.data.payload.error);
            error.name = e.data.payload.errorName;
            error.stack = e.data.payload.errorStack;
            worker.removeEventListener("message", handler);
            reject(error);
            break;
          }
          case "PROGRESS":
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };
      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "GENERATE_THUMBNAIL",
        data,
      });
    });
  }

  abortGenerateThumbnail() {
    if (this.generateThumbnailworker) {
      this.generateThumbnailworker.postMessage({ type: "ABORT" });
    }
  }

  // --- Hash Generation (Worker Pool) ---
  async generateHash(
    data: FileList | File[],
    onItemComplete?: (result: SingleHashResult) => void
  ): Promise<{ status: string }> {
    const filesArray = Array.isArray(data) ? data : Array.from(data);
    if (filesArray.length === 0) return { status: "complete" };

    const total = filesArray.length;
    let processed = 0;

    return new Promise((resolve, reject) => {
      let currentIndex = 0;
      let activeWorkers = 0;
      let hasError = false;

      const runTask = (workerIndex: number) => {
        if (currentIndex >= total || hasError) {
          if (activeWorkers === 0 && !hasError) resolve({ status: "complete" });
          return;
        }

        const fileIndex = currentIndex++;
        const file = filesArray[fileIndex];
        const worker = this.getOrInitializeWorker("hash", workerIndex);
        activeWorkers++;

        const handler = (e: MessageEvent) => {
          if (e.data.type === "HASH_SINGLE_COMPLETE") {
            if (onItemComplete) {
              onItemComplete({
                index: fileIndex,
                hash: e.data.payload.hash
              });
            }
            processed++;
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { 
                detail: { processed, total } 
              }),
            );
          } else if (e.data.type === "HASH_COMPLETE") {
            worker.removeEventListener("message", handler);
            activeWorkers--;
            runTask(workerIndex); // Get next task
          } else if (e.data.type === "ERROR") {
            worker.removeEventListener("message", handler);
            hasError = true;
            reject(new Error(e.data.payload?.error || "Hash Error"));
          }
        };

        worker.addEventListener("message", handler);
        worker.postMessage({
          type: "GENERATE_HASH",
          data: [file],
        });
      };

      // Start initial workers
      const numWorkers = Math.min(THREADS, total);
      for (let i = 0; i < numWorkers; i++) {
        runTask(i);
      }
    });
  }

  abortGenerateHash() {
    this.hashWorkers.forEach(w => w.postMessage({ type: "ABORT" }));
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
    const worker = this.getOrInitializeWorker("border");

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "GENERATE_BORDER_COMPLETE":
            worker.removeEventListener("message", handler);
            resolve(e.data.data);
            break;
          case "ERROR":
            worker.removeEventListener("message", handler);
            reject(new Error(e.data.payload.error));
            break;
        }
      };
      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "GENERATE_BORDER",
        data: { files },
        option,
        param,
      });
    });
  }

  abortGenerateBorders() {
    if (this.generateBorderworker) {
      this.generateBorderworker.postMessage({ type: "ABORT" });
    }
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
    const worker = this.getOrInitializeWorker("export");

    return new Promise((resolve) => {
      const handleMessage = (event: MessageEvent) => {
        const { type, result, error, payload } = event.data;
        if (type === "EXPORT_COMPLETE") {
          worker.removeEventListener("message", handleMessage);
          resolve({
            status: "complete",
            blob: result.blob,
            filename: result.filename,
          });
        } else if (type === "ERROR") {
          worker.removeEventListener("message", handleMessage);
          resolve({ status: "error", error: error || "Export failed" });
        } else if (type === "PROGRESS") {
          this.eventTarget.dispatchEvent(
            new CustomEvent("progress", {
              detail: { processed: payload?.processed || 0 },
            }),
          );
        }
      };
      worker.addEventListener("message", handleMessage);
      worker.postMessage({
        type: "EXPORT_IMAGE",
        data: { imageUrl, options },
      });
    });
  }

  abortExportImage() {
    if (this.exportWorker) {
      this.exportWorker.postMessage({ type: "ABORT" });
    }
  }

  // --- EXIF Extraction ---
  async extractExif(files: FileList | File[]): Promise<{
    exifResults: Array<{ index: number; exifData: Record<string, any> }>;
    status: string;
  }> {
    const worker = this.getOrInitializeWorker("exif");

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "EXIF_COMPLETE":
            worker.removeEventListener("message", handler);
            resolve({
              exifResults: e.data.payload.results,
              status: "complete",
            });
            break;
          case "ERROR":
            worker.removeEventListener("message", handler);
            reject(new Error(e.data.payload.error));
            break;
          case "PROGRESS":
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };
      worker.addEventListener("message", handler);
      const filesArray = Array.isArray(files) ? files : Array.from(files);
      worker.postMessage({
        type: "EXTRACT_EXIF",
        data: { files: filesArray },
      });
    });
  }

  abortExtractExif() {
    if (this.extractExifWorker) {
      this.extractExifWorker.postMessage({ type: "ABORT" });
    }
  }

  // --- Lifecycle Management ---
  /**
   * Terminates all active workers to clean up resources.
   * This should be called when the application is unmounting.
   */
  terminateAllWorkers(): void {
    if (this.generateThumbnailworker) {
      this.generateThumbnailworker.terminate();
      this.generateThumbnailworker = null;
    }
    this.hashWorkers.forEach(w => w.terminate());
    this.hashWorkers = [];

    if (this.generateBorderworker) {
      this.generateBorderworker.terminate();
      this.generateBorderworker = null;
    }
    if (this.exportWorker) {
      this.exportWorker.terminate();
      this.exportWorker = null;
    }
    if (this.extractExifWorker) {
      this.extractExifWorker.terminate();
      this.extractExifWorker = null;
    }
    console.log("All workers terminated.");
  }
}
