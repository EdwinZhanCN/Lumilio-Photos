/**
 * A unified client to manage and interact with all web workers in the application.
 * This class provides a clean, promise-based API for computationally expensive tasks,
 * abstracting away the underlying `postMessage` communication.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */

import {
  ChatCompletionMessageParam,
  CreateWebWorkerMLCEngine,
  ModelRecord,
  AppConfig,
  MLCEngineConfig,
  prebuiltAppConfig,
  modelLibURLPrefix,
  modelVersion,
  InitProgressReport,
} from "@mlc-ai/web-llm";

export type WorkerType =
  | "thumbnail"
  | "hash"
  | "border"
  | "export"
  | "exif"
  | "llm";

export interface WorkerClientOptions {
  preload?: WorkerType[];
  webllmConfig?: {
    modelRecords?: ModelRecord[];
    useIndexedDBCache?: boolean;
    modelId: string;
  };
}

// TODO: Make this Configurable
const qwen3Custom: ModelRecord[] = [
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-4B-q4f16_1-MLC",
    model_id: "Qwen3-4B-q4f16_1-MLC",
    model_lib:
      modelLibURLPrefix +
      modelVersion +
      "/Qwen3-4B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 3431.59,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-1.7B-q4f16_1-MLC",
    model_id: "Qwen3-1.7B-q4f16_1-MLC",
    model_lib:
      modelLibURLPrefix +
      modelVersion +
      "/Qwen3-1.7B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 2036.66,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
];

export class AppWorkerClient {
  private generateThumbnailworker: Worker | null = null;
  private hashAssetsworker: Worker | null = null;
  private generateBorderworker: Worker | null = null;
  private exportWorker: Worker | null = null;
  private extractExifWorker: Worker | null = null;
  private WebLLMWorker: Worker | null = null;
  private webLLMEngine: any = null;
  private webllmAppConfig: AppConfig = {
    useIndexedDBCache: true,
    model_list: qwen3Custom,
  };
  private currentModelId: string | null = null;
  private targetModelId: string = "Qwen3-1.7B-q4f16_1-MLC";

  private eventTarget: EventTarget;

  constructor(options: WorkerClientOptions = {}) {
    this.eventTarget = new EventTarget();

    // Set up WebLLM configuration if provided
    if (options.webllmConfig) {
      this.webllmAppConfig = {
        model_list: options.webllmConfig.modelRecords || qwen3Custom,
        useIndexedDBCache: options.webllmConfig.useIndexedDBCache ?? true,
      };
      this.targetModelId = options.webllmConfig.modelId;
    }

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

      case "llm":
        if (!this.WebLLMWorker) {
          this.WebLLMWorker = new Worker(
            new URL("./webllm.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.WebLLMWorker;

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
          case "ERROR":
            const error = new Error(e.data.payload.error);
            error.name = e.data.payload.errorName;
            error.stack = e.data.payload.errorStack;
            worker.removeEventListener("message", handler);
            reject(error);
            break;
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
  async generateHash(data: FileList | File[]): Promise<{
    hashResults: Array<{ index: number; hash: string }>;
    status: string;
  }> {
    const worker = this.getOrInitializeWorker("hash");

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        switch (e.data.type) {
          case "HASH_COMPLETE":
            resolve({
              hashResults: e.data.hashResult,
              status: "complete",
            });
            worker.removeEventListener("message", handler);
            break;
          case "ERROR":
            worker.removeEventListener("message", handler);
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
      worker.addEventListener("message", handler);
      const filesArray = Array.isArray(data) ? data : Array.from(data);
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

  /**
   * Initialize the WebLLM engine that communicates with the worker
   */
  public async initializeWebLLMEngine(
    modelId: string = this.targetModelId,
  ): Promise<void> {
    if (!this.WebLLMWorker) return;

    // Reset engine if model changes
    if (this.webLLMEngine && this.currentModelId !== modelId) {
      this.webLLMEngine = null;
    }

    if (this.webLLMEngine) return;

    const engineConfig: MLCEngineConfig = {
      initProgressCallback: (report: InitProgressReport) => {
        // report is exactly { progress, timeElapsed, text }
        this.eventTarget.dispatchEvent(
          new CustomEvent("progress", { detail: report }),
        );
      },
      appConfig: this.webllmAppConfig ?? prebuiltAppConfig,
    };

    // Add custom app config if provided
    if (this.webllmAppConfig) {
      engineConfig.appConfig = this.webllmAppConfig;
    }

    this.webLLMEngine = await CreateWebWorkerMLCEngine(
      this.WebLLMWorker,
      modelId,
      engineConfig,
    );

    this.currentModelId = modelId;
  }

  /**
   * Ask the LLM a question with streaming response
   */
  async askLLM(
    messages: ChatCompletionMessageParam[],
    options: {
      temperature?: number;
      stream?: boolean;
      onChunk?: (chunk: string) => void;
    } = {},
  ): Promise<string> {
    // Ensure engine initialized or reinitialized if model changed
    if (!this.webLLMEngine || this.currentModelId !== this.targetModelId) {
      await this.initializeWebLLMEngine(this.targetModelId);
    }

    const { temperature = 1, stream = true, onChunk } = options;

    if (stream) {
      // Streaming response
      const chunks = await this.webLLMEngine.chat.completions.create({
        messages,
        temperature,
        stream: true,
        stream_options: { include_usage: true },
      });

      let reply = "";
      for await (const chunk of chunks) {
        const content = chunk.choices[0]?.delta.content || "";
        reply += content;

        // Call the chunk callback if provided
        if (onChunk && content) {
          onChunk(content);
        }

        if (chunk.usage) {
          console.log("Usage:", chunk.usage);
        }
      }

      return reply;
    } else {
      // Non-streaming response
      const response = await this.webLLMEngine.chat.completions.create({
        messages,
        temperature,
        stream: false,
      });

      return response.choices[0]?.message?.content || "";
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
    if (this.WebLLMWorker) {
      this.WebLLMWorker.terminate();
      this.WebLLMWorker = null;
      this.webLLMEngine = null;
      this.currentModelId = null;
    }
    console.log("All workers terminated.");
  }
}
