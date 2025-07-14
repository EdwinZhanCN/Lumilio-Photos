import React, { useEffect, useState, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { WorkerClient } from "@/workers/workerClient.ts";

// Define the shape of the progress state for better type safety
type ExifExtractionProgress = {
  numberProcessed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

// The hook only needs the worker reference
type UseExtractExifdataProps = {
  workerClientRef: React.RefObject<WorkerClient | null>;
};

/**
 * A hook to extract EXIF data from files using a web worker.
 * It encapsulates the state for the extraction process, progress, and results.
 */
export const useExtractExifdata = ({
  workerClientRef,
}: UseExtractExifdataProps) => {
  const showMessage = useMessage();

  // All state is now managed inside the hook
  const [isExtracting, setIsExtracting] = useState(false);
  const [exifData, setExifData] = useState<Record<number, any> | null>(null);
  const [progress, setProgress] = useState<ExifExtractionProgress>(null);

  // Effect for listening to worker progress
  useEffect(() => {
    if (!workerClientRef.current) {
      return;
    }

    const progressListener = workerClientRef.current.addProgressListener(
      (detail) => {
        if (detail && typeof detail.processed === "number") {
          setProgress((prev) => {
            if (!prev) return prev; // Should not happen if progress is set before starting
            return {
              ...prev,
              numberProcessed: detail.processed,
              total: detail.total,
            };
          });
        }
      },
    );

    // Cleanup function to remove the listener
    return () => {
      progressListener();
    };
  }, [workerClientRef]); // Dependency array is simpler now

  /**
   * Extracts EXIF data from the given files.
   * The function is wrapped in useCallback for performance optimization,
   * ensuring it's not recreated on every render unless its dependencies change.
   */
  const extractExifData = useCallback(
    async (files: File[]): Promise<void | Error> => {
      if (!workerClientRef.current) {
        const error = new Error("Worker client is not initialized");
        showMessage("error", error.message);
        return error;
      }

      setIsExtracting(true);
      setExifData(null); // Reset previous data
      setProgress({ numberProcessed: 0, total: files.length });

      try {
        const results = await workerClientRef.current.extractExif(files);

        if (results && results.exifResults) {
          const formattedExifData = results.exifResults.reduce(
            (acc, item) => {
              acc[item.index] = item.exifData;
              return acc;
            },
            {} as Record<number, any>,
          );
          setExifData(formattedExifData);
        } else {
          setExifData(null);
        }
      } catch (error) {
        const errorMessage = (error as Error).message;
        showMessage("error", `Failed to extract EXIF data: ${errorMessage}`);
        setProgress((prev) => ({
          ...prev!,
          error: errorMessage,
          failedAt: prev?.numberProcessed,
        }));
        return error as Error;
      } finally {
        setIsExtracting(false);
      }
    },
    [workerClientRef, showMessage],
  ); // Dependencies for useCallback

  // Return the state values and the function to trigger the process
  return {
    isExtracting,
    exifData,
    progress,
    extractExifData,
  };
};
