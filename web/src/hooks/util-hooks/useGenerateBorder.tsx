import React, { useState, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import { 
  getOptimalBatchSize, 
  recordProcessingMetrics, 
  ProcessingPriority 
} from "@/utils/smartBatchSizing";

export type BorderOptions = "COLORED" | "FROSTED" | "VIGNETTE";

export type BorderParams = {
  COLORED: {
    border_width: number;
    r: number;
    g: number;
    b: number;
    jpeg_quality: number;
  };
  FROSTED: {
    blur_sigma: number;
    brightness_adjustment: number;
    corner_radius: number;
    jpeg_quality: number;
  };
  VIGNETTE: {
    strength: number;
    jpeg_quality: number;
  };
};

export type ProcessedImageMap = {
  [uuid: string]: {
    originalFileName: string;
    borderedFileURL?: string;
    error?: string;
  };
};

// Define the shape of the progress state for border generation
export type BorderGenerationProgress = {
  processed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

export interface UseGenerateBordersReturn {
  isGenerating: boolean;
  processedImages: ProcessedImageMap;
  progress: BorderGenerationProgress;
  setProcessedImages: React.Dispatch<React.SetStateAction<ProcessedImageMap>>;
  generateBorders: (
    files: File[],
    option: BorderOptions,
    param: BorderParams[BorderOptions],
    priority?: ProcessingPriority,
  ) => Promise<void>;
  cancelGeneration: () => void;
}

/**
 * Custom hook to generate images with borders using the shared web worker client.
 * It encapsulates all state related to the generation process.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 * @returns {UseGenerateBordersReturn} Hook state and actions for border generation.
 */
export const useGenerateBorders = (): UseGenerateBordersReturn => {
  const showMessage = useMessage();
  const workerClient = useWorker();

  const [isGenerating, setIsGenerating] = useState(false);
  const [processedImages, setProcessedImages] = useState<ProcessedImageMap>({});
  const [progress, setProgress] = useState<BorderGenerationProgress>(null);

  const generateBorders = useCallback(
    async (
      files: File[],
      option: BorderOptions,
      param: BorderParams[BorderOptions],
      priority: ProcessingPriority = ProcessingPriority.NORMAL,
    ): Promise<void> => {
      setIsGenerating(true);
      setProcessedImages({}); // Clear previous results
      setProgress({ processed: 0, total: files.length });

      const startTime = performance.now();
      let totalErrors = 0;

      try {
        // Get optimal batch size based on device capabilities and processing history
        const batchSize = getOptimalBatchSize("border", files.length, priority);
        
        // Process files in dynamic batches
        for (let i = 0; i < files.length; i += batchSize) {
          const batch = files.slice(i, i + batchSize);
          const batchStartTime = performance.now();
          
          try {
            const resultsMap = await workerClient.generateBorders(
              batch,
              option,
              param,
            );

            // Merge results from the current batch into the main state
            setProcessedImages((prev) => ({
              ...prev,
              ...resultsMap,
            }));

            // Update progress
            setProgress((prev) => ({
              processed: (prev?.processed || 0) + batch.length,
              total: files.length,
            }));

            // Record successful batch metrics
            const batchProcessingTime = performance.now() - batchStartTime;
            recordProcessingMetrics({
              operationType: "border",
              batchSize: batch.length,
              processingTimeMs: batchProcessingTime,
              filesProcessed: batch.length,
              avgTimePerFile: batchProcessingTime / batch.length,
              success: true,
              errorRate: 0,
            });

          } catch (batchError) {
            totalErrors += batch.length;
            console.warn(`Border generation batch ${i / batchSize + 1} failed:`, batchError);
            
            // Record failed batch metrics
            const batchProcessingTime = performance.now() - batchStartTime;
            recordProcessingMetrics({
              operationType: "border",
              batchSize: batch.length,
              processingTimeMs: batchProcessingTime,
              filesProcessed: 0, // No files processed in failed batch
              avgTimePerFile: 0,
              success: false,
              errorRate: 1.0,
            });

            // Update processed count even for failed batch
            setProgress((prev) => ({
              processed: (prev?.processed || 0) + batch.length,
              total: files.length,
            }));
          }
        }

        const successCount = files.length - totalErrors;
        if (successCount > 0) {
          showMessage("success", `Border generation complete! ${successCount}/${files.length} images processed successfully.`);
        } else {
          showMessage("error", "Border generation failed for all images.");
        }

      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : String(error || "Unknown error");
        showMessage("error", `Border generation failed: ${errorMessage}`);
        setProgress((prev) =>
          prev
            ? {
                ...prev,
                error: errorMessage,
                failedAt: prev.processed,
              }
            : null,
        );

        // Record overall failure metrics
        const totalProcessingTime = performance.now() - startTime;
        recordProcessingMetrics({
          operationType: "border",
          batchSize: files.length,
          processingTimeMs: totalProcessingTime,
          filesProcessed: 0,
          avgTimePerFile: 0,
          success: false,
          errorRate: 1.0,
        });
      } finally {
        setIsGenerating(false);
      }
    },
    [workerClient, showMessage],
  );

  const cancelGeneration = useCallback(() => {
    workerClient.abortGenerateBorders();
    setIsGenerating(false);
    setProgress(null);
    showMessage("info", "Border generation has been cancelled.");
  }, [workerClient, showMessage]);

  return {
    isGenerating,
    processedImages,
    progress,
    setProcessedImages, // Expose setter for external clearing/management
    generateBorders,
    cancelGeneration,
  };
};
