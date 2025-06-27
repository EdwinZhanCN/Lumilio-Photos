import React, { useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { WasmWorkerClient } from "@/workers/workerClient.ts";

const BATCH_SIZE = 10; // [TODO] add this in system config

type UseGenerateThumbnailProps = {
  setGenThumbnailProgress: React.Dispatch<
    React.SetStateAction<{
      numberProcessed: number;
      total: number;
      error?: string;
      failedAt?: number | null;
    } | null>
  >;
  setIsGenThumbnails: (isGenerating: boolean) => void;
  workerClientRef: React.RefObject<WasmWorkerClient | null>;
  wasmReady: boolean;
  //  setPreviews has been removed from here
};

/**
 * Custom hook to generate thumbnails using a Web Worker.
 *
 * @author Edwin Zhan
 * @param {UseGenerateThumbnailProps} options - Configuration options
 * @returns {{ generatePreviews: (files: File[]) => Promise<void | { index: number; url: string }[]> }} Object containing the generatePreviews function
 */
export const useGenerateThumbnail = ({
  setGenThumbnailProgress,
  setIsGenThumbnails,
  workerClientRef,
  wasmReady,
  // setPreviews // This is no longer needed
}: UseGenerateThumbnailProps): {
  generatePreviews: (
    files: File[],
  ) => Promise<{ index: number; url: string }[] | Error>;
} => {
  const showMessage = useMessage(); /**
   * Generates thumbnails for the given files.
   * @param {File[]} files - The files for which thumbnails need to be generated.
   */

  const generatePreviews = useCallback(
    async (
      files: File[],
    ): Promise<{ index: number; url: string }[] | Error> => {
      if (!workerClientRef.current || !wasmReady) {
        showMessage("error", "WebAssembly module is not ready yet");
        return new Error("WebAssembly module is not ready yet");
      }

      const removeProgressListener =
        workerClientRef.current.addProgressListener(({ processed }) => {
          setGenThumbnailProgress(
            (
              prev: {
                numberProcessed: number;
                total: number;
                error?: string;
                failedAt?: number | null;
              } | null,
            ) =>
              prev === null
                ? { numberProcessed: processed, total: files.length }
                : { ...prev, numberProcessed: processed, total: files.length },
          );
        });

      // This array will hold all the results from the batches.
      const allGeneratedPreviews: { index: number; url: string }[] = [];

      try {
        setIsGenThumbnails(true);
        const startIndex = 0;

        for (let i = 0; i < files.length; i += BATCH_SIZE) {
          const batch = files.slice(i, i + BATCH_SIZE);
          const result = await workerClientRef.current.generateThumbnail({
            files: batch,
            batchIndex: i / BATCH_SIZE,
            startIndex: startIndex + i,
          });

          if (result.status === "complete" && result.results) {
            // Instead of calling setPreviews, we accumulate the results.
            allGeneratedPreviews.push(...result.results);
          }
        }
        // After the loop, return all the accumulated results.
        return allGeneratedPreviews;
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : String(error || "Unknown error");
        showMessage("error", `Thumbnail generation failed: ${errorMessage}`);
        setGenThumbnailProgress((prev) =>
          prev
            ? {
                ...prev,
                error: errorMessage,
                failedAt: Date.now(),
              }
            : null,
        );
        // Return the error so the calling component can handle it.
        return error instanceof Error ? error : new Error(errorMessage);
      } finally {
        setIsGenThumbnails(false);
        removeProgressListener();
        setGenThumbnailProgress(null);
      }
    },
    [
      wasmReady,
      workerClientRef,
      setIsGenThumbnails,
      setGenThumbnailProgress,
      showMessage,
    ],
  );

  return {
    generatePreviews,
  };
};
