import React, { useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { WasmWorkerClient } from "@/workers/workerClient.ts";

const BATCH_SIZE = 5; // Example batch size, consider making this configurable

export type BorderOptions = "COLORED" | "FROSTED" | "VIGNETTE";

export type BorderParams = {
  // Define specific parameter types for each border option
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

type UseGenerateBordersProps = {
  setGenBordersProgress: React.Dispatch<
    React.SetStateAction<{
      numberProcessed: number;
      total: number;
      error?: string;
      failedAt?: number | null;
    } | null>
  >;
  setIsGenBorders: (isGenerating: boolean) => void;
  workerClientRef: React.RefObject<WasmWorkerClient | null>;
  wasmReady: boolean;
  setProcessedImages: React.Dispatch<
    React.SetStateAction<{
      [uuid: string]: {
        originalFileName: string;
        borderedFileURL?: string;
        error?: string;
      };
    }>
  >;
};

/**
 * Custom hook to generate images with borders using a Web Worker.
 *
 * @param {UseGenerateBordersProps} options - Configuration options
 * @returns {{ generateBorders: (files: File[], option: BorderOptions, param: BorderParams[BorderOptions]) => Promise<void> }} Object containing the generateBorders function
 */
export const useGenerateBorders = ({
  setGenBordersProgress,
  setIsGenBorders,
  workerClientRef,
  wasmReady,
  setProcessedImages,
}: UseGenerateBordersProps): {
  generateBorders: (
    files: File[],
    option: BorderOptions,
    param: BorderParams[BorderOptions],
  ) => Promise<void>;
} => {
  const showMessage = useMessage();

  /**
   * Generates bordered images for the given files.
   * @param {File[]} files - The files for which to generate borders.
   * @param {BorderOptions} option - The type of border to apply.
   * @param {BorderParams[BorderOptions]} param - The parameters for the selected border type.
   */
  const generateBorders = useCallback(
    async (
      files: File[],
      option: BorderOptions,
      param: BorderParams[BorderOptions],
    ): Promise<void> => {
      if (!workerClientRef.current || !wasmReady) {
        showMessage("error", "WebAssembly module is not ready yet");
        return;
      }

      const removeProgressListener =
        workerClientRef.current.addProgressListener(({ processed, total }) => {
          setGenBordersProgress({ numberProcessed: processed, total: total });
        });

      try {
        setIsGenBorders(true);
        setProcessedImages({}); // Clear previous results

        for (let i = 0; i < files.length; i += BATCH_SIZE) {
          const batch = files.slice(i, i + BATCH_SIZE);
          const resultsMap = await workerClientRef.current.generateBorders(
            batch,
            option,
            param,
          );

          setProcessedImages((prev) => ({
            ...prev,
            ...resultsMap,
          }));
        }
        showMessage("success", "Border generation complete!");
      } catch (error) {
        const errorMessage =
          error instanceof Error
            ? error.message
            : String(error || "Unknown error");
        showMessage("error", `Border generation failed: ${errorMessage}`);
        setGenBordersProgress((prev) =>
          prev
            ? {
                ...prev,
                error: errorMessage,
                failedAt: Date.now(),
              }
            : {
                numberProcessed: 0,
                total: files.length,
                error: errorMessage,
                failedAt: Date.now(),
              },
        );
      } finally {
        setIsGenBorders(false);
        removeProgressListener();
        setGenBordersProgress(null);
      }
    },
    [
      wasmReady,
      workerClientRef,
      setProcessedImages,
      setIsGenBorders,
      setGenBordersProgress,
      showMessage,
    ],
  );

  return {
    generateBorders,
  };
};
