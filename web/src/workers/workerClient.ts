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

export class AppWorkerClient {
  private generateThumbnailworker: Worker | null = null;
  private hashAssetsworker: Worker | null = null;
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
  private getOrInitializeWorker(type: WorkerType): Worker {
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
        if (!this.hashAssetsworker) {
          this.hashAssetsworker = new Worker(
            new URL("./hash.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.hashAssetsworker;

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

  // --- Hash Generation ---
  async generateHash(
    data: FileList | File[],
    // ✨ 新增：回调函数，每搞定一个就通知一次
    onItemComplete?: (result: SingleHashResult) => void
  ): Promise<{ status: string }> { // Promise 只负责告诉我们“全部结束了”，不负责传数据

    const worker = this.getOrInitializeWorker("hash");

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          // ✨ 新增：监听单个完成事件
          case "HASH_SINGLE_COMPLETE":
            if (onItemComplete) {
              // 这里立刻回调，React 那边接到这个回调就可以直接触发上传逻辑
              // 此时第 2-1000 张还在算，但第 1 张已经在往服务器传了
              onItemComplete({
                index: e.data.payload.index,
                hash: e.data.payload.hash
              });
            }
            break;

          case "HASH_COMPLETE":
            // 全部结束，清理监听器
            worker.removeEventListener("message", handler);
            resolve({ status: "complete" });
            break;

          case "ERROR":
            worker.removeEventListener("message", handler);
            reject(new Error(e.data.payload?.error || "Hash Error"));
            break;

          case "PROGRESS":
            // 这里的进度依然保留，用于 UI 进度条展示
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", { detail: e.data.payload }),
            );
            break;
        }
      };

      worker.addEventListener("message", handler);

      const filesArray = Array.isArray(data) ? data : Array.from(data);

      // 发送任务
      worker.postMessage({
        type: "GENERATE_HASH",
        data: filesArray,
      });
    });
  }

  abortGenerateHash() {
    if (this.hashAssetsworker) {
      this.hashAssetsworker.postMessage({ type: "ABORT" });
    }
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
    console.log(files);

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "GENERATE_BORDER_COMPLETE":
            worker.removeEventListener("message", handler);
            resolve(e.data.data);
            console.log(e.data.data);
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
    if (this.hashAssetsworker) {
      this.hashAssetsworker.terminate();
      this.hashAssetsworker = null;
    }
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
