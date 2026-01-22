import { describe, it, expect, beforeEach } from 'vitest';
import { AppWorkerClient } from './workerClient';

describe('Hash Worker Performance (Final Strategy)', () => {
  let client: AppWorkerClient;

  beforeEach(() => {
    client = new AppWorkerClient();
  });

  const createMockFiles = (count: number, sizeInMB: number) => {
    const files: File[] = [];
    for (let i = 0; i < count; i++) {
      const size = sizeInMB * 1024 * 1024;
      const content = new Uint8Array(size);
      content[0] = i; 
      files.push(new File([content], `test-${i}-${sizeInMB}MB.bin`, { type: 'application/octet-stream' }));
    }
    return files;
  };

  const timeout = 300000; 

  it('Final Strategy: Pure Worker Pool - 50 files x 50MB', async () => {
    const files = createMockFiles(50, 50);
    const totalMB = 50 * 50;
    const startTime = performance.now();
    
    await client.generateHash(files, (result) => {
      expect(result.hash).toBeTruthy();
    });

    const endTime = performance.now();
    const durationInSeconds = (endTime - startTime) / 1000;
    const speedMBps = totalMB / durationInSeconds;

    console.log(`[FINAL] Total: ${totalMB}MB, Duration: ${durationInSeconds.toFixed(2)}s, Speed: ${speedMBps.toFixed(2)} MB/s`);
  }, timeout);
});
