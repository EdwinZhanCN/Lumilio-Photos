import {
  BookPlus,
  CloudDownload,
  ImageDown,
  Images,
  Paintbrush,
  RefreshCw,
  Share2,
  SquareArrowOutUpRight,
  X,
} from "lucide-react";
import { useCallback, useMemo, useState, useRef, useEffect } from "react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { useMessage } from "@/features/notifications";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { downloadPhotoBlob } from "@/lib/photo-export/download";
import { downloadOriginalPhoto } from "../../api/exportPhoto";
import { isExportSupported } from "../../model/mediaTypes";
import { RetryProcessingDialog } from "../export/RetryProcessingDialog";
import { useAssetReprocess } from "../export/useAssetReprocess";

interface AssetActionsDialogProps {
  asset?: Asset;
  onExport: (asset: Asset) => void;
  onOpenStudio?: (asset: Asset) => void;
  onAddToAlbum?: (asset: Asset) => void | Promise<void>;
  onFindSimilar?: (asset: Asset) => void;
  onShare?: (asset: Asset) => void;
}

export function AssetActionsDialog({
  asset,
  onExport,
  onOpenStudio,
  onAddToAlbum,
  onFindSimilar,
  onShare,
}: AssetActionsDialogProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [isDownloading, setIsDownloading] = useState(false);
  const downloadController = useRef<AbortController | null>(null);
  useEffect(() => () => downloadController.current?.abort(), []);
  const reprocess = useAssetReprocess(asset);
  const originalUrl = useMemo(
    () => (asset?.asset_id ? assetUrls.getOriginalFileUrl(asset.asset_id) : ""),
    [asset?.asset_id],
  );
  const canAct = Boolean(asset) && !isDownloading;
  const canExport = Boolean(asset && isExportSupported(asset));

  const handleDownloadOriginal = useCallback(async () => {
    if (!asset || downloadController.current) return;
    const controller = new AbortController();
    downloadController.current = controller;
    setIsDownloading(true);
    try {
      const blob = await downloadOriginalPhoto(asset, controller.signal);
      if (!controller.signal.aborted)
        downloadPhotoBlob(blob, asset.original_filename || "download");
    } catch (error) {
      if (!controller.signal.aborted)
        showMessage(
          "error",
          localizeAPIProblem(error, t, t("assets.export.downloadFailed", "Download failed.")),
        );
    } finally {
      downloadController.current = null;
      setIsDownloading(false);
    }
  }, [asset, showMessage, t]);

  const handleOpenOriginal = useCallback(async () => {
    if (!asset) return;
    if (originalUrl) window.open(originalUrl, "_blank", "noopener,noreferrer");
  }, [asset, originalUrl]);

  const handleFindSimilar = useCallback(() => {
    if (!asset || !onFindSimilar) return;
    document.querySelector<HTMLDialogElement>("#asset_actions_dialog")?.close();
    onFindSimilar(asset);
  }, [asset, onFindSimilar]);

  const handleShare = useCallback(() => {
    if (!asset || !onShare) return;
    document.querySelector<HTMLDialogElement>("#asset_actions_dialog")?.close();
    onShare(asset);
  }, [asset, onShare]);

  const handleExport = () => {
    if (!asset || !canExport) return;
    document.querySelector<HTMLDialogElement>("#asset_actions_dialog")?.close();
    onExport(asset);
  };

  return (
    <>
      <dialog id="asset_actions_dialog" className="modal">
        <div className="modal-box">
          <h3 className="font-semibold mb-4">{t("photoExport.actions", "Photo actions")}</h3>
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
                aria-label={t("exportModal.studio", { defaultValue: "Studio" })}
                type="button"
                className="btn btn-soft btn-circle"
                disabled={!canExport || !onOpenStudio}
                onClick={() => {
                  if (asset) {
                    document.querySelector<HTMLDialogElement>("#asset_actions_dialog")?.close();
                    onOpenStudio?.(asset);
                  }
                }}
              >
                <Paintbrush className="size-6" />
              </button>
            </div>
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.addToAlbum", { defaultValue: "Add to album" })}
            >
              <button
                aria-label={t("exportModal.addToAlbum", { defaultValue: "Add to album" })}
                type="button"
                className="btn btn-soft btn-circle"
                disabled={!asset || !onAddToAlbum}
                onClick={() => asset && void onAddToAlbum?.(asset)}
              >
                <BookPlus />
              </button>
            </div>
            {onFindSimilar && (
              <div
                className="tooltip tooltip-bottom"
                data-tip={t("assets.mediaViewer.similar", "Similar")}
              >
                <button
                  type="button"
                  className="btn btn-soft btn-circle"
                  disabled={!asset}
                  onClick={handleFindSimilar}
                  aria-label={t("assets.mediaViewer.similar", "Similar")}
                >
                  <Images />
                </button>
              </div>
            )}
            {onShare && (
              <div
                className="tooltip tooltip-bottom"
                data-tip={t("exportModal.share", { defaultValue: "Share" })}
              >
                <button
                  aria-label={t("exportModal.share", { defaultValue: "Share" })}
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
                aria-label={t("exportModal.downloadOriginal", {
                  defaultValue: "Download Original",
                })}
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
                aria-label={t("exportModal.viewOriginal", {
                  defaultValue: "View Original in New Tab",
                })}
                type="button"
                className="btn btn-soft btn-circle"
                onClick={() => void handleOpenOriginal()}
                disabled={!originalUrl || isDownloading}
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
                aria-label={t("exportModal.retryProcessing", { defaultValue: "Retry Processing" })}
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
            <button
              type="button"
              className="btn btn-primary w-full mt-4"
              disabled={!canAct}
              onClick={handleExport}
            >
              <ImageDown /> {t("photoExport.title", "Export photo")}
            </button>
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
