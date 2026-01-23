import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { AppWorkerClient, SingleHashResult } from './workerClient';

describe('Hash Worker Performance (Final Strategy)', () => {
  let client: AppWorkerClient;

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
      files.push(new File([content], `test-${i}.bin`, { type: 'application/octet-stream' }));
    }
    return files;
  };

  const timeout = 300000; 

  it('Final Strategy: Pure Worker Pool - 20 files x 50MB', async () => {
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
    results.forEach(result => {
      expect(result.error, `Hash failed for file ${result.index}: ${result.error}`).toBeUndefined();
      expect(result.hash, `Hash is empty for file ${result.index}`).toBeTruthy();
    });

    console.log(`[FINAL] Total: ${totalMB}MB, Duration: ${durationInSeconds.toFixed(2)}s, Speed: ${speedMBps.toFixed(2)} MB/s`);
  }, timeout);
});
