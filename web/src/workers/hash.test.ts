import { describe, it, expect, beforeEach } from 'vitest';
import { AppWorkerClient } from './workerClient';

describe('Hash Worker Performance (Real Browser)', () => {
  let client: AppWorkerClient;

  beforeEach(() => {
    client = new AppWorkerClient();
  });

  const createMockFile = (sizeInMB: number) => {
    const size = sizeInMB * 1024 * 1024;
    const content = new Uint8Array(size);
    // Fill with some data to avoid all-zero optimization if any
    for (let i = 0; i < 100; i++) content[i] = i; 
    return new File([content], `test-${sizeInMB}MB.bin`, { type: 'application/octet-stream' });
  };

  const testHashPerformance = async (sizeInMB: number) => {
    const file = createMockFile(sizeInMB);
    
    // Wait for WASM to be ready if needed (the worker sends WASM_READY)
    // In this client, we just call generateHash which handles initialization.
    
    const startTime = performance.now();
    
    let hashResult = '';
    await client.generateHash([file], (result) => {
      hashResult = result.hash;
    });

    const endTime = performance.now();
    const durationInSeconds = (endTime - startTime) / 1000;
    const speedMBps = sizeInMB / durationInSeconds;

    console.log(`[PERF] File Size: ${sizeInMB}MB, Hash: ${hashResult}, Duration: ${durationInSeconds.toFixed(4)}s, Speed: ${speedMBps.toFixed(2)} MB/s`);
    
    expect(hashResult).toBeTruthy();
    expect(hashResult.length).toBeGreaterThan(10); // Should be a hex string
    
    return speedMBps;
  };

  // Increase timeout for large files
  const timeout = 60000; 

  it('should hash a 3MB file and record speed', async () => {
    await testHashPerformance(3);
  }, timeout);

  it('should hash a 50MB file and record speed', async () => {
    await testHashPerformance(50);
  }, timeout);

  it('should hash a 99MB file and record speed', async () => {
    await testHashPerformance(99);
  }, timeout);
});
