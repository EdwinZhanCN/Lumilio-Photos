import { useEffect, useState, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import {
  getOptimalBatchSize,
  recordProcessingMetrics,
  ProcessingPriority
} from "@/lib/utils/smartBatchSizing.ts";

// Define the shape of the progress state for better type safety
export type ExifExtractionProgress = {
  processed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

export interface useExtractExifdataReturn {
  isExtracting: boolean;
  exifData: Record<number, any> | null;
  progress: ExifExtractionProgress;
  extractExifData: (files: File[], priority?: ProcessingPriority) => Promise<void>;
  cancelExtraction: () => void;
}

/**
 * A hook to extract EXIF data from files using the shared web worker client.
 * It encapsulates the state for the extraction process, progress, and results.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 * @author Edwin Zhan
 * @since 1.1.0
 */
export const useExtractExifdata = (): useExtractExifdataReturn => {
  const showMessage = useMessage();
  const workerClient = useWorker(); // Get the shared worker client instance

  // All states are now managed inside the hook
  const [isExtracting, setIsExtracting] = useState(false);
  const [exifData, setExifData] = useState<Record<number, any> | null>(null);
  const [progress, setProgress] = useState<ExifExtractionProgress>(null);

  // Effect for listening to worker progress
  useEffect(() => {
    // The useWorker hook ensures workerClient is always available here
    const progressListener = workerClient.addProgressListener((detail) => {
      if (detail && typeof detail.processed === "number") {
        setProgress({
          processed: detail.processed,
          total: detail.total,
        });
      }
    });

    // Cleanup function to remove the listener
    return () => {
      progressListener();
    };
  }, [workerClient]); // Depend on the workerClient instance

  /**
   * Extracts EXIF data from the given files using smart batch sizing.
   * The function is wrapped in useCallback for performance optimization,
   * ensuring it's not recreated on every render unless its dependencies change.
   */
  const extractExifData = useCallback(
    async (files: File[], priority: ProcessingPriority = ProcessingPriority.NORMAL): Promise<void> => {
      setIsExtracting(true);
      setExifData(null); // Reset previous data
      setProgress({ processed: 0, total: files.length });

      const startTime = performance.now();

      try {
        // Record anticipated batch size for performance tracking
        const anticipatedBatchSize = getOptimalBatchSize("exif", files.length, priority);
        
        const results = await workerClient.extractExif(files);
        const processingTime = performance.now() - startTime;

        if (results && results.exifResults) {
          const formattedExifData = results.exifResults.reduce(
            (acc, item) => {
              acc[item.index] = item.exifData;
              return acc;
            },
            {} as Record<number, any>,
          );
          setExifData(formattedExifData);

          // Record successful processing metrics
          recordProcessingMetrics({
            operationType: "exif",
            batchSize: anticipatedBatchSize,
            processingTimeMs: processingTime,
            filesProcessed: results.exifResults.length,
            avgTimePerFile: processingTime / results.exifResults.length,
            success: true,
            errorRate: Math.max(0, (files.length - results.exifResults.length) / files.length),
          });
        } else {
          setExifData(null);
          
          // Record failure metrics
          recordProcessingMetrics({
            operationType: "exif",
            batchSize: anticipatedBatchSize,
            processingTimeMs: processingTime,
            filesProcessed: 0,
            avgTimePerFile: 0,
            success: false,
            errorRate: 1.0,
          });
        }
      } catch (error) {
        const errorMessage = (error as Error).message;
        showMessage("error", `Failed to extract EXIF data: ${errorMessage}`);
        setProgress((prev) => ({
          ...prev!,
          error: errorMessage,
          failedAt: prev?.processed,
        }));

        // Record error metrics
        const processingTime = performance.now() - startTime;
        recordProcessingMetrics({
          operationType: "exif",
          batchSize: files.length,
          processingTimeMs: processingTime,
          filesProcessed: 0,
          avgTimePerFile: 0,
          success: false,
          errorRate: 1.0,
        });
      } finally {
        setIsExtracting(false);
        // Do not clear progress here to allow UI to show final state
      }
    },
    [workerClient, showMessage], // Dependencies for useCallback
  );

  const cancelExtraction = () => {
    workerClient.abortExtractExif();
    setIsExtracting(false);
    setProgress(null);
    showMessage("info", "EXIF extraction cancelled.");
  };

  // Return the state values and the function to trigger the process
  return {
    isExtracting,
    exifData,
    progress,
    extractExifData,
    cancelExtraction,
  };
};
