import {
  BookPlus,
  CloudDownload,
  ImageDown,
  Share2,
  SquareArrowOutUpRight,
  X,
  RefreshCw,
  Loader2,
  Paintbrush,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useExportImage, type ExportOptions } from "../../../../hooks/useExportImage";
import { assetUrls } from "@/lib/assets/assetUrls";
import { $api } from "@/lib/http-commons/queryClient";
import type { Asset } from "@/lib/assets/types";
import {
  getRetryTasksByCategoryForAssetType,
  isRetryTaskSupportedForAssetType,
} from "@/config/retryTasks";
import { isExportSupported } from "../../../../utils/mediaTypes";
import { useI18n } from "@/lib/i18n";

type ExportFormat = "png" | "jpeg" | "webp" | "avif";

interface ExportModalProps {
  asset?: Asset;

  onCopyOriginalUrl?: (url: string) => void | Promise<void>;
  onDownloadOriginal?: (asset: Asset) => void | Promise<void>;
  onOpenOriginalInNewTab?: (asset: Asset) => void | Promise<void>;
  onExport?: (asset: Asset, options: ExportOptions) => void | Promise<void>;

  onOpenStudio?: (asset: Asset) => void;
  onAddToAlbum?: (asset: Asset) => void | Promise<void>;
  onShare?: (asset: Asset) => void;
}

export default function ExportModal({
  asset,
  onDownloadOriginal,
  onOpenOriginalInNewTab,
  onExport,
  onOpenStudio,
  onAddToAlbum,
  onShare,
}: ExportModalProps) {
  const { isExporting, exportImage, downloadOriginal: defaultDownloadOriginal } = useExportImage();
  const { t } = useI18n();

  const [format, setFormat] = useState<ExportFormat>("png");

  // Retry-related state
  const [selectedTasks, setSelectedTasks] = useState<string[]>([]);
  const [forceFullRetry, setForceFullRetry] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const reprocessMutation = $api.useMutation("post", "/api/v1/assets/{id}/reprocess");

  const originalUrl = useMemo(() => {
    if (!asset?.asset_id) return "";
    try {
      return assetUrls.getOriginalFileUrl(asset.asset_id) || "";
    } catch {
      return "";
    }
  }, [asset?.asset_id]);

  const canAct = !!asset && !isExporting;
  const canExport = !!asset && isExportSupported(asset);
  const retryTasksByCategory = useMemo(
    () => getRetryTasksByCategoryForAssetType(asset?.type),
    [asset?.type],
  );
  const supportedSelectedTasks = useMemo(
    () => selectedTasks.filter((task) => isRetryTaskSupportedForAssetType(task, asset?.type)),
    [asset?.type, selectedTasks],
  );
  const hasRetryableTasks =
    retryTasksByCategory.metadata.length > 0 ||
    retryTasksByCategory.media.length > 0 ||
    retryTasksByCategory.ml.length > 0;
  const canSubmitRetry =
    !!asset?.asset_id && !isRetrying && (forceFullRetry || supportedSelectedTasks.length > 0);

  useEffect(() => {
    setSelectedTasks((current) =>
      current.filter((task) => isRetryTaskSupportedForAssetType(task, asset?.type)),
    );
  }, [asset?.type]);

  const handleDownloadOriginal = useCallback(async () => {
    if (!asset) return;
    if (onDownloadOriginal) {
      await onDownloadOriginal(asset);
      return;
    }
    await defaultDownloadOriginal(asset);
  }, [asset, onDownloadOriginal, defaultDownloadOriginal]);

  const handleOpenOriginalInNewTab = useCallback(async () => {
    if (!asset) return;
    if (onOpenOriginalInNewTab) {
      await onOpenOriginalInNewTab(asset);
      return;
    }
    if (!originalUrl) return;
    window.open(originalUrl, "_blank", "noopener,noreferrer");
  }, [asset, onOpenOriginalInNewTab, originalUrl]);

  const handleShare = useCallback(() => {
    if (!asset || !onShare) return;
    // CreateShareLinkModal isn't a native <dialog>, so it can't stack above
    // this top-layer dialog — close it first so the share modal is visible.
    const modal = document.getElementById("export_modal") as HTMLDialogElement | null;
    modal?.close();
    onShare(asset);
  }, [asset, onShare]);

  const buildExportOptions = useCallback((): ExportOptions => {
    switch (format) {
      case "jpeg":
        return {
          format: "jpeg",
          quality: 0.8,
        };
      case "png":
        return {
          format: "png",
          quality: 1,
        };
      case "webp":
        return {
          format: "webp",
          quality: 0.8,
        };
      case "avif":
        return {
          format: "avif",
          quality: 0.55,
        };
      default:
        return { format: "png", quality: 1 };
    }
  }, [format]);

  const handleExport = useCallback(async () => {
    if (!asset || !canExport) return;
    const options = buildExportOptions();
    if (onExport) {
      await onExport(asset, options);
    } else {
      await exportImage(asset, options);
    }
    // Close modal after export (if dialog exists)
    const modal = document.getElementById("export_modal") as HTMLDialogElement | null;
    modal?.close();
  }, [asset, canExport, buildExportOptions, exportImage, onExport]);

  const handleRetry = useCallback(async () => {
    if (!asset?.asset_id || (!forceFullRetry && supportedSelectedTasks.length === 0)) {
      return;
    }

    setIsRetrying(true);
    try {
      await reprocessMutation.mutateAsync({
        params: { path: { id: asset.asset_id } },
        body: {
          tasks: forceFullRetry ? [] : supportedSelectedTasks,
          force_full_retry: forceFullRetry,
        },
      });

      // Close retry modal on success
      const retryModal = document.getElementById("retry_modal") as HTMLDialogElement | null;
      retryModal?.close();

      // Reset state
      setSelectedTasks([]);
      setForceFullRetry(false);

      // TODO: Show success toast
      console.log(
        "Retry job submitted successfully for tasks:",
        forceFullRetry ? "full retry" : supportedSelectedTasks,
      );
    } catch (err) {
      console.error("Failed to submit retry job:", err);
      // TODO: Show error toast
    } finally {
      setIsRetrying(false);
    }
  }, [asset, forceFullRetry, reprocessMutation, supportedSelectedTasks]);

  return (
    <dialog id="export_modal" className="modal">
      <div className="modal-box">
        <form method="dialog">
          {/* if there is a button in form, it will close the modal */}
          <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
            <X />
          </button>
        </form>

        <div className="flex flex-wrap gap-2 mb-2">
          <div
            className="tooltip tooltip-bottom"
            data-tip={t("exportModal.studio", { defaultValue: "Studio" })}
          >
            <button
              className="btn btn-soft btn-circle"
              onClick={() => {
                if (asset && onOpenStudio) onOpenStudio(asset);
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
              className="btn btn-soft btn-circle"
              onClick={() => {
                if (asset && onAddToAlbum) void onAddToAlbum(asset);
              }}
            >
              <BookPlus />
            </button>
          </div>

          {onShare && (
            <div
              className="tooltip tooltip-bottom"
              data-tip={t("exportModal.share", { defaultValue: "Share" })}
            >
              <button className="btn btn-soft btn-circle" onClick={handleShare} disabled={!canAct}>
                <Share2 />
              </button>
            </div>
          )}

          <div
            className="tooltip tooltip-bottom"
            data-tip={t("exportModal.downloadOriginal", { defaultValue: "Download Original" })}
          >
            <button
              className="btn btn-soft btn-circle"
              onClick={handleDownloadOriginal}
              disabled={!canAct}
            >
              <CloudDownload />
            </button>
          </div>

          <div
            className="tooltip tooltip-bottom"
            data-tip={t("exportModal.viewOriginal", { defaultValue: "View Original in New Tab" })}
          >
            <button
              className="btn btn-soft btn-circle"
              onClick={handleOpenOriginalInNewTab}
              disabled={!originalUrl || isExporting}
            >
              <SquareArrowOutUpRight />
            </button>
          </div>

          <div
            className="tooltip tooltip-bottom"
            data-tip={t("exportModal.retryProcessing", { defaultValue: "Retry Processing" })}
          >
            <button
              className="btn btn-soft btn-circle"
              disabled={!asset}
              onClick={() => {
                const retryModal = document.getElementById(
                  "retry_modal",
                ) as HTMLDialogElement | null;
                retryModal?.showModal();
              }}
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
              onChange={(e) => setFormat(e.target.value as ExportFormat)}
              disabled={!canAct}
            >
              <option value="png">{t("exportModal.format.png", { defaultValue: "PNG" })}</option>
              <option value="jpeg">
                {t("exportModal.format.jpeg", { defaultValue: "JPEG (80%)" })}
              </option>
              <option value="webp">
                {t("exportModal.format.webp", { defaultValue: "WebP (80%)" })}
              </option>
              <option value="avif">{t("exportModal.format.avif", { defaultValue: "AVIF" })}</option>
            </select>
            <span className="label">{t("exportModal.optional", { defaultValue: "Optional" })}</span>
            <button className="btn btn-soft btn-primary" onClick={handleExport} disabled={!canAct}>
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
          <div className="mt-3 text-sm opacity-70">
            {t("exportModal.exportUnavailable", {
              defaultValue:
                "Export conversion is unavailable for video and audio assets. You can still download the original file.",
            })}
          </div>
        )}

        {!asset && (
          <div className="mt-3 text-xs opacity-70">
            {t("exportModal.noAsset", {
              defaultValue: "No asset selected. Actions are disabled.",
            })}
          </div>
        )}
      </div>

      {/* Retry Modal */}
      <dialog id="retry_modal" className="modal">
        <div className="modal-box max-w-lg">
          <form method="dialog">
            <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
              <X />
            </button>
          </form>

          <h3 className="font-semibold text-lg mb-1">
            {t("exportModal.retryTitle", { defaultValue: "Retry Processing Tasks" })}
          </h3>
          <p className="text-xs opacity-60 mb-4">
            {t("exportModal.selectedCount", {
              defaultValue: "{{count}} selected",
              count: supportedSelectedTasks.length,
            })}
          </p>

          <div className="flex flex-col gap-3 max-h-[60vh] overflow-y-auto pr-1">
            {/* Metadata Tasks */}
            {retryTasksByCategory.metadata.length > 0 && (
              <div className="border border-base-300 rounded-lg overflow-hidden">
                <div className="bg-base-200/50 px-3 py-1.5 text-xs font-semibold opacity-70 uppercase tracking-wider">
                  {t("exportModal.category.metadata", { defaultValue: "Metadata" })}
                </div>
                <div className="divide-y divide-base-200">
                  {retryTasksByCategory.metadata.map((task) => (
                    <label
                      key={task.key}
                      className="flex items-start gap-3 px-3 py-2.5 hover:bg-base-200/50 cursor-pointer transition-colors"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm mt-0.5"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(selectedTasks.filter((t) => t !== task.key));
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium">
                          {t(`exportModal.retryTasks.${task.key}.label`, {
                            defaultValue: task.label,
                          })}
                        </div>
                        <div className="text-xs opacity-60 mt-0.5">
                          {t(`exportModal.retryTasks.${task.key}.description`, {
                            defaultValue: task.description,
                          })}
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* Media Tasks */}
            {retryTasksByCategory.media.length > 0 && (
              <div className="border border-base-300 rounded-lg overflow-hidden">
                <div className="bg-base-200/50 px-3 py-1.5 text-xs font-semibold opacity-70 uppercase tracking-wider">
                  {t("exportModal.category.media", { defaultValue: "Media Processing" })}
                </div>
                <div className="divide-y divide-base-200">
                  {retryTasksByCategory.media.map((task) => (
                    <label
                      key={task.key}
                      className="flex items-start gap-3 px-3 py-2.5 hover:bg-base-200/50 cursor-pointer transition-colors"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm mt-0.5"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(selectedTasks.filter((t) => t !== task.key));
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium">
                          {t(`exportModal.retryTasks.${task.key}.label`, {
                            defaultValue: task.label,
                          })}
                        </div>
                        <div className="text-xs opacity-60 mt-0.5">
                          {t(`exportModal.retryTasks.${task.key}.description`, {
                            defaultValue: task.description,
                          })}
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {/* ML Tasks */}
            {retryTasksByCategory.ml.length > 0 && (
              <div className="border border-base-300 rounded-lg overflow-hidden">
                <div className="bg-base-200/50 px-3 py-1.5 text-xs font-semibold opacity-70 uppercase tracking-wider">
                  {t("exportModal.category.ml", { defaultValue: "Machine Learning" })}
                </div>
                <div className="divide-y divide-base-200">
                  {retryTasksByCategory.ml.map((task) => (
                    <label
                      key={task.key}
                      className="flex items-start gap-3 px-3 py-2.5 hover:bg-base-200/50 cursor-pointer transition-colors"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm mt-0.5"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(selectedTasks.filter((t) => t !== task.key));
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="text-sm font-medium">
                          {t(`exportModal.retryTasks.${task.key}.label`, {
                            defaultValue: task.label,
                          })}
                        </div>
                        <div className="text-xs opacity-60 mt-0.5">
                          {t(`exportModal.retryTasks.${task.key}.description`, {
                            defaultValue: task.description,
                          })}
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            )}

            {!hasRetryableTasks && (
              <div className="text-sm opacity-70 py-4 text-center">
                {t("exportModal.noRetryTasks", {
                  defaultValue: "No retry tasks are available for this asset type.",
                })}
              </div>
            )}

            {/* Force full retry */}
            <div className="border border-base-300 rounded-lg p-3">
              <label className="flex items-start gap-3 cursor-pointer">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm mt-0.5"
                  checked={forceFullRetry}
                  onChange={(e) => setForceFullRetry(e.target.checked)}
                  disabled={isRetrying}
                />
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium">
                    {t("exportModal.forceFullRetry", { defaultValue: "Force full retry" })}
                  </div>
                  <div className="text-xs opacity-60 mt-0.5">
                    {t("exportModal.forceFullRetryHint", {
                      defaultValue: "Re-run all processing tasks regardless of previous status",
                    })}
                  </div>
                </div>
              </label>
            </div>
          </div>

          <div className="modal-action mt-4 pt-3 border-t border-base-300">
            <form method="dialog">
              <button className="btn btn-ghost btn-sm" disabled={isRetrying}>
                {t("common.cancel")}
              </button>
            </form>
            <button
              className="btn btn-primary btn-sm"
              disabled={!canSubmitRetry}
              onClick={handleRetry}
            >
              {isRetrying ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  {t("exportModal.submitting", { defaultValue: "Submitting..." })}
                </>
              ) : (
                <>
                  <RefreshCw className="size-4" />
                  {t("exportModal.submitRetry", { defaultValue: "Submit Retry" })}
                </>
              )}
            </button>
          </div>
        </div>
      </dialog>
    </dialog>
  );
}
