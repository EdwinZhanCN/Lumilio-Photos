/**
 * Smart Batch Sizing System
 *
 * Provides dynamic batch sizing based on device capabilities and processing feedback
 * to optimize memory usage vs processing speed.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */

import {
  globalPerformancePreferences,
  PerformanceProfile,
} from "./performancePreferences";

// Device capability metrics
export interface DeviceCapabilities {
  cpuCores: number;
  availableMemoryMB: number;
  isLowEndDevice: boolean;
  isMobile: boolean;
  maxConcurrency: number;
}

// Processing performance metrics
export interface ProcessingMetrics {
  operationType: string;
  batchSize: number;
  processingTimeMs: number;
  memoryUsedMB?: number;
  filesProcessed: number;
  avgTimePerFile: number;
  success: boolean;
  errorRate: number;
}

// Batch sizing configuration
export interface BatchSizingConfig {
  minBatchSize: number;
  maxBatchSize: number;
  targetProcessingTimeMs: number;
  memoryThresholdMB: number;
  priorityMultiplier: number;
  adaptationRate: number;
}

// Operation priority levels
export enum ProcessingPriority {
  LOW = 1,
  NORMAL = 2,
  HIGH = 3,
  CRITICAL = 4, // User-visible operations
}

// Default configurations for different operation types
const DEFAULT_CONFIGS: Record<string, BatchSizingConfig> = {
  thumbnail: {
    minBatchSize: 2,
    maxBatchSize: 20,
    targetProcessingTimeMs: 3000,
    memoryThresholdMB: 100,
    priorityMultiplier: 1.5,
    adaptationRate: 0.2,
  },
  border: {
    minBatchSize: 1,
    maxBatchSize: 10,
    targetProcessingTimeMs: 5000,
    memoryThresholdMB: 200,
    priorityMultiplier: 1.2,
    adaptationRate: 0.15,
  },
  hash: {
    minBatchSize: 5,
    maxBatchSize: 50,
    targetProcessingTimeMs: 2000,
    memoryThresholdMB: 50,
    priorityMultiplier: 1.0,
    adaptationRate: 0.25,
  },
  exif: {
    minBatchSize: 3,
    maxBatchSize: 30,
    targetProcessingTimeMs: 4000,
    memoryThresholdMB: 75,
    priorityMultiplier: 1.1,
    adaptationRate: 0.2,
  },
  export: {
    minBatchSize: 1,
    maxBatchSize: 5,
    targetProcessingTimeMs: 10000,
    memoryThresholdMB: 300,
    priorityMultiplier: 2.0,
    adaptationRate: 0.1,
  },
};

/**
 * Detects device capabilities for batch sizing optimization
 */
export function detectDeviceCapabilities(): DeviceCapabilities {
  const cpuCores =
    (typeof navigator !== "undefined" && navigator.hardwareConcurrency) || 4;

  // Estimate available memory (conservative approach)
  let availableMemoryMB = 1024; // Default conservative estimate

  // Use experimental memory API if available
  if (
    typeof performance !== "undefined" &&
    "memory" in performance &&
    (performance as any).memory
  ) {
    const memory = (performance as any).memory;
    availableMemoryMB = Math.floor(memory.jsHeapSizeLimit / (1024 * 1024));
  }

  // Device classification based on capabilities
  const isLowEndDevice = cpuCores <= 2 || availableMemoryMB < 512;
  const isMobile =
    typeof navigator !== "undefined" &&
    /Android|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
      navigator.userAgent,
    );

  // Calculate max concurrency based on cores and device type
  let maxConcurrency = Math.max(1, Math.floor(cpuCores * 0.8));
  if (isLowEndDevice) maxConcurrency = Math.min(maxConcurrency, 2);
  if (isMobile) maxConcurrency = Math.min(maxConcurrency, 3);

  return {
    cpuCores,
    availableMemoryMB,
    isLowEndDevice,
    isMobile,
    maxConcurrency,
  };
}

/**
 * Smart Batch Sizing Manager
 * Maintains performance history and adapts batch sizes dynamically
 */
export class SmartBatchSizer {
  private metricsHistory: Map<string, ProcessingMetrics[]> = new Map();
  private currentBatchSizes: Map<string, number> = new Map();
  private deviceCapabilities: DeviceCapabilities;
  private readonly maxHistorySize = 10;

  constructor() {
    this.deviceCapabilities = detectDeviceCapabilities();
  }

  /**
   * Gets the optimal batch size for a given operation type and priority
   */
  getOptimalBatchSize(
    operationType: string,
    totalItems: number,
    priority: ProcessingPriority = ProcessingPriority.NORMAL,
  ): number {
    const config = DEFAULT_CONFIGS[operationType] || DEFAULT_CONFIGS.thumbnail;
    const history = this.metricsHistory.get(operationType) || [];

    let batchSize =
      this.currentBatchSizes.get(operationType) ||
      this.getInitialBatchSize(operationType);

    // Adjust based on recent performance
    if (history.length > 0) {
      batchSize = this.adaptBatchSize(batchSize, history, config);
    }

    // Apply user performance preferences
    batchSize = this.applyPerformancePreferences(batchSize);

    // Apply priority adjustments (after preferences to respect user choice)
    if (
      priority >= ProcessingPriority.HIGH &&
      globalPerformancePreferences.shouldPrioritizeUserOperations()
    ) {
      batchSize = Math.ceil(batchSize * config.priorityMultiplier);
    }

    // Apply device capability constraints
    batchSize = this.applyDeviceConstraints(batchSize, operationType);

    // Ensure batch size is within bounds and doesn't exceed total items
    batchSize = Math.max(
      config.minBatchSize,
      Math.min(config.maxBatchSize, batchSize, totalItems),
    );

    this.currentBatchSizes.set(operationType, batchSize);
    return batchSize;
  }

  /**
   * Records processing metrics for future batch size optimization
   */
  recordMetrics(metrics: ProcessingMetrics): void {
    const { operationType } = metrics;

    if (!this.metricsHistory.has(operationType)) {
      this.metricsHistory.set(operationType, []);
    }

    const history = this.metricsHistory.get(operationType)!;
    history.push({
      ...metrics,
      avgTimePerFile: metrics.processingTimeMs / metrics.filesProcessed,
    });

    // Keep only recent history
    if (history.length > this.maxHistorySize) {
      history.shift();
    }
  }

  /**
   * Checks if memory pressure is detected
   */
  isMemoryPressureDetected(): boolean {
    if (
      typeof performance !== "undefined" &&
      "memory" in performance &&
      (performance as any).memory
    ) {
      const memory = (performance as any).memory;
      const usedMemoryMB = memory.usedJSHeapSize / (1024 * 1024);
      const limitMemoryMB = memory.jsHeapSizeLimit / (1024 * 1024);

      // Consider memory pressure if using more than 80% of available
      return usedMemoryMB / limitMemoryMB > 0.8;
    }

    // Fallback: assume memory pressure on low-end devices
    return this.deviceCapabilities.isLowEndDevice;
  }

  /**
   * Gets initial batch size based on device capabilities
   */
  private getInitialBatchSize(operationType: string): number {
    const config = DEFAULT_CONFIGS[operationType] || DEFAULT_CONFIGS.thumbnail;
    const { isLowEndDevice, isMobile, maxConcurrency } =
      this.deviceCapabilities;

    let initialSize = Math.ceil(
      (config.minBatchSize + config.maxBatchSize) / 2,
    );

    // Reduce for low-end devices
    if (isLowEndDevice) {
      initialSize = Math.ceil(initialSize * 0.5);
    } else if (isMobile) {
      initialSize = Math.ceil(initialSize * 0.7);
    }

    // Consider CPU cores
    initialSize = Math.min(initialSize, maxConcurrency * 2);

    return Math.max(
      config.minBatchSize,
      Math.min(config.maxBatchSize, initialSize),
    );
  }

  /**
   * Adapts batch size based on performance history
   */
  private adaptBatchSize(
    currentSize: number,
    history: ProcessingMetrics[],
    config: BatchSizingConfig,
  ): number {
    const recentMetrics = history.slice(-3); // Consider last 3 operations
    const avgProcessingTime =
      recentMetrics.reduce((sum, m) => sum + m.processingTimeMs, 0) /
      recentMetrics.length;
    const avgErrorRate =
      recentMetrics.reduce((sum, m) => sum + m.errorRate, 0) /
      recentMetrics.length;

    let adjustment = 0;

    // If processing is too slow, reduce batch size
    if (avgProcessingTime > config.targetProcessingTimeMs * 1.2) {
      adjustment = -Math.ceil(currentSize * config.adaptationRate);
    }
    // If processing is fast and stable, increase batch size
    else if (
      avgProcessingTime < config.targetProcessingTimeMs * 0.8 &&
      avgErrorRate < 0.05
    ) {
      adjustment = Math.ceil(currentSize * config.adaptationRate);
    }

    // If high error rate, reduce batch size
    if (avgErrorRate > 0.1) {
      adjustment = Math.min(adjustment, -Math.ceil(currentSize * 0.3));
    }

    return Math.max(
      config.minBatchSize,
      Math.min(config.maxBatchSize, currentSize + adjustment),
    );
  }

  /**
   * Applies device-specific constraints to batch size
   */
  private applyDeviceConstraints(
    batchSize: number,
    operationType: string,
  ): number {
    const config = DEFAULT_CONFIGS[operationType] || DEFAULT_CONFIGS.thumbnail;

    // Apply user memory preference constraints
    const memoryMultiplier =
      globalPerformancePreferences.getMemoryConstraintMultiplier();

    // Reduce batch size under memory pressure or user preference
    if (this.isMemoryPressureDetected() || memoryMultiplier < 1.0) {
      batchSize = Math.ceil(batchSize * Math.min(0.6, memoryMultiplier));
    }

    // Additional constraints for mobile devices
    if (this.deviceCapabilities.isMobile) {
      batchSize = Math.min(batchSize, this.deviceCapabilities.maxConcurrency);
    }

    return Math.max(config.minBatchSize, batchSize);
  }

  /**
   * Applies user performance preferences to batch size
   */
  private applyPerformancePreferences(batchSize: number): number {
    const preferences = globalPerformancePreferences.getPreferences();
    const multiplier = globalPerformancePreferences.getBatchSizeMultiplier();

    // Apply the multiplier based on user preference
    batchSize = Math.ceil(batchSize * multiplier);

    // Special handling for adaptive mode
    if (preferences.profile === PerformanceProfile.ADAPTIVE) {
      // Use device capabilities more heavily
      if (this.deviceCapabilities.isLowEndDevice) {
        batchSize = Math.ceil(batchSize * 0.7);
      } else if (this.deviceCapabilities.cpuCores >= 8) {
        batchSize = Math.ceil(batchSize * 1.3);
      }
    }

    return batchSize;
  }

  /**
   * Gets device capabilities
   */
  getDeviceCapabilities(): DeviceCapabilities {
    return { ...this.deviceCapabilities };
  }

  /**
   * Resets metrics history (useful for testing or configuration changes)
   */
  resetMetrics(): void {
    this.metricsHistory.clear();
    this.currentBatchSizes.clear();
  }
}

// Global instance for the application
export const globalBatchSizer = new SmartBatchSizer();

/**
 * Hook-friendly utility for getting optimal batch size
 */
export function getOptimalBatchSize(
  operationType: string,
  totalItems: number,
  priority: ProcessingPriority = ProcessingPriority.NORMAL,
): number {
  return globalBatchSizer.getOptimalBatchSize(
    operationType,
    totalItems,
    priority,
  );
}

/**
 * Hook-friendly utility for recording processing metrics
 */
export function recordProcessingMetrics(metrics: ProcessingMetrics): void {
  globalBatchSizer.recordMetrics(metrics);
}
