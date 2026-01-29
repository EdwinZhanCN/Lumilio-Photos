import { useState, useEffect, useCallback } from "react";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import type {
  LayoutBox,
  LayoutConfig,
  LayoutResult,
} from "@/lib/layout/justifiedLayout.ts";

export interface UseJustifiedLayoutServiceResult {
  isReady: boolean;
  isLoading: boolean;
  error: string | null;
  initialize: () => Promise<void>;
  calculateLayout: (
    boxes: LayoutBox[],
    config: LayoutConfig,
  ) => Promise<LayoutResult>;
  calculateMultipleLayouts: (
    groups: Record<string, LayoutBox[]>,
    config: LayoutConfig,
  ) => Promise<Record<string, LayoutResult>>;
}

/**
 * Hook for managing the justified layout worker initialization
 * Provides status and access to the worker-backed layout calculation
 */
export const useJustifiedLayoutService = (): UseJustifiedLayoutServiceResult => {
  const workerClient = useWorker();
  const [isReady, setIsReady] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const initialize = useCallback(async (): Promise<void> => {
    if (isReady || isLoading) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      await workerClient.initializeJustifiedLayout();
      setIsReady(true);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Unknown error";
      setError(errorMessage);
      console.error("Failed to initialize justified layout worker:", err);
    } finally {
      setIsLoading(false);
    }
  }, [isReady, isLoading, workerClient]);

  const calculateLayout = useCallback(
    async (boxes: LayoutBox[], config: LayoutConfig): Promise<LayoutResult> => {
      await initialize();
      return workerClient.calculateJustifiedLayout(boxes, config);
    },
    [initialize, workerClient],
  );

  const calculateMultipleLayouts = useCallback(
    async (
      groups: Record<string, LayoutBox[]>,
      config: LayoutConfig,
    ): Promise<Record<string, LayoutResult>> => {
      await initialize();
      return workerClient.calculateJustifiedLayouts(groups, config);
    },
    [initialize, workerClient],
  );

  // Auto-initialize on mount
  useEffect(() => {
    if (!isReady && !isLoading && !error) {
      initialize();
    }
  }, [isReady, isLoading, error, initialize]);

  return {
    isReady,
    isLoading,
    error,
    initialize,
    calculateLayout,
    calculateMultipleLayouts,
  };
};

/**
 * Hook for preloading the justified layout service
 * Use this in components that will need the service later
 */
export const usePreloadJustifiedLayoutService = (): void => {
  const workerClient = useWorker();

  useEffect(() => {
    workerClient.initializeJustifiedLayout().catch(() => {
      // Silently fail for preload
    });
  }, [workerClient]);
};
