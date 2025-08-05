import { createContext, useContext, useRef, useEffect, ReactNode } from "react";
import { AppWorkerClient, WorkerType } from "@/workers/workerClient";
import { ModelRecord } from "@mlc-ai/web-llm";
import { useSettings } from "@/contexts/SettingsContext";

const WorkerContext = createContext<AppWorkerClient | null>(null);

/**
 * Custom hook to safely access the AppWorkerClient instance from the context.
 * It ensures that the hook is used within a component wrapped by WorkerProvider.
 *
 * @returns {AppWorkerClient} The shared instance of the worker client.
 * @throws {Error} If the hook is used outside of a WorkerProvider.
 */
export const useWorker = (): AppWorkerClient => {
  const context = useContext(WorkerContext);
  if (!context) {
    throw new Error("useWorker() must be used within a <WorkerProvider>");
  }
  return context;
};

interface WorkerProviderProps {
  children: ReactNode;
  /**
   * Array of worker types to pre-load immediately when the provider mounts.
   * Workers not in this list will be lazy-loaded on first use.
   *
   * @example
   * // Pre-load thumbnail and hash workers for immediate use
   * <WorkerProvider preload={['thumbnail', 'hash']}>
   *
   * @example
   * // No pre-loading - all workers lazy-loaded (good for Assets page)
   * <WorkerProvider>
   */
  preload?: WorkerType[];
  webllmConfig?: {
    modelRecords?: ModelRecord[];
    useIndexedDBCache?: boolean;
    modelId: string;
  };
}

export const WorkerProvider = ({
  children,
  preload,
  webllmConfig,
}: WorkerProviderProps) => {
  const { settings } = useSettings();
  const workerClientRef = useRef<AppWorkerClient | null>(null);

  if (workerClientRef.current === null) {
    // Use settings if available and no explicit webllmConfig provided
    const finalWebllmConfig =
      webllmConfig ||
      (settings.lumen && settings.lumen.enabled
        ? {
            modelRecords: settings.lumen.modelRecords || [],
            useIndexedDBCache: true,
            modelId: settings.lumen.model,
          }
        : undefined);

    // Only create worker client if we have valid configuration
    if (
      finalWebllmConfig &&
      (!finalWebllmConfig.modelRecords ||
        finalWebllmConfig.modelRecords.length === 0)
    ) {
      console.warn(
        "WorkerProvider: No model records available, WebLLM functionality may not work",
      );
    }

    workerClientRef.current = new AppWorkerClient({
      preload,
      webllmConfig: finalWebllmConfig,
    });
  }

  useEffect(() => {
    const client = workerClientRef.current;
    return () => {
      client?.terminateAllWorkers();
    };
  }, []);

  return (
    <WorkerContext.Provider value={workerClientRef.current}>
      {children}
    </WorkerContext.Provider>
  );
};
