import { useState, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";

const BATCH_SIZE = 10; // This can be moved to a config file later

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
  generatePreviews: (files: File[]) => Promise<ThumbnailResult[] | undefined>;
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
    async (files: File[]): Promise<ThumbnailResult[] | undefined> => {
      setIsGenerating(true);
      setProgress({ numberProcessed: 0, total: files.length });

      const allGeneratedPreviews: ThumbnailResult[] = [];

      try {
        for (let i = 0; i < files.length; i += BATCH_SIZE) {
          const batch = files.slice(i, i + BATCH_SIZE);
          const result = await workerClient.generateThumbnail({
            files: batch,
            batchIndex: i / BATCH_SIZE,
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
          }
        }
        showMessage("success", "Thumbnail generation complete.");
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
