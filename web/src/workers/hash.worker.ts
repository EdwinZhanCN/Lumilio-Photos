/// <reference lib="webworker" />

import init, {StreamingHasher} from "../wasm/blake3/blake3_wasm";

// --- Constants (Matching Backend) ---
const QUICK_HASH_THRESHOLD = 100 * 1024 * 1024; // 100 MB
const QUICK_HASH_CHUNK_SIZE = 1 * 1024 * 1024; // 1 MB
const CHUNK_SIZE = 4 * 1024 * 1024; // 4MB chunks for efficient streaming

// --- Type Definitions ---
export interface WorkerMessage {
  type: "ABORT" | "GENERATE_HASH";
  data?: File[];
}

export interface SingleHashPayload {
  index: number;
  hash: string;
  error?: string;
}

// --- Initialization Control ---
let initializationPromise: Promise<void> | null = null;

async function initialize(): Promise<void> {
  if (initializationPromise) return initializationPromise;

  initializationPromise = (async () => {
    try {
      await init();
      self.postMessage({ type: "WASM_READY" });
    } catch (error: unknown) {
      const errMsg = (error as Error).message ?? "Unknown worker error";
      self.postMessage({ type: "ERROR", payload: { error: errMsg } });
      throw new Error(errMsg);
    }
  })();

  return initializationPromise;
}

async function calculateQuickHash(file: File, signal: AbortSignal): Promise<string> {
  const hasher = new StreamingHasher();
  const sizeBuf = new ArrayBuffer(8);
  const sizeView = new BigUint64Array(sizeBuf);
  sizeView[0] = BigInt(file.size);
  hasher.update(new Uint8Array(sizeBuf));

  const firstChunk = await file.slice(0, QUICK_HASH_CHUNK_SIZE).arrayBuffer();
  if (signal.aborted) throw new Error("Aborted");
  hasher.update(new Uint8Array(firstChunk));

  if (file.size > QUICK_HASH_CHUNK_SIZE) {
    let lastChunkStart = file.size - QUICK_HASH_CHUNK_SIZE;
    if (lastChunkStart < QUICK_HASH_CHUNK_SIZE) lastChunkStart = QUICK_HASH_CHUNK_SIZE;
    const lastChunk = await file.slice(lastChunkStart, file.size).arrayBuffer();
    if (signal.aborted) throw new Error("Aborted");
    hasher.update(new Uint8Array(lastChunk));
  }
  return hasher.finalize();
}

async function calculateFullHash(file: File, signal: AbortSignal): Promise<string> {
  const hasher = new StreamingHasher();
  let offset = 0;
  while (offset < file.size) {
    if (signal.aborted) throw new Error("Aborted");
    const end = Math.min(offset + CHUNK_SIZE, file.size);
    const chunk = await file.slice(offset, end).arrayBuffer();
    hasher.update(new Uint8Array(chunk));
    offset = end;
  }
  return hasher.finalize();
}

let abortController = new AbortController();

self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
  const { type, data } = e.data;

  switch (type) {
    case "ABORT":
      abortController.abort();
      abortController = new AbortController();
      break;

    case "GENERATE_HASH": {
      const signal = abortController.signal;

      try {
        await initialize();
        if (!data || data.length === 0) {
          self.postMessage({ type: "HASH_COMPLETE" });
          return;
        }

        for (let i = 0; i < data.length; i++) {
          if (signal.aborted) break;
          const asset = data[i];
          try {
            const hash = asset.size > QUICK_HASH_THRESHOLD 
              ? await calculateQuickHash(asset, signal) 
              : await calculateFullHash(asset, signal);

            self.postMessage({
              type: "HASH_SINGLE_COMPLETE",
              payload: { index: i, hash }
            });
          } catch (err: any) {
            if (err.message === "Aborted") break;
            self.postMessage({
              type: "HASH_SINGLE_COMPLETE",
              payload: { index: i, hash: "", error: err.message }
            });
          }
        }
        if (!signal.aborted) self.postMessage({ type: "HASH_COMPLETE" });
      } catch (err: any) {
        self.postMessage({ type: "ERROR", payload: { error: err.message } });
      }
      break;
    }
  }
};
