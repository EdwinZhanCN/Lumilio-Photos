import { afterEach, beforeEach, describe, expect, it } from "vite-plus/test";
import { AppWorkerClient } from "./workerClient";
import type { SingleHashResult } from "./workerClient";

describe("Hash worker performance", () => {
  let client: AppWorkerClient;

  beforeEach(() => {
    client = new AppWorkerClient();
  });

  afterEach(() => {
    client.terminateAllWorkers();
  });

  it("hashes 20 files x 50 MB through the worker pool", async () => {
    const count = 20;
    const sizeInMB = 50;
    const content = new Uint8Array(sizeInMB * 1024 * 1024);
    content.fill(0x42);
    const files = Array.from(
      { length: count },
      (_, index) => new File([content], `test-${index}.bin`, { type: "application/octet-stream" }),
    );
    const totalMB = count * sizeInMB;
    const startTime = performance.now();
    const results: SingleHashResult[] = [];

    await client.generateHash(files, (result) => {
      results.push(result);
    });

    const durationInSeconds = (performance.now() - startTime) / 1000;
    const speedMBps = totalMB / durationInSeconds;

    expect(results).toHaveLength(count);
    results.forEach((result) => {
      expect(result.error, `Hash failed for file ${result.index}: ${result.error}`).toBeUndefined();
      expect(result.hash, `Hash is empty for file ${result.index}`).toBeTruthy();
    });

    console.log(
      `[hash-perf] Total: ${totalMB} MB, Duration: ${durationInSeconds.toFixed(2)} s, Speed: ${speedMBps.toFixed(2)} MB/s`,
    );
  }, 300_000);
});
