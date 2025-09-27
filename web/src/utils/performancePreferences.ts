/**
 * Performance Preferences System
 * 
 * Manages user preferences for memory vs speed trade-offs in batch processing.
 * 
 * @author Edwin Zhan
 * @since 1.1.0
 */

export enum PerformanceProfile {
  MEMORY_SAVER = "memory_saver",    // Optimize for low memory usage
  BALANCED = "balanced",            // Balance memory and speed
  SPEED_OPTIMIZED = "speed",        // Optimize for fastest processing
  ADAPTIVE = "adaptive",            // Let system decide based on device
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

const STORAGE_KEY = 'lumilio_performance_preferences';

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
   * Loads preferences from localStorage
   */
  private loadPreferences(): PerformancePreferences {
    try {
      if (typeof localStorage !== 'undefined') {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
          const parsed = JSON.parse(stored);
          return { ...DEFAULT_PREFERENCES, ...parsed };
        }
      }
    } catch (error) {
      console.warn('Failed to load performance preferences:', error);
    }
    return { ...DEFAULT_PREFERENCES };
  }

  /**
   * Saves preferences to localStorage
   */
  private savePreferences(): void {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(STORAGE_KEY, JSON.stringify(this.preferences));
      }
    } catch (error) {
      console.warn('Failed to save performance preferences:', error);
    }
  }

  /**
   * Notifies all listeners of preference changes
   */
  private notifyListeners(): void {
    this.listeners.forEach(listener => {
      try {
        listener(this.preferences);
      } catch (error) {
        console.warn('Performance preferences listener error:', error);
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
  return {
    preferences: globalPerformancePreferences.getPreferences(),
    updatePreferences: (updates: Partial<PerformancePreferences>) => 
      globalPerformancePreferences.updatePreferences(updates),
    resetToDefaults: () => globalPerformancePreferences.resetToDefaults(),
  };
}