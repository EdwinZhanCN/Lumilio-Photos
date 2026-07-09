import { describe, it, expect, beforeAll, beforeEach, afterEach } from "vite-plus/test";
import { AppWorkerClient, SingleHashResult } from "./workerClient";
import init, { StreamingHasher } from "../wasm/blake3/blake3_wasm";

const QUICK_HASH_THRESHOLD = 100 * 1024 * 1024;
const QUICK_HASH_CHUNK_SIZE = 1 * 1024 * 1024;

const backendCompatibleQuickHash = (
  fileSize: number,
  firstChunk: Uint8Array,
  lastChunk: Uint8Array,
) => {
  const hasher = new StreamingHasher();
  const sizeBuf = new ArrayBuffer(8);
  new DataView(sizeBuf).setBigUint64(0, BigInt(fileSize), true);
  hasher.update(new Uint8Array(sizeBuf));
  hasher.update(firstChunk);
  hasher.update(lastChunk);
  return hasher.finalize();
};

describe("Hash Worker Performance (Final Strategy)", () => {
  let client: AppWorkerClient;

  beforeAll(async () => {
    await init(new URL("../wasm/blake3/blake3_wasm_bg.wasm", import.meta.url));
  });

  beforeEach(() => {
    client = new AppWorkerClient();
  });

  afterEach(() => {
    client.terminateAllWorkers();
  });

  const createMockFiles = (count: number, sizeInMB: number) => {
    const files: File[] = [];
    const size = sizeInMB * 1024 * 1024;
    const content = new Uint8Array(size);
    content.fill(0x42);

    for (let i = 0; i < count; i++) {
      files.push(new File([content], `test-${i}.bin`, { type: "application/octet-stream" }));
    }
    return files;
  };

  const timeout = 300000;

  it(
    "Final Strategy: Pure Worker Pool - 20 files x 50MB",
    async () => {
      const count = 20;
      const sizeInMB = 50;
      const files = createMockFiles(count, sizeInMB);
      const totalMB = count * sizeInMB;
      const startTime = performance.now();

      const results: SingleHashResult[] = [];

      await client.generateHash(files, (result) => {
        results.push(result);
      });

      const endTime = performance.now();
      const durationInSeconds = (endTime - startTime) / 1000;
      const speedMBps = totalMB / durationInSeconds;

      // Assertions after completion to avoid interrupting the worker loop
      expect(results.length).toBe(count);
      results.forEach((result) => {
        expect(
          result.error,
          `Hash failed for file ${result.index}: ${result.error}`,
        ).toBeUndefined();
        expect(result.hash, `Hash is empty for file ${result.index}`).toBeTruthy();
      });

      console.log(
        `[FINAL] Total: ${totalMB}MB, Duration: ${durationInSeconds.toFixed(2)}s, Speed: ${speedMBps.toFixed(2)} MB/s`,
      );
    },
    timeout,
  );

  it(
    "uses the backend-compatible quick hash fingerprint for files over 100MB",
    async () => {
      const firstChunk = new Uint8Array(QUICK_HASH_CHUNK_SIZE);
      firstChunk.fill(0x11);

      const middleChunk = new Uint8Array(
        QUICK_HASH_THRESHOLD + QUICK_HASH_CHUNK_SIZE - QUICK_HASH_CHUNK_SIZE * 2,
      );
      middleChunk.fill(0x22);

      const lastChunk = new Uint8Array(QUICK_HASH_CHUNK_SIZE);
      lastChunk.fill(0x33);

      const file = new File([firstChunk, middleChunk, lastChunk], "quick-hash.bin", {
        type: "application/octet-stream",
      });
      const expected = backendCompatibleQuickHash(file.size, firstChunk, lastChunk);
      const results: SingleHashResult[] = [];

      await client.generateHash([file], (result) => {
        results.push(result);
      });

      expect(results).toHaveLength(1);
      expect(results[0].error).toBeUndefined();
      expect(results[0].hash).toBe(expected);
    },
    timeout,
  );
});
