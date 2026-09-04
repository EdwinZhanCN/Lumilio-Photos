import { useCallback, useRef, useState } from "react";
import { useMessage } from "@/features/notifications";
import type { Asset } from "@/lib/assets/types";
import { assetUrls } from "@/lib/assets/assetUrls";
import { isExportSupported } from "../model/mediaTypes";
import { useI18n } from "@/lib/i18n";
import {
  isAbortProblem,
  localizeAPIProblem,
  normalizeProblem,
  readProblemResponse,
} from "@/lib/http-commons/problem";

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
  const { t } = useI18n();

  const [isExporting, setIsExporting] = useState(false);
  const [exportProgress, setExportProgress] = useState<ExportProgress | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  /**
   * Downloads the original image file without any processing.
   */
  const downloadOriginal = useCallback(
    async (asset: Asset): Promise<void> => {
      if (!asset.asset_id) {
        showMessage("error", t("assets.export.noDownload", "No image is available for download."));
        return;
      }

      setIsExporting(true);
      setExportProgress({
        processed: 0,
        total: 1,
        currentFile: t("assets.export.downloadingOriginal", "Downloading original…"),
      });
      abortControllerRef.current = new AbortController();

      try {
        const response = await fetch(assetUrls.getOriginalFileUrl(asset.asset_id), {
          signal: abortControllerRef.current.signal,
          credentials: "include",
        });
        if (!response.ok) {
          throw await readProblemResponse(response);
        }

        const blob = await response.blob();
        downloadBlob(blob, asset.original_filename || "download");
        showMessage("success", t("assets.export.downloaded", "Image downloaded."));
      } catch (error) {
        if (isAbortProblem(error)) {
          showMessage("info", t("assets.export.downloadCancelled", "Download cancelled."));
        } else {
          showMessage(
            "error",
            localizeAPIProblem(error, t, t("assets.export.downloadFailed", "Download failed.")),
          );
        }
      } finally {
        setIsExporting(false);
        setExportProgress(null);
        abortControllerRef.current = null;
      }
    },
    [showMessage, t],
  );

  const fetchExport = useCallback(
    async (asset: Asset, options: ExportOptions, signal: AbortSignal): Promise<void> => {
      const assetId = asset.asset_id;
      if (!assetId) throw normalizeProblem(undefined);

      if (options.format === "original") {
        const response = await fetch(assetUrls.getOriginalFileUrl(assetId), {
          signal,
          credentials: "include",
        });
        if (!response.ok) {
          throw await readProblemResponse(response);
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
        throw await readProblemResponse(response);
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
        showMessage("error", t("assets.export.noExport", "No image is available for export."));
        return;
      }
      if (options.format !== "original" && !isExportSupported(asset)) {
        showMessage(
          "info",
          t(
            "assets.export.conversionUnavailable",
            "Export conversion is unavailable for video and audio assets.",
          ),
        );
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
        showMessage("success", t("assets.export.exported", "Image exported."));
      } catch (error) {
        if (isAbortProblem(error)) {
          return;
        }
        const message = localizeAPIProblem(error, t, t("assets.export.failed", "Export failed."));
        showMessage("error", message);
        setExportProgress((prev) => (prev ? { ...prev, error: message } : null));
      } finally {
        setIsExporting(false);
        abortControllerRef.current = null;
        setTimeout(() => setExportProgress(null), 3000);
      }
    },
    [fetchExport, showMessage, t],
  );

  /**
   * Exports multiple images sequentially via the backend.
   */
  const exportMultiple = useCallback(
    async (assets: Asset[], options: ExportOptions): Promise<void> => {
      if (assets.length === 0) {
        showMessage("info", t("assets.export.noneSelected", "No images are selected for export."));
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
            if (isAbortProblem(error)) break;
            console.warn(`Failed to export ${asset.original_filename}:`, error);
          }
        }

        if (successCount > 0) {
          showMessage(
            successCount === assets.length ? "success" : "info",
            t("assets.export.multipleCompleted", "Exported {{count}} of {{total}} images.", {
              count: successCount,
              total: assets.length,
            }),
          );
        } else {
          showMessage("error", t("assets.export.allFailed", "No images could be exported."));
        }
      } finally {
        setIsExporting(false);
        abortControllerRef.current = null;
        setTimeout(() => setExportProgress(null), 3000);
      }
    },
    [fetchExport, showMessage, t],
  );

  /**
   * Cancels any ongoing export or download operation.
   */
  const cancelExport = useCallback(() => {
    abortControllerRef.current?.abort();
    setIsExporting(false);
    setExportProgress(null);
    showMessage("info", t("assets.export.cancelled", "Export cancelled."));
  }, [showMessage, t]);

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
