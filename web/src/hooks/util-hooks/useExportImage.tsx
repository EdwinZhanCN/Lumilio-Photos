import { useCallback, useState, useRef } from "react";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import { Asset } from "@/lib/assets/types";
import { assetUrls } from "@/lib/assets/assetUrls";
import { isExportSupported } from "@/lib/utils/mediaTypes";

export interface ExportOptions {
  format: "jpeg" | "png" | "webp" | "avif" | "original";
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

const EXTENSION_BY_FORMAT: Record<string, string> = {
  jpeg: "jpg",
  png: "png",
  webp: "webp",
  avif: "avif",
};

/**
 * Hook for downloading and exporting images.
 *
 * Export/transcode is handled entirely server-side: the backend (libvips)
 * re-encodes the original to the requested format/size and streams it back.
 * The browser just fetches the URL and triggers a download — no wasm, no worker.
 *
 * @author Edwin Zhan
 * @since 1.1.0
 */
export const useExportImage = (): useExportImageReturn => {
  const showMessage = useMessage();

  const [isExporting, setIsExporting] = useState(false);
  const [exportProgress, setExportProgress] = useState<ExportProgress | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  /**
   * Downloads the original image file without any processing.
   */
  const downloadOriginal = useCallback(
    async (asset: Asset): Promise<void> => {
      if (!asset.asset_id) {
        showMessage("error", "No image available for download");
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
        const response = await fetch(assetUrls.getOriginalFileUrl(asset.asset_id), {
          signal: abortControllerRef.current.signal,
          credentials: "include",
        });
        if (!response.ok) {
          throw new Error(`Failed to fetch image: ${response.statusText}`);
        }

        const blob = await response.blob();
        downloadBlob(blob, asset.original_filename || "download");
        showMessage("success", "Image downloaded successfully");
      } catch (error) {
        if (error instanceof Error && error.name === "AbortError") {
          showMessage("info", "Download cancelled");
        } else {
          showMessage("error", error instanceof Error ? error.message : "Download failed");
        }
      } finally {
        setIsExporting(false);
        setExportProgress(null);
        abortControllerRef.current = null;
      }
    },
    [showMessage],
  );

  const fetchExport = useCallback(
    async (asset: Asset, options: ExportOptions, signal: AbortSignal): Promise<void> => {
      const assetId = asset.asset_id;
      if (!assetId) throw new Error("No image available for export");

      if (options.format === "original") {
        const response = await fetch(assetUrls.getOriginalFileUrl(assetId), {
          signal,
          credentials: "include",
        });
        if (!response.ok) {
          throw new Error(`Export failed: ${response.statusText}`);
        }
        downloadBlob(await response.blob(), asset.original_filename || "download");
        return;
      }

      const base = baseFilename(asset, options);
      const url = assetUrls.getExportUrl(assetId, {
        format: options.format,
        quality: toServerQuality(options.quality),
        maxWidth: options.maxWidth,
        maxHeight: options.maxHeight,
        filename: base,
      });

      const response = await fetch(url, { signal, credentials: "include" });
      if (!response.ok) {
        const message = await safeErrorMessage(response);
        throw new Error(message);
      }
      const ext = EXTENSION_BY_FORMAT[options.format] ?? options.format;
      downloadBlob(await response.blob(), `${base}.${ext}`);
    },
    [],
  );

  /**
   * Exports a single image with the specified format/quality via the backend.
   */
  const exportImage = useCallback(
    async (asset: Asset, options: ExportOptions): Promise<void> => {
      if (!asset.asset_id) {
        showMessage("error", "No image available for export");
        return;
      }
      if (options.format !== "original" && !isExportSupported(asset)) {
        showMessage("info", "Export conversion is unavailable for video and audio assets.");
        return;
      }

      setIsExporting(true);
      setExportProgress({
        processed: 0,
        total: 100,
        currentFile: asset.original_filename || "image",
      });
      abortControllerRef.current = new AbortController();

      try {
        await fetchExport(asset, options, abortControllerRef.current.signal);
        setExportProgress((prev) => (prev ? { ...prev, processed: 100 } : null));
        showMessage("success", "Image exported successfully");
      } catch (error) {
        if (error instanceof Error && error.name === "AbortError") {
          return;
        }
        const message = error instanceof Error ? error.message : "Export failed";
        showMessage("error", message);
        setExportProgress((prev) => (prev ? { ...prev, error: message } : null));
      } finally {
        setIsExporting(false);
        abortControllerRef.current = null;
        setTimeout(() => setExportProgress(null), 3000);
      }
    },
    [fetchExport, showMessage],
  );

  /**
   * Exports multiple images sequentially via the backend.
   */
  const exportMultiple = useCallback(
    async (assets: Asset[], options: ExportOptions): Promise<void> => {
      if (assets.length === 0) {
        showMessage("info", "No images selected for export");
        return;
      }

      setIsExporting(true);
      setExportProgress({ processed: 0, total: assets.length });
      abortControllerRef.current = new AbortController();
      const signal = abortControllerRef.current.signal;

      let successCount = 0;
      try {
        for (let i = 0; i < assets.length; i++) {
          if (signal.aborted) break;
          const asset = assets[i];
          if (!asset.asset_id || (options.format !== "original" && !isExportSupported(asset))) {
            continue;
          }

          setExportProgress({
            processed: i,
            total: assets.length,
            currentFile: asset.original_filename || `Image ${i + 1}`,
          });

          try {
            await fetchExport(asset, options, signal);
            successCount++;
          } catch (error) {
            if (error instanceof Error && error.name === "AbortError") break;
            console.warn(`Failed to export ${asset.original_filename}:`, error);
          }
        }

        if (successCount > 0) {
          showMessage(
            successCount === assets.length ? "success" : "info",
            `Export completed. Successfully exported ${successCount} of ${assets.length} images.`,
          );
        } else {
          showMessage("error", "Export failed for all images.");
        }
      } finally {
        setIsExporting(false);
        abortControllerRef.current = null;
        setTimeout(() => setExportProgress(null), 3000);
      }
    },
    [fetchExport, showMessage],
  );

  /**
   * Cancels any ongoing export or download operation.
   */
  const cancelExport = useCallback(() => {
    abortControllerRef.current?.abort();
    setIsExporting(false);
    setExportProgress(null);
    showMessage("info", "Export cancelled");
  }, [showMessage]);

  return {
    isExporting,
    exportProgress,
    downloadOriginal,
    exportImage,
    exportMultiple,
    cancelExport,
  };
};

/** Converts a 0.1–1.0 UI quality into the backend's 1–100 scale. */
function toServerQuality(quality: number): number {
  if (!Number.isFinite(quality) || quality <= 0) return 0;
  const scaled = quality <= 1 ? Math.round(quality * 100) : Math.round(quality);
  return Math.max(1, Math.min(100, scaled));
}

async function safeErrorMessage(response: Response): Promise<string> {
  try {
    const data = (await response.json()) as { message?: string; error?: string };
    return data.message || data.error || `Export failed: ${response.statusText}`;
  } catch {
    return `Export failed: ${response.statusText}`;
  }
}

function baseFilename(asset: Asset, options: ExportOptions): string {
  if (options.filename) {
    return options.filename.replace(/\.[^.]+$/, "");
  }
  return asset.original_filename?.replace(/\.[^.]+$/, "") || "export";
}

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
