/**
 * Performance Preferences System
 *
 * Manages user preferences for memory vs speed trade-offs in batch processing.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */

import { useCallback, useSyncExternalStore } from "react";

import {
  LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY,
  PERFORMANCE_PREFERENCES_STORAGE_KEY,
  PERFORMANCE_PREFERENCES_STORAGE_VERSION,
} from "@/lib/settings/registry";
import {
  isRecord,
  readVersionedStorageCandidate,
  removeStorageKeys,
  writeVersionedStorageData,
} from "@/lib/settings/storage";

export enum PerformanceProfile {
  MEMORY_SAVER = "memory_saver", // Optimize for low memory usage
  BALANCED = "balanced", // Balance memory and speed
  SPEED_OPTIMIZED = "speed", // Optimize for fastest processing
  ADAPTIVE = "adaptive", // Let system decide based on device
}

export interface PerformancePreferences {
  profile: PerformanceProfile;
  customBatchSizeMultiplier?: number; // 0.5 to 2.0, only used in custom mode
  respectMemoryLimits: boolean;
  prioritizeUserOperations: boolean;
  maxConcurrentOperations: number;
}

const DEFAULT_PREFERENCES: PerformancePreferences = {
  profile: PerformanceProfile.ADAPTIVE,
  respectMemoryLimits: true,
  prioritizeUserOperations: true,
  maxConcurrentOperations: 3,
};

function clampInt(
  value: unknown,
  min: number,
  max: number,
  fallback: number,
): number {
  if (typeof value !== "number" || !Number.isFinite(value)) return fallback;
  const n = Math.floor(value);
  return Math.min(max, Math.max(min, n));
}

function asProfile(
  value: unknown,
  fallback: PerformanceProfile,
): PerformanceProfile {
  return value === PerformanceProfile.MEMORY_SAVER ||
    value === PerformanceProfile.BALANCED ||
    value === PerformanceProfile.SPEED_OPTIMIZED ||
    value === PerformanceProfile.ADAPTIVE
    ? value
    : fallback;
}

function sanitizePreferences(candidate: unknown): PerformancePreferences {
  if (!isRecord(candidate)) {
    return { ...DEFAULT_PREFERENCES };
  }

  const customBatchSizeMultiplier =
    typeof candidate.customBatchSizeMultiplier === "number" &&
    Number.isFinite(candidate.customBatchSizeMultiplier)
      ? Math.min(2.0, Math.max(0.5, candidate.customBatchSizeMultiplier))
      : undefined;

  return {
    profile: asProfile(candidate.profile, DEFAULT_PREFERENCES.profile),
    customBatchSizeMultiplier,
    respectMemoryLimits:
      typeof candidate.respectMemoryLimits === "boolean"
        ? candidate.respectMemoryLimits
        : DEFAULT_PREFERENCES.respectMemoryLimits,
    prioritizeUserOperations:
      typeof candidate.prioritizeUserOperations === "boolean"
        ? candidate.prioritizeUserOperations
        : DEFAULT_PREFERENCES.prioritizeUserOperations,
    maxConcurrentOperations: clampInt(
      candidate.maxConcurrentOperations,
      1,
      8,
      DEFAULT_PREFERENCES.maxConcurrentOperations,
    ),
  };
}

/**
 * Performance Preferences Manager
 */
export class PerformancePreferencesManager {
  private preferences: PerformancePreferences;
  private listeners: Set<(prefs: PerformancePreferences) => void> = new Set();

  constructor() {
    this.preferences = this.loadPreferences();
  }

  /**
   * Gets current performance preferences
   */
  getPreferences(): PerformancePreferences {
    return { ...this.preferences };
  }

  /**
   * Gets the current immutable snapshot for React subscriptions.
   */
  getSnapshot(): PerformancePreferences {
    return this.preferences;
  }

  /**
   * Updates performance preferences
   */
  updatePreferences(updates: Partial<PerformancePreferences>): void {
    this.preferences = { ...this.preferences, ...updates };
    this.savePreferences();
    this.notifyListeners();
  }

  /**
   * Gets batch size multiplier based on current profile
   */
  getBatchSizeMultiplier(): number {
    // If custom multiplier is set, use it regardless of profile
    if (this.preferences.customBatchSizeMultiplier !== undefined) {
      return this.preferences.customBatchSizeMultiplier;
    }

    switch (this.preferences.profile) {
      case PerformanceProfile.MEMORY_SAVER:
        return 0.6; // Reduce batch sizes by 40%
      case PerformanceProfile.BALANCED:
        return 1.0; // Use default batch sizes
      case PerformanceProfile.SPEED_OPTIMIZED:
        return 1.5; // Increase batch sizes by 50%
      case PerformanceProfile.ADAPTIVE:
        return 1.0; // Let device capabilities decide
      default:
        return 1.0;
    }
  }

  /**
   * Gets memory constraint multiplier
   */
  getMemoryConstraintMultiplier(): number {
    if (!this.preferences.respectMemoryLimits) {
      return 1.0;
    }

    switch (this.preferences.profile) {
      case PerformanceProfile.MEMORY_SAVER:
        return 0.5; // Very strict memory limits
      case PerformanceProfile.BALANCED:
        return 0.8; // Moderate memory limits
      case PerformanceProfile.SPEED_OPTIMIZED:
        return 1.2; // Relaxed memory limits
      case PerformanceProfile.ADAPTIVE:
        return 0.8; // Default conservative approach
      default:
        return 0.8;
    }
  }

  /**
   * Checks if priority operations should be enhanced
   */
  shouldPrioritizeUserOperations(): boolean {
    return this.preferences.prioritizeUserOperations;
  }

  /**
   * Gets maximum concurrent operations allowed
   */
  getMaxConcurrentOperations(): number {
    return this.preferences.maxConcurrentOperations;
  }

  /**
   * Resets preferences to defaults
   */
  resetToDefaults(): void {
    this.preferences = { ...DEFAULT_PREFERENCES };
    this.savePreferences();
    this.notifyListeners();
  }

  /**
   * Adds a listener for preference changes
   */
  addListener(listener: (prefs: PerformancePreferences) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  /**
   * Subscribes to any preference change, for useSyncExternalStore.
   */
  subscribe(onStoreChange: () => void): () => void {
    return this.addListener(() => {
      onStoreChange();
    });
  }

  /**
   * Loads preferences from localStorage
   */
  private loadPreferences(): PerformancePreferences {
    try {
      const readResult = readVersionedStorageCandidate({
        key: PERFORMANCE_PREFERENCES_STORAGE_KEY,
        version: PERFORMANCE_PREFERENCES_STORAGE_VERSION,
        legacyKeys: [LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY],
      });

      if (readResult.candidate !== null) {
        const normalized = sanitizePreferences(readResult.candidate);
        if (readResult.needsRewrite) {
          this.writePreferencesEnvelope(normalized);
          removeStorageKeys([LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY]);
        }
        return normalized;
      }
    } catch (error) {
      console.warn("Failed to load performance preferences:", error);
    }
    return { ...DEFAULT_PREFERENCES };
  }

  /**
   * Saves preferences to localStorage
   */
  private savePreferences(): void {
    try {
      this.writePreferencesEnvelope(this.preferences);
      removeStorageKeys([LEGACY_PERFORMANCE_PREFERENCES_STORAGE_KEY]);
    } catch (error) {
      console.warn("Failed to save performance preferences:", error);
    }
  }

  private writePreferencesEnvelope(preferences: PerformancePreferences): void {
    writeVersionedStorageData<PerformancePreferences>(
      PERFORMANCE_PREFERENCES_STORAGE_KEY,
      PERFORMANCE_PREFERENCES_STORAGE_VERSION,
      preferences,
    );
  }

  /**
   * Notifies all listeners of preference changes
   */
  private notifyListeners(): void {
    this.listeners.forEach((listener) => {
      try {
        listener(this.preferences);
      } catch (error) {
        console.warn("Performance preferences listener error:", error);
      }
    });
  }
}

// Global instance
export const globalPerformancePreferences = new PerformancePreferencesManager();

/**
 * React hook for using performance preferences
 */
export function usePerformancePreferences() {
  const preferences = useSyncExternalStore(
    (onStoreChange) => globalPerformancePreferences.subscribe(onStoreChange),
    () => globalPerformancePreferences.getSnapshot(),
    () => globalPerformancePreferences.getSnapshot(),
  );

  const updatePreferences = useCallback(
    (updates: Partial<PerformancePreferences>) =>
      globalPerformancePreferences.updatePreferences(updates),
    [],
  );

  const resetToDefaults = useCallback(
    () => globalPerformancePreferences.resetToDefaults(),
    [],
  );

  return {
    preferences,
    updatePreferences,
    resetToDefaults,
  };
}
