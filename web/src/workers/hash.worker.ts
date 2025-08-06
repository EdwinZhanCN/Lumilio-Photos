/// <reference lib="webworker" />

import init, { hash_asset, HashResult } from "@/wasm/blake3_wasm";

// --- Type Definitions ---
export interface WorkerMessage {
  type: "ABORT" | "GENERATE_HASH";
  data?: File[];
}

export interface WorkerHashResult {
  index: number;
  hash: string;
  error?: string;
}

// --- Initialization Control ---
const initializationPromise: Promise<void> | null = null;

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
        console.error("Error initializing genHash WebAssembly module:", error);
        self.postMessage({ type: "ERROR", payload: { error: errMsg } });
        reject(new Error(errMsg));
      });
  });
}

// --- Abort Control ---
let abortController = new AbortController();

// --- Main Logic ---
self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
  const { type, data } = e.data;

  switch (type) {
    case "ABORT":
      abortController.abort();
      abortController = new AbortController(); // Reset for the next task
      break;

    case "GENERATE_HASH": {
      abortController = new AbortController(); // Create a new controller for this job
      const signal = abortController.signal;
      let numberOfFilesProcessed = 0;

      try {
        await initialize();

        if (!data) {
          throw new Error("No files provided for hashing");
        }

        const assets = data;
        const CONCURRENCY =
          assets.length > 0 && assets[0].size > 100_000_000 ? 10 : 100;
        const allResults: WorkerHashResult[] = [];

        for (let i = 0; i < assets.length; i += CONCURRENCY) {
          if (signal.aborted) break;

          const batch = assets.slice(i, i + CONCURRENCY);
          const promises = batch.map(async (asset, batchIndex) => {
            const globalIndex = i + batchIndex;
            if (signal.aborted) {
              return {
                index: globalIndex,
                hash: "0".repeat(64),
                error: "Operation aborted",
              };
            }

            try {
              const arrayBuffer = await asset.arrayBuffer();
              if (signal.aborted) {
                return {
                  index: globalIndex,
                  hash: "0".repeat(64),
                  error: "Operation aborted",
                };
              }
              const rawHash: HashResult = hash_asset(
                new Uint8Array(arrayBuffer),
              );
              return { index: globalIndex, hash: rawHash.hash };
            } catch (err: unknown) {
              const errorMessage = `Error generating hash for ${asset.name}`;
              console.error(errorMessage, err);
              return {
                index: globalIndex,
                hash: "0".repeat(64),
                error: (err as Error).message,
              };
            } finally {
              self.postMessage({
                type: "PROGRESS",
                payload: {
                  processed: ++numberOfFilesProcessed,
                  total: assets.length,
                },
              });
            }
          });

          const batchResults = await Promise.all(promises);
          allResults.push(...batchResults);
        }

        self.postMessage({
          type: "HASH_COMPLETE",
          hashResult: allResults.sort((a, b) => a.index - b.index),
        });
      } catch (err: unknown) {
        const errorMessage =
          err instanceof Error ? err.message : "Unknown worker error";
        console.error("Error in GENERATE_HASH task:", err);
        self.postMessage({ type: "ERROR", payload: { error: errorMessage } });
      }
      break;
    }

    default:
      self.postMessage({
        type: "ERROR",
        payload: { error: `Unknown message type: ${type}` },
      });
      break;
  }
};
