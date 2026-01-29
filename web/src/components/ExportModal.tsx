import {
  BookPlus,
  CloudDownload,
  ImageDown,
  Link,
  SquareArrowOutUpRight,
  X,
  RefreshCw,
  Loader2,
} from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import {
  useExportImage,
  type ExportOptions,
} from "@/hooks/util-hooks/useExportImage.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import { $api } from "@/lib/http-commons/queryClient";
import { PaintBrushIcon } from "@heroicons/react/24/outline";
import { Asset } from "@/lib/assets/types";
import { RETRY_TASKS_BY_CATEGORY } from "@/config/retryTasks";

type ExportFormat = "png" | "jpeg" | "webp";

interface ExportModalProps {
  asset?: Asset;

  onCopyOriginalUrl?: (url: string) => void | Promise<void>;
  onDownloadOriginal?: (asset: Asset) => void | Promise<void>;
  onOpenOriginalInNewTab?: (asset: Asset) => void | Promise<void>;
  onExport?: (asset: Asset, options: ExportOptions) => void | Promise<void>;

  // Unfinished features (kept for future wiring)
  onAddToAlbum?: (asset: Asset) => void | Promise<void>;
  onCopyShareLink?: (asset: Asset) => void | Promise<void>;
}

export default function ExportModal({
  asset,
  onDownloadOriginal,
  onOpenOriginalInNewTab,
  onExport,
  onAddToAlbum,
  onCopyShareLink,
}: ExportModalProps) {
  const {
    isExporting,
    exportImage,
    downloadOriginal: defaultDownloadOriginal,
  } = useExportImage();

  const [format, setFormat] = useState<ExportFormat>("png");

  // Retry-related state
  const [selectedTasks, setSelectedTasks] = useState<string[]>([]);
  const [forceFullRetry, setForceFullRetry] = useState(false);
  const [isRetrying, setIsRetrying] = useState(false);
  const reprocessMutation = $api.useMutation(
    "post",
    "/api/v1/assets/{id}/reprocess",
  );

  const originalUrl = useMemo(() => {
    if (!asset?.asset_id) return "";
    try {
      return assetUrls.getOriginalFileUrl(asset.asset_id) || "";
    } catch {
      return "";
    }
  }, [asset?.asset_id]);

  const canAct = !!asset && !isExporting;

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
          quality: 0.92,
        };
      default:
        return { format: "png", quality: 1 };
    }
  }, [format]);

  const handleExport = useCallback(async () => {
    if (!asset) return;
    const options = buildExportOptions();
    if (onExport) {
      await onExport(asset, options);
    } else {
      await exportImage(asset, options);
    }
    // Close modal after export (if dialog exists)
    const modal = document.getElementById(
      "export_modal",
    ) as HTMLDialogElement | null;
    modal?.close();
  }, [asset, buildExportOptions, exportImage, onExport]);

  const handleRetry = useCallback(async () => {
    if (!asset?.asset_id || selectedTasks.length === 0) return;

    setIsRetrying(true);
    try {
      await reprocessMutation.mutateAsync({
        params: { path: { id: asset.asset_id } },
        body: {
          tasks: selectedTasks,
          force_full_retry: forceFullRetry,
        },
      });

      // Close retry modal on success
      const retryModal = document.getElementById(
        "retry_modal",
      ) as HTMLDialogElement | null;
      retryModal?.close();

      // Reset state
      setSelectedTasks([]);
      setForceFullRetry(false);

      // TODO: Show success toast
      console.log("Retry job submitted successfully for tasks:", selectedTasks);
    } catch (err) {
      console.error("Failed to submit retry job:", err);
      // TODO: Show error toast
    } finally {
      setIsRetrying(false);
    }
  }, [asset, selectedTasks, forceFullRetry]);

  return (
    <dialog id="export_modal" className="modal">
      <div className="modal-box">
        <form method="dialog">
          {/* if there is a button in form, it will close the modal */}
          <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
            <X />
          </button>
        </form>

        <div className="flex gap-2 mb-2">
          <div className="tooltip tooltip-bottom" data-tip="Studio">
            <button className="btn btn-soft btn-circle" disabled>
              <PaintBrushIcon className="size-6" />
            </button>
          </div>

          {/* TODO: Add to album not Implement */}
          <div className="tooltip tooltip-bottom" data-tip="Add to album">
            <button
              className="btn btn-soft btn-circle"
              disabled
              onClick={() => {
                if (asset && onAddToAlbum) onAddToAlbum(asset);
              }}
            >
              <BookPlus />
            </button>
          </div>

          {/* TODO: Implement the share function */}
          <div
            className="tooltip tooltip-bottom"
            data-tip="Copy the share link"
          >
            <button
              className="btn btn-soft btn-circle"
              disabled
              onClick={() => {
                if (asset && onCopyShareLink) onCopyShareLink(asset);
              }}
            >
              <Link />
            </button>
          </div>

          <div className="tooltip tooltip-bottom" data-tip="Download Original">
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
            data-tip="View Original in New Tab"
          >
            <button
              className="btn btn-soft btn-circle"
              onClick={handleOpenOriginalInNewTab}
              disabled={!originalUrl || isExporting}
            >
              <SquareArrowOutUpRight />
            </button>
          </div>

          <div className="tooltip tooltip-bottom" data-tip="Retry Processing">
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

        <fieldset className="fieldset">
          <legend className="fieldset-legend">Export Format</legend>
          <select
            className="select mb-2"
            value={format}
            onChange={(e) => setFormat(e.target.value as ExportFormat)}
            disabled={!canAct}
          >
            <option value="png">PNG</option>
            <option value="jpeg">JPEG (80%)</option>
            <option value="webp">WebP</option>
          </select>
          <span className="label">Optional</span>
          <button
            className="btn btn-soft btn-primary"
            onClick={handleExport}
            disabled={!canAct}
          >
            {isExporting ? (
              <>
                <span className="loading loading-spinner loading-xs" />
                Exporting...
              </>
            ) : (
              <>
                <ImageDown /> Export
              </>
            )}
          </button>
        </fieldset>

        {!asset && (
          <div className="mt-3 text-xs opacity-70">
            No asset selected. Actions are disabled.
          </div>
        )}
      </div>

      {/* Retry Modal */}
      <dialog id="retry_modal" className="modal">
        <div className="modal-box">
          <form method="dialog">
            <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
              <X />
            </button>
          </form>

          <h3 className="font-semibold text-lg mb-4">Retry Processing Tasks</h3>

          <div className="space-y-4">
            <div className="form-control w-full">
              <label className="label">
                <span className="label-text">Select Tasks to Retry</span>
                <span className="label-text-alt">
                  {selectedTasks.length} selected
                </span>
              </label>

              {/* Metadata Tasks */}
              <div className="mb-3">
                <div className="text-xs font-semibold opacity-60 uppercase mb-2">
                  Metadata
                </div>
                <div className="space-y-2">
                  {RETRY_TASKS_BY_CATEGORY.metadata.map((task) => (
                    <label
                      key={task.key}
                      className="label cursor-pointer justify-start gap-3 hover:bg-base-200 rounded px-2"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(
                              selectedTasks.filter((t) => t !== task.key),
                            );
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1">
                        <span className="label-text font-medium">
                          {task.label}
                        </span>
                        <p className="text-xs opacity-60">{task.description}</p>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* Media Tasks */}
              <div className="mb-3">
                <div className="text-xs font-semibold opacity-60 uppercase mb-2">
                  Media Processing
                </div>
                <div className="space-y-2">
                  {RETRY_TASKS_BY_CATEGORY.media.map((task) => (
                    <label
                      key={task.key}
                      className="label cursor-pointer justify-start gap-3 hover:bg-base-200 rounded px-2"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(
                              selectedTasks.filter((t) => t !== task.key),
                            );
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1">
                        <span className="label-text font-medium">
                          {task.label}
                        </span>
                        <p className="text-xs opacity-60">{task.description}</p>
                      </div>
                    </label>
                  ))}
                </div>
              </div>

              {/* ML Tasks */}
              <div className="mb-3">
                <div className="text-xs font-semibold opacity-60 uppercase mb-2">
                  Machine Learning
                </div>
                <div className="space-y-2">
                  {RETRY_TASKS_BY_CATEGORY.ml.map((task) => (
                    <label
                      key={task.key}
                      className="label cursor-pointer justify-start gap-3 hover:bg-base-200 rounded px-2"
                    >
                      <input
                        type="checkbox"
                        className="checkbox checkbox-sm"
                        checked={selectedTasks.includes(task.key)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setSelectedTasks([...selectedTasks, task.key]);
                          } else {
                            setSelectedTasks(
                              selectedTasks.filter((t) => t !== task.key),
                            );
                          }
                        }}
                        disabled={isRetrying}
                      />
                      <div className="flex-1">
                        <span className="label-text font-medium">
                          {task.label}
                        </span>
                        <p className="text-xs opacity-60">{task.description}</p>
                      </div>
                    </label>
                  ))}
                </div>
              </div>
            </div>

            <div className="form-control">
              <label className="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm"
                  checked={forceFullRetry}
                  onChange={(e) => setForceFullRetry(e.target.checked)}
                  disabled={isRetrying}
                />
                <span className="label-text">Force full retry</span>
              </label>
              <label className="label">
                <span className="label-text-alt opacity-70">
                  Re-run all processing tasks regardless of previous status
                </span>
              </label>
            </div>
          </div>

          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost" disabled={isRetrying}>
                Cancel
              </button>
            </form>
            <button
              className="btn btn-primary"
              disabled={selectedTasks.length === 0 || isRetrying}
              onClick={handleRetry}
            >
              {isRetrying ? (
                <>
                  <Loader2 className="size-4 animate-spin" />
                  Submitting...
                </>
              ) : (
                <>
                  <RefreshCw className="size-4" />
                  Submit Retry
                </>
              )}
            </button>
          </div>
        </div>
      </dialog>
    </dialog>
  );
}
