import {
  BookPlus,
  CloudDownload,
  ImageDown,
  Paintbrush,
  RefreshCw,
  Share2,
  SquareArrowOutUpRight,
  X,
} from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { useExportImage, type ExportOptions } from "../../api/useExportImage";
import { isExportSupported } from "../../model/mediaTypes";
import { RetryProcessingDialog } from "./RetryProcessingDialog";
import { useAssetReprocess } from "./useAssetReprocess";

type ExportFormat = "png" | "jpeg" | "webp" | "avif";

interface AssetExportDialogProps {
  asset?: Asset;
  onDownloadOriginal?: (asset: Asset) => void | Promise<void>;
  onOpenOriginalInNewTab?: (asset: Asset) => void | Promise<void>;
  onExport?: (asset: Asset, options: ExportOptions) => void | Promise<void>;
  onOpenStudio?: (asset: Asset) => void;
  onAddToAlbum?: (asset: Asset) => void | Promise<void>;
  onShare?: (asset: Asset) => void;
}

export function AssetExportDialog({
  asset,
  onDownloadOriginal,
  onOpenOriginalInNewTab,
  onExport,
  onOpenStudio,
  onAddToAlbum,
  onShare,
}: AssetExportDialogProps) {
  const { t } = useI18n();
  const { isExporting, exportImage, downloadOriginal } = useExportImage();
  const [format, setFormat] = useState<ExportFormat>("png");
  const reprocess = useAssetReprocess(asset);
  const originalUrl = useMemo(
    () => (asset?.asset_id ? assetUrls.getOriginalFileUrl(asset.asset_id) : ""),
    [asset?.asset_id],
  );
  const canAct = Boolean(asset) && !isExporting;
  const canExport = Boolean(asset && isExportSupported(asset));

  const exportOptions = useMemo<ExportOptions>(() => {
    if (format === "jpeg" || format === "webp") return { format, quality: 0.8 };
    if (format === "avif") return { format, quality: 0.55 };
    return { format: "png", quality: 1 };
  }, [format]);

  const handleDownloadOriginal = useCallback(async () => {
    if (!asset) return;
    if (onDownloadOriginal) await onDownloadOriginal(asset);
    else await downloadOriginal(asset);
  }, [asset, downloadOriginal, onDownloadOriginal]);

  const handleOpenOriginal = useCallback(async () => {
    if (!asset) return;
    if (onOpenOriginalInNewTab) await onOpenOriginalInNewTab(asset);
    else if (originalUrl) window.open(originalUrl, "_blank", "noopener,noreferrer");
  }, [asset, onOpenOriginalInNewTab, originalUrl]);

  const handleShare = useCallback(() => {
    if (!asset || !onShare) return;
    document.querySelector<HTMLDialogElement>("#asset_export_dialog")?.close();
    onShare(asset);
  }, [asset, onShare]);

  const handleExport = useCallback(async () => {
    if (!asset || !canExport) return;
    if (onExport) await onExport(asset, exportOptions);
    else await exportImage(asset, exportOptions);
    document.querySelector<HTMLDialogElement>("#asset_export_dialog")?.close();
  }, [asset, canExport, exportImage, exportOptions, onExport]);

  return (
    <>
      <dialog id="asset_export_dialog" className="modal">
        <div className="modal-box">
          <form method="dialog">
            <button
              className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2"
              aria-label={t("common.close")}
            >
              <X />
            </button>
          </form>

          <div className="mb-2 flex flex-wrap gap-2">
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.studio", { defaultValue: "Studio" })}
            >
              <button
                type="button"
                className="btn btn-soft btn-circle"
                disabled={!asset || !onOpenStudio}
                onClick={() => asset && onOpenStudio?.(asset)}
              >
                <Paintbrush className="size-6" />
              </button>
            </div>
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.addToAlbum", { defaultValue: "Add to album" })}
            >
              <button
                type="button"
                className="btn btn-soft btn-circle"
                disabled={!asset || !onAddToAlbum}
                onClick={() => asset && void onAddToAlbum?.(asset)}
              >
                <BookPlus />
              </button>
            </div>
            {onShare && (
              <div
                className="tooltip tooltip-bottom"
                data-tip={t("exportModal.share", { defaultValue: "Share" })}
              >
                <button
                  type="button"
                  className="btn btn-soft btn-circle"
                  onClick={handleShare}
                  disabled={!canAct}
                >
                  <Share2 />
                </button>
              </div>
            )}
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.downloadOriginal", {
                defaultValue: "Download Original",
              })}
            >
              <button
                type="button"
                className="btn btn-soft btn-circle"
                onClick={() => void handleDownloadOriginal()}
                disabled={!canAct}
              >
                <CloudDownload />
              </button>
            </div>
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.viewOriginal", {
                defaultValue: "View Original in New Tab",
              })}
            >
              <button
                type="button"
                className="btn btn-soft btn-circle"
                onClick={() => void handleOpenOriginal()}
                disabled={!originalUrl || isExporting}
              >
                <SquareArrowOutUpRight />
              </button>
            </div>
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.retryProcessing", {
                defaultValue: "Retry Processing",
              })}
            >
              <button
                type="button"
                className="btn btn-soft btn-circle"
                disabled={!asset}
                onClick={() =>
                  document.querySelector<HTMLDialogElement>("#asset_retry_dialog")?.showModal()
                }
              >
                <RefreshCw className="size-5" />
              </button>
            </div>
          </div>

          {canExport && (
            <fieldset className="fieldset">
              <legend className="fieldset-legend">
                {t("exportModal.exportFormat", { defaultValue: "Export Format" })}
              </legend>
              <select
                className="select mb-2"
                value={format}
                onChange={(event) => setFormat(event.target.value as ExportFormat)}
                disabled={!canAct}
              >
                <option value="png">{t("exportModal.format.png", { defaultValue: "PNG" })}</option>
                <option value="jpeg">
                  {t("exportModal.format.jpeg", { defaultValue: "JPEG (80%)" })}
                </option>
                <option value="webp">
                  {t("exportModal.format.webp", { defaultValue: "WebP (80%)" })}
                </option>
                <option value="avif">
                  {t("exportModal.format.avif", { defaultValue: "AVIF" })}
                </option>
              </select>
              <span className="label">
                {t("exportModal.optional", { defaultValue: "Optional" })}
              </span>
              <button
                type="button"
                className="btn btn-soft btn-primary"
                onClick={() => void handleExport()}
                disabled={!canAct}
              >
                {isExporting ? (
                  <>
                    <span className="loading loading-spinner loading-xs" />
                    {t("exportModal.exporting", { defaultValue: "Exporting..." })}
                  </>
                ) : (
                  <>
                    <ImageDown /> {t("exportModal.export", { defaultValue: "Export" })}
                  </>
                )}
              </button>
            </fieldset>
          )}

          {asset && !canExport && (
            <p className="mt-3 text-sm opacity-70">
              {t("exportModal.exportUnavailable", {
                defaultValue:
                  "Export conversion is unavailable for video and audio assets. You can still download the original file.",
              })}
            </p>
          )}
          {!asset && (
            <p className="mt-3 text-xs opacity-70">
              {t("exportModal.noAsset", {
                defaultValue: "No asset selected. Actions are disabled.",
              })}
            </p>
          )}
        </div>
      </dialog>
      <RetryProcessingDialog controller={reprocess} />
    </>
  );
}
