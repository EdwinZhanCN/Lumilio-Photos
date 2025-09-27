import {
  BookPlus,
  CloudDownload,
  ImageDown,
  Link,
  SquareArrowOutUpRight,
  X,
} from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import {
  useExportImage,
  type ExportOptions,
} from "@/hooks/util-hooks/useExportImage.tsx";
import { assetService } from "@/services/assetsService";
import { PaintBrushIcon } from "@heroicons/react/24/outline";

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

  const originalUrl = useMemo(() => {
    if (!asset?.asset_id) return "";
    try {
      return assetService.getOriginalFileUrl(asset.asset_id) || "";
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
    </dialog>
  );
}
