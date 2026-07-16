/**
 * A unified client to manage and interact with all web workers in the application.
 * This class provides a clean, promise-based API for computationally expensive tasks,
 * abstracting away the underlying `postMessage` communication.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */

import { detectDeviceCapabilities } from "@/lib/workers/batchSizing.ts";
import type { LayoutBox, LayoutConfig, LayoutResult } from "@/lib/layout/justifiedLayout";

export type WorkerType = "hash" | "justified" | "tool";

export interface WorkerClientOptions {
  preload?: WorkerType[];
}

export interface SingleHashResult {
  index: number;
  hash: string;
  file?: File;
  error?: string;
}

export class AppWorkerClient {
  private hashWorkers: Worker[] = [];
  private justifiedLayoutWorker: Worker | null = null;
  private toolWorker: Worker | null = null;
  private justifiedInitPromise: Promise<void> | null = null;
  private justifiedRequestId = 0;
  private toolRequestId = 0;
  private toolLoadedSet: Set<string> = new Set();

  private eventTarget: EventTarget;

  constructor(options: WorkerClientOptions = {}) {
    this.eventTarget = new EventTarget();

    if (options.preload) {
      options.preload.forEach((workerType) => {
        this.getOrInitializeWorker(workerType);
      });
    }
  }

  private getOrInitializeWorker(type: WorkerType, index: number = 0): Worker {
    switch (type) {
      case "hash":
        if (!this.hashWorkers[index]) {
          this.hashWorkers[index] = new Worker(new URL("./hash.worker.ts", import.meta.url), {
            type: "module",
          });
        }
        return this.hashWorkers[index];

      case "justified":
        if (!this.justifiedLayoutWorker) {
          this.justifiedLayoutWorker = new Worker(
            new URL("./justified.worker.ts", import.meta.url),
            { type: "module" },
          );
        }
        return this.justifiedLayoutWorker;

      case "tool":
        if (!this.toolWorker) {
          this.toolWorker = new Worker(new URL("./tool.worker.ts", import.meta.url), {
            type: "module",
          });
        }
        return this.toolWorker;

      default:
        throw new Error(`Unknown worker type: ${String(type)}`);
    }
  }

  addProgressListener(callback: (detail: any) => void): () => void {
    const handler = (e: CustomEvent) => callback(e.detail);
    this.eventTarget.addEventListener("progress", handler as EventListener);
    return () => this.eventTarget.removeEventListener("progress", handler as EventListener);
  }

  private nextJustifiedRequestId(): number {
    this.justifiedRequestId += 1;
    return this.justifiedRequestId;
  }

  private nextToolRequestId(): number {
    this.toolRequestId += 1;
    return this.toolRequestId;
  }

  async initializeJustifiedLayout(): Promise<void> {
    if (this.justifiedInitPromise) return this.justifiedInitPromise;
    const worker = this.getOrInitializeWorker("justified");

    this.justifiedInitPromise = new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        if (e.data?.type === "JUSTIFIED_READY") {
          worker.removeEventListener("message", handler);
          resolve();
        } else if (e.data?.type === "ERROR" && !e.data?.payload?.requestId) {
          worker.removeEventListener("message", handler);
          this.justifiedInitPromise = null;
          reject(new Error(e.data?.payload?.error || "Justified layout init failed"));
        }
      };

      worker.addEventListener("message", handler);
      worker.postMessage({ type: "INIT" });
    });

    return this.justifiedInitPromise;
  }

  async calculateJustifiedLayout(boxes: LayoutBox[], config: LayoutConfig): Promise<LayoutResult> {
    await this.initializeJustifiedLayout();
    const worker = this.getOrInitializeWorker("justified");
    const requestId = this.nextJustifiedRequestId();

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        const { type, payload } = e.data || {};
        if (!payload || payload.requestId !== requestId) return;

        if (type === "JUSTIFIED_LAYOUT_COMPLETE") {
          worker.removeEventListener("message", handler);
          resolve(payload.result as LayoutResult);
        } else if (type === "ERROR") {
          worker.removeEventListener("message", handler);
          reject(new Error(payload.error || "Justified layout failed"));
        }
      };

      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "CALCULATE_LAYOUT",
        payload: { requestId, boxes, config },
      });
    });
  }

  async calculateJustifiedLayouts(
    groups: Record<string, LayoutBox[]>,
    config: LayoutConfig,
  ): Promise<Record<string, LayoutResult>> {
    await this.initializeJustifiedLayout();
    const worker = this.getOrInitializeWorker("justified");
    const requestId = this.nextJustifiedRequestId();

    return new Promise((resolve, reject) => {
      const handler = (e: MessageEvent) => {
        const { type, payload } = e.data || {};
        if (!payload || payload.requestId !== requestId) return;

        if (type === "JUSTIFIED_LAYOUTS_COMPLETE") {
          worker.removeEventListener("message", handler);
          resolve(payload.results as Record<string, LayoutResult>);
        } else if (type === "ERROR") {
          worker.removeEventListener("message", handler);
          reject(new Error(payload.error || "Justified layout failed"));
        }
      };

      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "CALCULATE_MULTIPLE_LAYOUTS",
        payload: { requestId, groups, config },
      });
    });
  }

  // --- Hash Generation (Worker Pool) ---
  async generateHash(
    data: FileList | File[],
    onItemComplete?: (result: SingleHashResult) => void,
  ): Promise<{ status: string }> {
    const filesArray = Array.isArray(data) ? data : Array.from(data);
    if (filesArray.length === 0) return { status: "complete" };

    const total = filesArray.length;
    let processed = 0;
    const maxThreads = detectDeviceCapabilities().maxConcurrency;

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
            if (hasError) {
              worker.removeEventListener("message", handler);
              activeWorkers--;
              return;
            }
            if (e.data.payload?.error || !e.data.payload?.hash) {
              worker.removeEventListener("message", handler);
              activeWorkers--;
              hasError = true;
              this.abortGenerateHash();
              reject(new Error(e.data.payload?.error || "Hash worker returned an empty digest"));
              return;
            }
            if (onItemComplete) {
              try {
                onItemComplete({
                  index: fileIndex,
                  hash: e.data.payload.hash,
                  error: e.data.payload.error,
                });
              } catch (err) {
                console.error("Error in onItemComplete callback:", err);
              }
            }
            processed++;
            this.eventTarget.dispatchEvent(
              new CustomEvent("progress", {
                detail: { processed, total },
              }),
            );
          } else if (e.data.type === "HASH_COMPLETE") {
            worker.removeEventListener("message", handler);
            activeWorkers--;
            runTask(workerIndex);
          } else if (e.data.type === "ERROR") {
            worker.removeEventListener("message", handler);
            hasError = true;
            this.abortGenerateHash();
            reject(new Error(e.data.payload?.error || "Hash Error"));
          }
        };

        worker.addEventListener("message", handler);
        worker.postMessage({
          type: "GENERATE_HASH",
          data: [file],
        });
      };

      const numWorkers = Math.min(maxThreads, total);
      for (let i = 0; i < numWorkers; i++) {
        runTask(i);
      }
    });
  }

  abortGenerateHash() {
    this.hashWorkers.forEach((w) => w.postMessage({ type: "ABORT" }));
  }

  // --- Tool Runtime ---
  async loadTool(toolId: string): Promise<void> {
    if (this.toolLoadedSet.has(toolId)) {
      return;
    }

    const worker = this.getOrInitializeWorker("tool");

    await new Promise<void>((resolve, reject) => {
      const timeoutId = globalThis.setTimeout(() => {
        worker.removeEventListener("message", handler);
        reject(new Error(`Timed out loading tool: ${toolId}`));
      }, 15000);

      const handler = (event: MessageEvent) => {
        const { type, payload } = event.data || {};

        if (type === "TOOL_LOADED" && payload?.toolId === toolId) {
          globalThis.clearTimeout(timeoutId);
          worker.removeEventListener("message", handler);
          this.toolLoadedSet.add(toolId);
          resolve();
          return;
        }

        if (type === "ERROR" && payload?.stage === "load_tool" && payload?.toolId === toolId) {
          globalThis.clearTimeout(timeoutId);
          worker.removeEventListener("message", handler);
          reject(new Error(payload?.error || `Failed to load tool: ${toolId}`));
        }
      };

      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "LOAD_TOOL",
        payload: { toolId },
      });
    });
  }

  async runTool(
    toolId: string,
    file: File,
    params: Record<string, unknown>,
  ): Promise<{
    fileName: string;
    mimeType: string;
    blob: Blob;
  }> {
    await this.loadTool(toolId);
    const worker = this.getOrInitializeWorker("tool");
    const requestId = this.nextToolRequestId();

    return new Promise((resolve, reject) => {
      const handler = (event: MessageEvent) => {
        const { type, payload } = event.data || {};

        if (!payload || payload.requestId !== requestId) {
          return;
        }

        if (type === "TOOL_PROGRESS") {
          this.eventTarget.dispatchEvent(
            new CustomEvent("progress", {
              detail: {
                operation: "tool",
                processed: payload.processed,
                total: payload.total,
              },
            }),
          );
          return;
        }

        if (type === "TOOL_COMPLETE") {
          worker.removeEventListener("message", handler);
          const bytes =
            payload.bytes instanceof Uint8Array ? payload.bytes : new Uint8Array(payload.bytes);

          resolve({
            fileName: payload.fileName || "tool-output.bin",
            mimeType: payload.mimeType || "application/octet-stream",
            blob: new Blob([bytes], {
              type: payload.mimeType || "application/octet-stream",
            }),
          });
          return;
        }

        if (type === "ERROR" && payload.stage === "run_tool") {
          worker.removeEventListener("message", handler);
          reject(new Error(payload.error || `Tool execution failed: ${toolId}`));
        }
      };

      worker.addEventListener("message", handler);
      worker.postMessage({
        type: "RUN_TOOL",
        payload: {
          requestId,
          toolId,
          file,
          params,
        },
      });
    });
  }

  abortTool(): void {
    if (this.toolWorker) {
      this.toolWorker.postMessage({ type: "ABORT" });
    }
  }

  // --- Lifecycle Management ---
  terminateAllWorkers(): void {
    this.hashWorkers.forEach((w) => {
      if (w) w.terminate();
    });
    this.hashWorkers = [];
    if (this.justifiedLayoutWorker) {
      this.justifiedLayoutWorker.terminate();
      this.justifiedLayoutWorker = null;
    }
    if (this.toolWorker) {
      this.toolWorker.terminate();
      this.toolWorker = null;
    }
    this.justifiedInitPromise = null;
    this.toolLoadedSet.clear();
    console.log("All workers terminated.");
  }
}
