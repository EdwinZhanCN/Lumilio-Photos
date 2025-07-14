import React, { useState, useEffect, useCallback } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { WasmWorkerClient } from "@/workers/workerClient.ts";

const BATCH_SIZE = 5; // [TODO] Example batch size, consider making this configurable

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
  // [TODO] Add the display text: Camera manufacture logo, Lens Info, ISO, Shutter Speed, and Aperture
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

export type BorderGenerationProgress = {
  numberProcessed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

type UseGenerateBordersProps = {
  workerClientRef: React.RefObject<WasmWorkerClient | null>;
  wasmReady: boolean;
};

/**
 * Custom hook to generate images with borders using a Web Worker and WebAssembly.
 * It encapsulates all state related to the generation process.
 * @author Edwin Zhan
 * @param {UseGenerateBordersProps} props - Props for the hook.
 * @returns {boolean}isGenerating
 *
 */
export const useGenerateBorders = ({
  workerClientRef,
  wasmReady,
}: UseGenerateBordersProps) => {
  const showMessage = useMessage();

  // Internal state management
  const [isGenerating, setIsGenerating] = useState(false);
  const [progress, setProgress] = useState<BorderGenerationProgress>(null);
  const [processedImages, setProcessedImages] = useState<ProcessedImageMap>({});

  // Effect for progress listener
  useEffect(() => {
    if (!workerClientRef.current) return;

    const removeProgressListener = workerClientRef.current.addProgressListener(
      ({ processed, total }) => {
        setProgress((prev) =>
          prev
            ? { ...prev, numberProcessed: processed, total: total }
            : { numberProcessed: processed, total: total },
        );
      },
    );

    return () => removeProgressListener();
  }, [workerClientRef]);

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

      try {
        setIsGenerating(true);
        setProcessedImages({}); // Clear previous results
        setProgress({ numberProcessed: 0, total: files.length });

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
        setProgress((prev) =>
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
        setIsGenerating(false);
        // We don't nullify progress here so the UI can show the final state
      }
    },
    [wasmReady, workerClientRef, showMessage],
  );

  // Expose state and the trigger function
  return {
    isGenerating,
    progress,
    processedImages,
    setProcessedImages,
    generateBorders,
  };
};
