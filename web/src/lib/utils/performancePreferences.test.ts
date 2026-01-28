import { describe, it, expect, beforeEach, vi } from 'vitest';
import { 
  PerformancePreferencesManager, 
  PerformanceProfile,
  globalPerformancePreferences,
  usePerformancePreferences
} from './performancePreferences.ts';

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
};

Object.defineProperty(window, 'localStorage', {
  value: localStorageMock,
  configurable: true,
});

describe('PerformancePreferencesManager', () => {
  let manager: PerformancePreferencesManager;

  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.getItem.mockReturnValue(null);
    manager = new PerformancePreferencesManager();
  });

  describe('initialization', () => {
    it('should initialize with default preferences', () => {
      const preferences = manager.getPreferences();
      expect(preferences.profile).toBe(PerformanceProfile.ADAPTIVE);
      expect(preferences.respectMemoryLimits).toBe(true);
      expect(preferences.prioritizeUserOperations).toBe(true);
      expect(preferences.maxConcurrentOperations).toBe(3);
    });

    it('should load preferences from localStorage if available', () => {
      const storedPreferences = {
        profile: PerformanceProfile.SPEED_OPTIMIZED,
        respectMemoryLimits: false,
        maxConcurrentOperations: 5,
      };
      
      localStorageMock.getItem.mockReturnValue(JSON.stringify(storedPreferences));
      
      const newManager = new PerformancePreferencesManager();
      const preferences = newManager.getPreferences();
      
      expect(preferences.profile).toBe(PerformanceProfile.SPEED_OPTIMIZED);
      expect(preferences.respectMemoryLimits).toBe(false);
      expect(preferences.maxConcurrentOperations).toBe(5);
      expect(preferences.prioritizeUserOperations).toBe(true); // Should merge with defaults
    });
  });

  describe('updatePreferences', () => {
    it('should update preferences and save to localStorage', () => {
      manager.updatePreferences({ profile: PerformanceProfile.MEMORY_SAVER });
      
      const preferences = manager.getPreferences();
      expect(preferences.profile).toBe(PerformanceProfile.MEMORY_SAVER);
      expect(localStorageMock.setItem).toHaveBeenCalledWith(
        'lumilio_performance_preferences',
        expect.stringContaining(PerformanceProfile.MEMORY_SAVER)
      );
    });

    it('should notify listeners when preferences change', () => {
      const listener = vi.fn();
      const removeListener = manager.addListener(listener);
      
      manager.updatePreferences({ profile: PerformanceProfile.SPEED_OPTIMIZED });
      
      expect(listener).toHaveBeenCalledWith(
        expect.objectContaining({ profile: PerformanceProfile.SPEED_OPTIMIZED })
      );
      
      removeListener();
    });
  });

  describe('getBatchSizeMultiplier', () => {
    it('should return correct multiplier for MEMORY_SAVER', () => {
      manager.updatePreferences({ profile: PerformanceProfile.MEMORY_SAVER });
      expect(manager.getBatchSizeMultiplier()).toBe(0.6);
    });

    it('should return correct multiplier for BALANCED', () => {
      manager.updatePreferences({ profile: PerformanceProfile.BALANCED });
      expect(manager.getBatchSizeMultiplier()).toBe(1.0);
    });

    it('should return correct multiplier for SPEED_OPTIMIZED', () => {
      manager.updatePreferences({ profile: PerformanceProfile.SPEED_OPTIMIZED });
      expect(manager.getBatchSizeMultiplier()).toBe(1.5);
    });

    it('should return correct multiplier for ADAPTIVE', () => {
      manager.updatePreferences({ profile: PerformanceProfile.ADAPTIVE });
      expect(manager.getBatchSizeMultiplier()).toBe(1.0);
    });

    it('should use custom multiplier when provided', () => {
      manager.updatePreferences({ 
        profile: PerformanceProfile.BALANCED, // This will be overridden
        customBatchSizeMultiplier: 1.3 
      });
      expect(manager.getBatchSizeMultiplier()).toBe(1.3);
    });
  });

  describe('getMemoryConstraintMultiplier', () => {
    it('should return 1.0 when respectMemoryLimits is false', () => {
      manager.updatePreferences({ respectMemoryLimits: false });
      expect(manager.getMemoryConstraintMultiplier()).toBe(1.0);
    });

    it('should return correct multiplier for MEMORY_SAVER when respecting limits', () => {
      manager.updatePreferences({ 
        profile: PerformanceProfile.MEMORY_SAVER,
        respectMemoryLimits: true 
      });
      expect(manager.getMemoryConstraintMultiplier()).toBe(0.5);
    });

    it('should return correct multiplier for SPEED_OPTIMIZED when respecting limits', () => {
      manager.updatePreferences({ 
        profile: PerformanceProfile.SPEED_OPTIMIZED,
        respectMemoryLimits: true 
      });
      expect(manager.getMemoryConstraintMultiplier()).toBe(1.2);
    });
  });

  describe('resetToDefaults', () => {
    it('should reset all preferences to defaults', () => {
      // Change some preferences
      manager.updatePreferences({ 
        profile: PerformanceProfile.SPEED_OPTIMIZED,
        respectMemoryLimits: false,
        maxConcurrentOperations: 8 
      });
      
      // Reset to defaults
      manager.resetToDefaults();
      
      const preferences = manager.getPreferences();
      expect(preferences.profile).toBe(PerformanceProfile.ADAPTIVE);
      expect(preferences.respectMemoryLimits).toBe(true);
      expect(preferences.maxConcurrentOperations).toBe(3);
    });

    it('should notify listeners when reset', () => {
      const listener = vi.fn();
      manager.addListener(listener);
      
      manager.resetToDefaults();
      
      expect(listener).toHaveBeenCalledWith(
        expect.objectContaining({ profile: PerformanceProfile.ADAPTIVE })
      );
    });
  });

  describe('listener management', () => {
    it('should add and remove listeners correctly', () => {
      const listener1 = vi.fn();
      const listener2 = vi.fn();
      
      const remove1 = manager.addListener(listener1);
      const remove2 = manager.addListener(listener2);
      
      manager.updatePreferences({ profile: PerformanceProfile.MEMORY_SAVER });
      
      expect(listener1).toHaveBeenCalledTimes(1);
      expect(listener2).toHaveBeenCalledTimes(1);
      
      // Remove one listener
      remove1();
      
      manager.updatePreferences({ profile: PerformanceProfile.BALANCED });
      
      expect(listener1).toHaveBeenCalledTimes(1); // Not called again
      expect(listener2).toHaveBeenCalledTimes(2); // Called again
      
      remove2();
    });

    it('should handle listener errors gracefully', () => {
      const errorListener = vi.fn().mockImplementation(() => {
        throw new Error('Listener error');
      });
      const workingListener = vi.fn();
      
      manager.addListener(errorListener);
      manager.addListener(workingListener);
      
      // Should not throw even if one listener errors
      expect(() => {
        manager.updatePreferences({ profile: PerformanceProfile.MEMORY_SAVER });
      }).not.toThrow();
      
      expect(workingListener).toHaveBeenCalled();
    });
  });
});

describe('Global instance', () => {
  it('should provide a global instance', () => {
    expect(globalPerformancePreferences).toBeInstanceOf(PerformancePreferencesManager);
  });

  it('should maintain state across calls', () => {
    globalPerformancePreferences.updatePreferences({ profile: PerformanceProfile.SPEED_OPTIMIZED });
    
    const preferences1 = globalPerformancePreferences.getPreferences();
    const preferences2 = globalPerformancePreferences.getPreferences();
    
    expect(preferences1.profile).toBe(PerformanceProfile.SPEED_OPTIMIZED);
    expect(preferences2.profile).toBe(PerformanceProfile.SPEED_OPTIMIZED);
  });
});

describe('usePerformancePreferences hook', () => {
  it('should return preferences and update functions', () => {
    const hookResult = usePerformancePreferences();
    
    expect(hookResult).toHaveProperty('preferences');
    expect(hookResult).toHaveProperty('updatePreferences');
    expect(hookResult).toHaveProperty('resetToDefaults');
    expect(typeof hookResult.updatePreferences).toBe('function');
    expect(typeof hookResult.resetToDefaults).toBe('function');
  });

  it('should use the global instance', () => {
    const hookResult = usePerformancePreferences();
    
    // Should match global preferences
    expect(hookResult.preferences).toEqual(globalPerformancePreferences.getPreferences());
  });
});