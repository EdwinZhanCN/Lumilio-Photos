import { createContext, useContext, useRef, useEffect, ReactNode } from "react";
import { AppWorkerClient } from "@/workers/workerClient";

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
}

export const WorkerProvider = ({ children }: WorkerProviderProps) => {
  const workerClientRef = useRef<AppWorkerClient | null>(null);

  if (workerClientRef.current === null) {
    workerClientRef.current = new AppWorkerClient();
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
