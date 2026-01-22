/// <reference lib="webworker" />

// 修正 WASM 路径，使用相对路径以确保 Worker 能够正确解析
import init, {initThreadPool, StreamingHasher} from "../wasm/blake3/blake3_wasm";

// --- Constants (Matching Backend) ---
const QUICK_HASH_THRESHOLD = 100 * 1024 * 1024; // 100 MB
const QUICK_HASH_CHUNK_SIZE = 1 * 1024 * 1024; // 1 MB

const THREADS = Math.max(1, navigator.hardwareConcurrency - 1);
const CHUNK_SIZE = THREADS > 4 ? 8 * 1024 * 1024 : 4 * 1024 * 1024;


// --- Type Definitions ---
// 输入消息类型
export interface WorkerMessage {
  type: "ABORT" | "GENERATE_HASH";
  data?: File[];
}

// 单个结果返回类型 (前端需要根据这个来定义接收逻辑)
export interface SingleHashPayload {
  index: number;
  hash: string;
  error?: string;
  file?: File; // 可选：如果你需要把文件对象传回主线程（注意这不会拷贝文件内容，只是引用）
}

// 进度返回类型
export interface ProgressPayload {
  processed: number;
  total: number;
}

// --- Initialization Control ---
let initializationPromise: Promise<void> | null = null;

async function initialize(): Promise<void> {
  if (initializationPromise) {
    return initializationPromise;
  }

  initializationPromise = (async () => {
    try {
      await init();

      const isIsolated = self.crossOriginIsolated;

      if (isIsolated && navigator.hardwareConcurrency > 1 && initThreadPool) {
        
        await initThreadPool(THREADS);
      } else {
        console.warn("Worker is not cross-origin isolated or multi-threading not supported. Falling back to single-threaded mode.");
      }

      self.postMessage({ type: "WASM_READY" });
    } catch (error: unknown) {
      const errMsg = (error as Error).message ?? "Unknown worker error";
      console.error("Error initializing genHash WebAssembly module:", error);
      self.postMessage({ type: "ERROR", payload: { error: errMsg } });
      throw new Error(errMsg);
    }
  })();

  return initializationPromise;
}

/**
 * 策略 A: 快速哈希 (针对大文件)
 * Strategy: hash(file_size_64bit_le + first_chunk + last_chunk)
 */
async function calculateQuickHash(file: File, signal: AbortSignal): Promise<string> {
  const hasher = new StreamingHasher();

  // 1. Write file size as 8-byte little-endian
  const sizeBuf = new ArrayBuffer(8);
  const sizeView = new BigUint64Array(sizeBuf);
  sizeView[0] = BigInt(file.size);
  // 注意：hasher.update 需要 Uint8Array 视图
  hasher.update(new Uint8Array(sizeBuf));

  // 2. Read first chunk
  const firstChunk = await file.slice(0, QUICK_HASH_CHUNK_SIZE).arrayBuffer();
  if (signal.aborted) throw new Error("Aborted");
  hasher.update(new Uint8Array(firstChunk));

  // 3. Read last chunk (if file is large enough)
  if (file.size > QUICK_HASH_CHUNK_SIZE) {
    let lastChunkStart = file.size - QUICK_HASH_CHUNK_SIZE;
    if (lastChunkStart < QUICK_HASH_CHUNK_SIZE) {
      lastChunkStart = QUICK_HASH_CHUNK_SIZE;
    }
    const lastChunk = await file.slice(lastChunkStart, file.size).arrayBuffer();
    if (signal.aborted) throw new Error("Aborted");
    hasher.update(new Uint8Array(lastChunk));
  }

  // finalize 消耗 hasher 并返回 hex string
  return hasher.finalize();
}

/**
 * 策略 B: 全量哈希 (针对小文件)
 * 使用 CHUNK_SIZE 分块读取，提高 BLAKE3 效率
 */
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

// --- Abort Control ---
let abortController = new AbortController();

// --- Main Logic ---
self.onmessage = async (e: MessageEvent<WorkerMessage>) => {
  const { type, data } = e.data;

  switch (type) {
    case "ABORT":
      abortController.abort();
      // 重置 Controller 以便下次使用
      abortController = new AbortController();
      break;

    case "GENERATE_HASH": {
      // 每次新任务开始前，确保之前的被 Abort (或者重置信号)
      abortController.abort();
      abortController = new AbortController();
      const signal = abortController.signal;

      let numberOfFilesProcessed = 0;

      try {
        await initialize();

        if (!data || data.length === 0) {
          // 处理空数组情况，直接返回完成
          self.postMessage({ type: "HASH_COMPLETE" });
          return;
        }

        const assets = data;
        const total = assets.length;

        // --- 核心循环：流式处理 ---
        for (let i = 0; i < total; i++) {
          // 检查中止信号
          if (signal.aborted) break;

          const asset = assets[i];
          const globalIndex = i; // 这里假设传入的数组 index 就是全局 index，或者你可以从 data 里传 id 进来

          try {
            let hash: string;

            // 根据大小选择策略
            if (asset.size > QUICK_HASH_THRESHOLD) {
              hash = await calculateQuickHash(asset, signal);
            } else {
              hash = await calculateFullHash(asset, signal);
            }

            // 🔥 关键修改：算完一个，立马吐出来
            // 这样主线程可以立刻把这个文件扔进上传队列
            const payload: SingleHashPayload = {
              index: globalIndex,
              hash: hash,
              // file: asset // 如果需要回传文件对象用于上传，可以在这里带上
            };

            self.postMessage({
              type: "HASH_SINGLE_COMPLETE",
              payload: payload
            });

          } catch (err: unknown) {
            // 如果是 Aborted，通常不需要报错，直接退出循环即可
            if ((err as Error).message === "Aborted") {
              break;
            }

            const errorMessage = `Error generating hash for ${asset.name}`;
            console.error(errorMessage, err);

            // 单个失败不应打断整体流程，返回错误信息即可
            self.postMessage({
              type: "HASH_SINGLE_COMPLETE",
              payload: {
                index: globalIndex,
                hash: "", // 空 hash 代表失败
                error: (err as Error).message
              }
            });
          } finally {
            // 只有在没被中断的情况下才更新进度
            if (!signal.aborted) {
              self.postMessage({
                type: "PROGRESS",
                payload: {
                  processed: ++numberOfFilesProcessed,
                  total: total,
                },
              });
            }
          }
        }

        // 循环结束，发送总完成信号 (不带数据)
        if (!signal.aborted) {
          self.postMessage({ type: "HASH_COMPLETE" });
        }

      } catch (err: unknown) {
        const errorMessage = err instanceof Error ? err.message : "Unknown worker error";
        console.error("Error in GENERATE_HASH task:", err);
        self.postMessage({ type: "ERROR", payload: { error: errorMessage } });
      }
      break;
    }

    default:
      // @ts-ignore
      console.warn(`Unknown message type: ${type}`);
      break;
  }
};
