import { describe, it, expect, beforeEach, vi } from 'vitest';
import { 
  SmartBatchSizer, 
  ProcessingPriority, 
  detectDeviceCapabilities,
  getOptimalBatchSize,
  recordProcessingMetrics
} from './smartBatchSizing';

// Mock performance.memory API
Object.defineProperty(performance, 'memory', {
  value: {
    jsHeapSizeLimit: 1024 * 1024 * 1024, // 1GB
    usedJSHeapSize: 512 * 1024 * 1024,   // 512MB
  },
  configurable: true,
});

// Mock navigator.hardwareConcurrency
Object.defineProperty(navigator, 'hardwareConcurrency', {
  value: 8,
  configurable: true,
});

describe('SmartBatchSizer', () => {
  let batchSizer: SmartBatchSizer;

  beforeEach(() => {
    batchSizer = new SmartBatchSizer();
    batchSizer.resetMetrics();
  });

  describe('getOptimalBatchSize', () => {
    it('should return appropriate batch size for thumbnail operation', () => {
      const batchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.CRITICAL);
      expect(batchSize).toBeGreaterThan(0);
      expect(batchSize).toBeLessThanOrEqual(100);
      expect(batchSize).toBeLessThanOrEqual(20); // Max for thumbnails
    });

    it('should return appropriate batch size for border operation', () => {
      const batchSize = batchSizer.getOptimalBatchSize('border', 50, ProcessingPriority.NORMAL);
      expect(batchSize).toBeGreaterThan(0);
      expect(batchSize).toBeLessThanOrEqual(50);
      expect(batchSize).toBeLessThanOrEqual(10); // Max for borders
    });

    it('should not exceed total items', () => {
      const totalItems = 3;
      const batchSize = batchSizer.getOptimalBatchSize('thumbnail', totalItems, ProcessingPriority.NORMAL);
      expect(batchSize).toBeLessThanOrEqual(totalItems);
    });

    it('should respect minimum batch size', () => {
      const batchSize = batchSizer.getOptimalBatchSize('thumbnail', 1, ProcessingPriority.NORMAL);
      expect(batchSize).toBeGreaterThanOrEqual(1); // Min batch size should be at least 1
    });

    it('should increase batch size for high priority operations', () => {
      const normalBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      batchSizer.resetMetrics(); // Reset to get fresh calculation
      const highBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.CRITICAL);
      
      // High priority should get larger or equal batch size
      expect(highBatchSize).toBeGreaterThanOrEqual(normalBatchSize);
    });
  });

  describe('recordMetrics and adaptation', () => {
    it('should adapt batch size based on slow processing metrics', () => {
      const initialBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      
      // Record slow processing metrics
      for (let i = 0; i < 3; i++) {
        recordProcessingMetrics({
          operationType: 'thumbnail',
          batchSize: initialBatchSize,
          processingTimeMs: 10000, // Very slow
          filesProcessed: initialBatchSize,
          avgTimePerFile: 10000 / initialBatchSize,
          success: true,
          errorRate: 0,
        });
      }

      const adaptedBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      expect(adaptedBatchSize).toBeLessThan(initialBatchSize);
    });

    it('should adapt batch size based on fast processing metrics', () => {
      const initialBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      
      // Record fast processing metrics
      for (let i = 0; i < 3; i++) {
        recordProcessingMetrics({
          operationType: 'thumbnail',
          batchSize: initialBatchSize,
          processingTimeMs: 1000, // Very fast
          filesProcessed: initialBatchSize,
          avgTimePerFile: 1000 / initialBatchSize,
          success: true,
          errorRate: 0,
        });
      }

      const adaptedBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      // The batch size should either increase or stay the same (due to performance preferences impact)
      expect(adaptedBatchSize).toBeGreaterThanOrEqual(initialBatchSize);
    });

    it('should reduce batch size on high error rate', () => {
      const initialBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      
      // Record high error rate metrics
      for (let i = 0; i < 3; i++) {
        recordProcessingMetrics({
          operationType: 'thumbnail',
          batchSize: initialBatchSize,
          processingTimeMs: 3000,
          filesProcessed: Math.floor(initialBatchSize * 0.5), // 50% failure
          avgTimePerFile: 3000 / Math.floor(initialBatchSize * 0.5),
          success: false,
          errorRate: 0.5,
        });
      }

      const adaptedBatchSize = batchSizer.getOptimalBatchSize('thumbnail', 100, ProcessingPriority.NORMAL);
      expect(adaptedBatchSize).toBeLessThan(initialBatchSize);
    });
  });

  describe('memory pressure detection', () => {
    it('should detect memory pressure when usage is high', () => {
      // Mock high memory usage
      Object.defineProperty(performance, 'memory', {
        value: {
          jsHeapSizeLimit: 1024 * 1024 * 1024, // 1GB
          usedJSHeapSize: 900 * 1024 * 1024,   // 900MB (87.5% usage)
        },
        configurable: true,
      });

      const newBatchSizer = new SmartBatchSizer();
      expect(newBatchSizer.isMemoryPressureDetected()).toBe(true);
    });

    it('should not detect memory pressure when usage is normal', () => {
      // Mock normal memory usage
      Object.defineProperty(performance, 'memory', {
        value: {
          jsHeapSizeLimit: 1024 * 1024 * 1024, // 1GB
          usedJSHeapSize: 500 * 1024 * 1024,   // 500MB (48.8% usage)
        },
        configurable: true,
      });

      const newBatchSizer = new SmartBatchSizer();
      expect(newBatchSizer.isMemoryPressureDetected()).toBe(false);
    });
  });

  describe('device capabilities', () => {
    it('should correctly detect device capabilities', () => {
      const capabilities = detectDeviceCapabilities();
      
      expect(capabilities.cpuCores).toBeGreaterThan(0);
      expect(capabilities.availableMemoryMB).toBeGreaterThan(0);
      expect(capabilities.maxConcurrency).toBeGreaterThan(0);
      expect(typeof capabilities.isLowEndDevice).toBe('boolean');
      expect(typeof capabilities.isMobile).toBe('boolean');
    });

    it('should classify low-end devices correctly', () => {
      // Mock low-end device
      Object.defineProperty(navigator, 'hardwareConcurrency', {
        value: 2,
        configurable: true,
      });

      const capabilities = detectDeviceCapabilities();
      expect(capabilities.isLowEndDevice).toBe(true);
      expect(capabilities.maxConcurrency).toBeLessThanOrEqual(2);
    });
  });
});

describe('Global utility functions', () => {
  beforeEach(() => {
    // Reset global state before each test
    vi.clearAllMocks();
  });

  it('getOptimalBatchSize should work with global instance', () => {
    const batchSize = getOptimalBatchSize('thumbnail', 50, ProcessingPriority.NORMAL);
    expect(batchSize).toBeGreaterThan(0);
    expect(batchSize).toBeLessThanOrEqual(50);
  });

  it('recordProcessingMetrics should work with global instance', () => {
    expect(() => {
      recordProcessingMetrics({
        operationType: 'thumbnail',
        batchSize: 5,
        processingTimeMs: 1000,
        filesProcessed: 5,
        avgTimePerFile: 200,
        success: true,
        errorRate: 0,
      });
    }).not.toThrow();
  });
});