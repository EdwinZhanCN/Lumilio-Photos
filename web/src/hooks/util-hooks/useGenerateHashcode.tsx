import { useState, useCallback, useEffect } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";

export interface HashcodeProgress {
  numberProcessed: number;
  total: number;
  error?: string;
}

interface PerformanceMetrics {
  startTime: number;
  fileCount: number;
  totalSize: number;
  processingTime: number;
  filesPerSecond: number;
  bytesPerSecond: number;
  numberProcessed: number;
}

interface HashcodeResult {
  hash: string;
  index: number;
}

export interface useGenerateHashcodeReturn {
  isGenerating: boolean;
  progress: HashcodeProgress | null;
  generateHashCodes: (
    files: FileList | File[],
  ) => Promise<HashcodeResult[] | undefined>;
  cancelGeneration: () => void;
}

/**
 * Custom hook for generating file hashcodes using a Web Worker.
 * Manages its own state for progress and generation status.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 * @author Edwin Zhan
 * @since 1.1.0
 * @returns {useGenerateHashcodeReturn} Hook state and actions for hashcode generation.
 */
export const useGenerateHashcode = (
  onPerformanceMetrics?: (metrics: PerformanceMetrics) => void,
): useGenerateHashcodeReturn => {
  const showMessage = useMessage();
  const workerClient = useWorker();

  const [isGenerating, setIsGenerating] = useState(false);
  const [progress, setProgress] = useState<HashcodeProgress | null>(null);

  useEffect(() => {
    // Only listen for progress if this hook instance is actively generating.
    if (!isGenerating) return;

    const removeProgressListener = workerClient.addProgressListener(
      ({ processed, total }) => {
        setProgress({ numberProcessed: processed, total });
      },
    );
    return () => removeProgressListener();
  }, [workerClient, isGenerating]);

  const generateHashCodes = useCallback(
    async (files: FileList | File[]): Promise<HashcodeResult[] | undefined> => {
      setIsGenerating(true);
      setProgress({ numberProcessed: 0, total: files.length });

      const startTime = performance.now();
      const totalSize = Array.from(files).reduce(
        (sum, file) => sum + file.size,
        0,
      );

      try {
        const { hashResults } = await workerClient.generateHash(files);
        const processingTime = performance.now() - startTime;

        if (onPerformanceMetrics) {
          onPerformanceMetrics({
            startTime,
            totalSize,
            processingTime,
            fileCount: files.length,
            numberProcessed: hashResults.length,
            filesPerSecond: files.length / (processingTime / 1000),
            bytesPerSecond: totalSize / (processingTime / 1000),
          });
        }

        if (!hashResults) {
          throw new Error("Hashcode generation failed: No result returned.");
        }

        return hashResults;
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : "Unknown hash generation error";
        showMessage("error", `HashCode generation failed: ${errorMessage}`);
        setProgress((prev) => (prev ? { ...prev, error: errorMessage } : null));
        return undefined; // Indicate failure
      } finally {
        setIsGenerating(false);
        // Do not clear progress immediately to show final state
        // It will be cleared on the next run.
      }
    },
    [workerClient, showMessage, onPerformanceMetrics],
  );

  const cancelGeneration = () => {
    workerClient.abortGenerateHash();
    setIsGenerating(false);
    setProgress(null);
    showMessage("info", "Hash generation cancelled.");
  };

  return { isGenerating, progress, generateHashCodes, cancelGeneration };
};
