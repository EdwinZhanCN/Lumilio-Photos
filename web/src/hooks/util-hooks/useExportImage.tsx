import { useCallback, useState, useRef, useEffect } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";

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
    exportMultiple: (assets: Asset[], options: ExportOptions) => Promise<void>;
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
export const useExportImage = ():useExportImageReturn => {
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
   * Exports multiple images, downloading them one by one.
   * For true batch processing (e.g., into a ZIP), a library like JSZip would be needed.
   */
  const exportMultiple = useCallback(
    async (assets: Asset[], options: ExportOptions): Promise<void> => {
      if (assets.length === 0) {
        showMessage("info", "No images selected for export");
        return;
      }

      setIsExporting(true);
      setExportProgress({ processed: 0, total: assets.length });

      let successCount = 0;
      for (let i = 0; i < assets.length; i++) {
        const asset = assets[i];
        if (!asset.storage_path) continue;

        setExportProgress((prev) => ({
          ...prev!,
          processed: i,
          currentFile: asset.original_filename || `Image ${i + 1}`,
        }));

        try {
          // This will re-trigger the progress listener, which is not ideal.
          // For true multi-export, the worker itself should handle the loop.
          // But for this refactor, we keep the original logic.
          const result = await workerClient.exportImage(asset.storage_path, {
            ...options,
            filename: generateFilename(asset, options),
          });

          if (result.status === "complete" && result.blob) {
            downloadBlob(result.blob, result.filename!);
            successCount++;
          }
        } catch (error) {
          console.warn(`Failed to export ${asset.original_filename}:`, error);
        }
      }

      setExportProgress((prev) => ({ ...prev!, processed: assets.length }));
      showMessage(
        "success",
        `Exported ${successCount} of ${assets.length} images.`,
      );

      setIsExporting(false);
      setTimeout(() => setExportProgress(null), 3000);
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
