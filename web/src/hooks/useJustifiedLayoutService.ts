import { useState, useEffect } from 'react';
import { justifiedLayoutService } from '@/services/justifiedLayoutService';

export interface UseJustifiedLayoutServiceResult {
  isReady: boolean;
  isLoading: boolean;
  error: string | null;
  initialize: () => Promise<void>;
}

/**
 * Hook for managing the justified layout service initialization
 * Provides status and control over the WASM-based layout service
 */
export const useJustifiedLayoutService = (): UseJustifiedLayoutServiceResult => {
  const [isReady, setIsReady] = useState(justifiedLayoutService.isReady());
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const initialize = async (): Promise<void> => {
    if (isReady || isLoading) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      await justifiedLayoutService.initialize();
      setIsReady(true);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Unknown error';
      setError(errorMessage);
      console.error('Failed to initialize justified layout service:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Auto-initialize on mount
  useEffect(() => {
    if (!isReady && !isLoading && !error) {
      initialize();
    }
  }, [isReady, isLoading, error]);

  return {
    isReady,
    isLoading,
    error,
    initialize,
  };
};

/**
 * Hook for preloading the justified layout service
 * Use this in components that will need the service later
 */
export const usePreloadJustifiedLayoutService = (): void => {
  useEffect(() => {
    // Preload in the background without blocking
    if (!justifiedLayoutService.isReady()) {
      justifiedLayoutService.initialize().catch(() => {
        // Silently fail for preload
      });
    }
  }, []);
};
