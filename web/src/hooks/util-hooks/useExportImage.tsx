import { useCallback, useState, useRef, useEffect } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import {
  getOptimalBatchSize,
  recordProcessingMetrics,
  ProcessingPriority,
} from "@/utils/smartBatchSizing";
import { Asset } from "@/lib/assets/types";

export interface ExportOptions {
  format: "jpeg" | "png" | "webp" | "original";
  quality: number; // 0.1 to 1.0 for lossy formats
  maxWidth?: number;
  maxHeight?: number;
  filename?: string;
}

export interface ExportProgress {
  processed: number; // Percentage for single export, count for multiple
  total: number;
  currentFile?: string;
  error?: string;
}

export interface useExportImageReturn {
  isExporting: boolean;
  exportProgress: ExportProgress | null;
  downloadOriginal: (asset: Asset) => Promise<void>;
  exportImage: (asset: Asset, options: ExportOptions) => Promise<void>;
  exportMultiple: (
    assets: Asset[],
    options: ExportOptions,
    priority?: ProcessingPriority,
  ) => Promise<void>;
  cancelExport: () => void;
}

/**
 * Custom hook for downloading and exporting images.
 * It uses the shared worker client for format conversion and processing.
 * This hook must be used within a component tree wrapped by `<WorkerProvider />`.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 * @returns {useExportImageReturn} Hook state and actions for image export.
 */
export const useExportImage = (): useExportImageReturn => {
  const workerClient = useWorker();
  const showMessage = useMessage();

  const [isExporting, setIsExporting] = useState(false);
  const [exportProgress, setExportProgress] = useState<ExportProgress | null>(
    null,
  );
  const abortControllerRef = useRef<AbortController | null>(null);

  // Effect to set up the progress listener
  useEffect(() => {
    const removeProgressListener = workerClient.addProgressListener(
      (detail) => {
        if (isExporting && detail && typeof detail.processed === "number") {
          setExportProgress((prev) =>
            prev ? { ...prev, processed: detail.processed } : null,
          );
        }
      },
    );
    return () => removeProgressListener();
  }, [workerClient, isExporting]);

  /**
   * Downloads the original image file without any processing.
   */
  const downloadOriginal = useCallback(
    async (asset: Asset): Promise<void> => {
      if (!asset.storage_path) {
        showMessage("error", "No image path available for download");
        return;
      }

      setIsExporting(true);
      setExportProgress({
        processed: 0,
        total: 1,
        currentFile: "Downloading original...",
      });
      abortControllerRef.current = new AbortController();

      try {
        const response = await fetch(asset.storage_path, {
          signal: abortControllerRef.current.signal,
        });

        if (!response.ok) {
          throw new Error(`Failed to fetch image: ${response.statusText}`);
        }

        const blob = await response.blob();
        const filename = asset.original_filename || "download";

        downloadBlob(blob, filename);
        showMessage("success", "Image downloaded successfully");
      } catch (error) {
        if (error instanceof Error && error.name === "AbortError") {
          showMessage("info", "Download cancelled");
        } else {
          showMessage(
            "error",
            error instanceof Error ? error.message : "Download failed",
          );
        }
      } finally {
        setIsExporting(false);
        setExportProgress(null);
        abortControllerRef.current = null;
      }
    },
    [showMessage],
  );

  /**
   * Exports a single image with specified format and quality options using WASM.
   */
  const exportImage = useCallback(
    async (asset: Asset, options: ExportOptions): Promise<void> => {
      if (!asset.storage_path) {
        showMessage("error", "No image path available for export");
        return;
      }

      setIsExporting(true);
      setExportProgress({
        processed: 0,
        total: 100, // For single image, this is percentage
        currentFile: asset.original_filename || "image",
      });

      try {
        const result = await workerClient.exportImage(
          asset.storage_path,
          options,
        );

        if (result.status === "complete" && result.blob) {
          const filename = result.filename || generateFilename(asset, options);
          downloadBlob(result.blob, filename);
          showMessage("success", "Image exported successfully");
        } else {
          throw new Error(result.error || "Export failed");
        }
      } catch (error) {
        const message =
          error instanceof Error ? error.message : "Export failed";
        showMessage("error", message);
        setExportProgress((prev) =>
          prev ? { ...prev, error: message } : null,
        );
      } finally {
        setIsExporting(false);
        setTimeout(() => setExportProgress(null), 3000); // Keep final state for a bit
      }
    },
    [workerClient, showMessage],
  );

  /**
   * Exports multiple images using smart batch sizing for optimal performance.
   */
  const exportMultiple = useCallback(
    async (
      assets: Asset[],
      options: ExportOptions,
      priority: ProcessingPriority = ProcessingPriority.HIGH,
    ): Promise<void> => {
      if (assets.length === 0) {
        showMessage("info", "No images selected for export");
        return;
      }

      setIsExporting(true);
      setExportProgress({ processed: 0, total: assets.length });

      const startTime = performance.now();
      let successCount = 0;
      let totalErrors = 0;

      try {
        // Get optimal batch size for export operations
        const batchSize = getOptimalBatchSize(
          "export",
          assets.length,
          priority,
        );

        // Process in smart batches
        for (let i = 0; i < assets.length; i += batchSize) {
          const batch = assets.slice(i, i + batchSize);
          const batchStartTime = performance.now();
          let batchSuccessCount = 0;
          let batchErrors = 0;

          // Process batch sequentially to avoid overwhelming the system
          for (let j = 0; j < batch.length; j++) {
            const asset = batch[j];
            const globalIndex = i + j;

            if (!asset.storage_path) {
              batchErrors++;
              continue;
            }

            setExportProgress((prev) => ({
              ...prev!,
              processed: globalIndex,
              currentFile:
                asset.original_filename || `Image ${globalIndex + 1}`,
            }));

            try {
              const result = await workerClient.exportImage(
                asset.storage_path,
                {
                  ...options,
                  filename: generateFilename(asset, options),
                },
              );

              if (result.status === "complete" && result.blob) {
                downloadBlob(result.blob, result.filename!);
                batchSuccessCount++;
                successCount++;
              } else {
                batchErrors++;
                totalErrors++;
              }
            } catch (error) {
              console.warn(
                `Failed to export ${asset.original_filename}:`,
                error,
              );
              batchErrors++;
              totalErrors++;
            }
          }

          // Record batch metrics
          const batchProcessingTime = performance.now() - batchStartTime;
          recordProcessingMetrics({
            operationType: "export",
            batchSize: batch.length,
            processingTimeMs: batchProcessingTime,
            filesProcessed: batchSuccessCount,
            avgTimePerFile:
              batchSuccessCount > 0
                ? batchProcessingTime / batchSuccessCount
                : 0,
            success: batchErrors === 0,
            errorRate: batchErrors / batch.length,
          });

          // Update progress after batch completion
          setExportProgress((prev) => ({
            ...prev!,
            processed: Math.min(i + batchSize, assets.length),
          }));
        }

        // Show final results
        if (successCount > 0) {
          showMessage(
            successCount === assets.length ? "success" : "info",
            `Export completed. Successfully exported ${successCount} of ${assets.length} images.`,
          );
        } else {
          showMessage("error", "Export failed for all images.");
        }
      } catch (error) {
        const errorMessage =
          error instanceof Error ? error.message : "Export failed";
        showMessage("error", errorMessage);
        setExportProgress((prev) =>
          prev ? { ...prev, error: errorMessage } : null,
        );

        // Record overall failure
        const totalProcessingTime = performance.now() - startTime;
        recordProcessingMetrics({
          operationType: "export",
          batchSize: assets.length,
          processingTimeMs: totalProcessingTime,
          filesProcessed: successCount,
          avgTimePerFile:
            successCount > 0 ? totalProcessingTime / successCount : 0,
          success: false,
          errorRate: totalErrors / assets.length,
        });
      } finally {
        setIsExporting(false);
        setTimeout(() => setExportProgress(null), 3000);
      }
    },
    [workerClient, showMessage],
  );

  /**
   * Cancels any ongoing export or download operation.
   */
  const cancelExport = useCallback(() => {
    abortControllerRef.current?.abort(); // Aborts direct fetch
    workerClient.abortExportImage(); // Aborts worker task
    setIsExporting(false);
    setExportProgress(null);
    showMessage("info", "Export cancelled");
  }, [workerClient, showMessage]);

  return {
    isExporting,
    exportProgress,
    downloadOriginal,
    exportImage,
    exportMultiple,
    cancelExport,
  };
};

/**
 * Utility function to trigger a file download in the browser.
 */
function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

/**
 * Generates a descriptive filename based on asset and export options.
 */
function generateFilename(asset: Asset, options: ExportOptions): string {
  if (options.filename) return options.filename;
  const baseName =
    asset.original_filename?.split(".").slice(0, -1).join(".") || "export";
  const extension = getFileExtension(asset, options);
  return `${baseName}.${extension}`;
}

/**
 * Gets the correct file extension for a given export format.
 */
function getFileExtension(asset: Asset, options: ExportOptions): string {
  if (options.format === "original") {
    return asset.original_filename?.split(".").pop() || "jpg";
  }
  return options.format;
}
