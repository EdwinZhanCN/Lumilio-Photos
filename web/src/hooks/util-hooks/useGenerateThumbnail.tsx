import { useState, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import {
  getOptimalBatchSize,
  recordProcessingMetrics,
  ProcessingPriority
} from "@/lib/utils/smartBatchSizing.ts";

export interface ThumbnailProgress {
  numberProcessed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
}

interface ThumbnailResult {
  index: number;
  url: string;
}

/**
 * Represents the state and actions returned by the useGenerateThumbnail hook.
 */
export interface UseGenerateThumbnailReturn {
  isGenerating: boolean;
  progress: ThumbnailProgress | null;
  generatePreviews: (files: File[], priority?: ProcessingPriority) => Promise<ThumbnailResult[] | undefined>;
  cancelGeneration: () => void;
}

/**
 * Custom hook to generate thumbnails using a Web Worker.
 * It manages its own state and uses the shared worker client.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 * @author Edwin Zhan
 * @since 1.1.1
 * @returns {UseGenerateThumbnailReturn} Hook state and actions for thumbnail generation.
 */
export const useGenerateThumbnail = (): UseGenerateThumbnailReturn => {
  const showMessage = useMessage();
  const workerClient = useWorker();

  const [isGenerating, setIsGenerating] = useState(false);
  const [progress, setProgress] = useState<ThumbnailProgress | null>(null);

  const generatePreviews = useCallback(
    async (files: File[], priority: ProcessingPriority = ProcessingPriority.CRITICAL): Promise<ThumbnailResult[] | undefined> => {
      setIsGenerating(true);
      setProgress({ numberProcessed: 0, total: files.length });

      const allGeneratedPreviews: ThumbnailResult[] = [];
      const startTime = performance.now();
      let totalErrors = 0;

      try {
        // Get optimal batch size - thumbnails are critical for user experience
        const batchSize = getOptimalBatchSize("thumbnail", files.length, priority);
        
        for (let i = 0; i < files.length; i += batchSize) {
          const batch = files.slice(i, i + batchSize);
          const batchStartTime = performance.now();
          
          try {
            const result = await workerClient.generateThumbnail({
              files: batch,
              batchIndex: Math.floor(i / batchSize),
              startIndex: i,
            });

            if (result.status === "complete" && result.results) {
              allGeneratedPreviews.push(...result.results);
              // Correctly update progress after each batch completes successfully.
              // The progress reflects the total number of previews generated so far.
              setProgress((prev) => ({
                ...prev!,
                numberProcessed: allGeneratedPreviews.length,
              }));

              // Record successful batch metrics
              const batchProcessingTime = performance.now() - batchStartTime;
              recordProcessingMetrics({
                operationType: "thumbnail",
                batchSize: batch.length,
                processingTimeMs: batchProcessingTime,
                filesProcessed: result.results.length,
                avgTimePerFile: batchProcessingTime / result.results.length,
                success: true,
                errorRate: 0,
              });
            } else {
              totalErrors += batch.length;
              const batchProcessingTime = performance.now() - batchStartTime;
              recordProcessingMetrics({
                operationType: "thumbnail",
                batchSize: batch.length,
                processingTimeMs: batchProcessingTime,
                filesProcessed: 0,
                avgTimePerFile: 0,
                success: false,
                errorRate: 1.0,
              });
            }
          } catch (batchError) {
            totalErrors += batch.length;
            console.warn(`Thumbnail generation batch ${Math.floor(i / batchSize) + 1} failed:`, batchError);
            
            const batchProcessingTime = performance.now() - batchStartTime;
            recordProcessingMetrics({
              operationType: "thumbnail",
              batchSize: batch.length,
              processingTimeMs: batchProcessingTime,
              filesProcessed: 0,
              avgTimePerFile: 0,
              success: false,
              errorRate: 1.0,
            });
          }
        }

        const successCount = allGeneratedPreviews.length;
        if (successCount > 0) {
          showMessage("success", `Thumbnail generation complete. Generated ${successCount}/${files.length} thumbnails.`);
        } else {
          showMessage("error", "Thumbnail generation failed for all images.");
        }
        
        return allGeneratedPreviews;
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : String(error || "Unknown error");
        showMessage("error", `Thumbnail generation failed: ${errorMessage}`);
        setProgress((prev) =>
          prev
            ? {
                ...prev,
                error: errorMessage,
                failedAt: Date.now(),
              }
            : null,
        );

        // Record overall failure metrics
        const totalProcessingTime = performance.now() - startTime;
        recordProcessingMetrics({
          operationType: "thumbnail",
          batchSize: files.length,
          processingTimeMs: totalProcessingTime,
          filesProcessed: allGeneratedPreviews.length,
          avgTimePerFile: allGeneratedPreviews.length > 0 ? totalProcessingTime / allGeneratedPreviews.length : 0,
          success: false,
          errorRate: totalErrors / files.length,
        });

        return undefined; // Indicate failure
      } finally {
        setIsGenerating(false);
        // Do not reset progress here to show final state. It will be reset on the next run.
      }
    },
    [workerClient, showMessage],
  );

  const cancelGeneration = useCallback(() => {
    workerClient.abortGenerateThumbnail();
    setIsGenerating(false);
    setProgress(null);
    showMessage("info", "Thumbnail generation cancelled.");
  }, [workerClient, showMessage]);

  return { isGenerating, progress, generatePreviews, cancelGeneration };
};
