import { describe, it, expect, beforeAll, beforeEach, afterEach } from "vite-plus/test";
import { AppWorkerClient } from "./workerClient";
import type { SingleHashResult } from "./workerClient";
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

describe("Hash worker contract", () => {
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

  const timeout = 300000;

  it(
    "hashes a small file through the worker",
    async () => {
      const results: SingleHashResult[] = [];

      await client.generateHash([new File(["lumilio"], "small.bin")], (result) => {
        results.push(result);
      });

      expect(results).toHaveLength(1);
      expect(results[0].error).toBeUndefined();
      expect(results[0].hash).toMatch(/^[a-f0-9]{64}$/);
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
