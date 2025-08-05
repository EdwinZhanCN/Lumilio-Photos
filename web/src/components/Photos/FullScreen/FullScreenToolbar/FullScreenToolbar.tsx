import {
  InformationCircleIcon,
  ShareIcon,
  TrashIcon,
  ArrowDownTrayIcon,
  Cog6ToothIcon,
  ArrowTopRightOnSquareIcon,
  GlobeAsiaAustraliaIcon,
  HeartIcon,
  ArchiveBoxArrowDownIcon,
} from "@heroicons/react/24/outline";
import { useState } from "react";
import {
  useExportImage,
  ExportOptions,
} from "@/hooks/util-hooks/useExportImage.tsx";
import { getAssetService } from "@/services/getAssetsService";
import { EllipsisHorizontalIcon } from "@heroicons/react/24/solid";

interface FullScreenToolbarProps {
  onToggleInfo: () => void;
  currentAsset?: Asset;
}

const FullScreenToolbar = ({
  onToggleInfo,
  currentAsset,
}: FullScreenToolbarProps) => {
  const wikiAvailable = true;
  const [showExportModal, setShowExportModal] = useState(false);
  const [exportOptions, setExportOptions] = useState<ExportOptions>({
    format: "original",
    quality: 0.9,
    maxWidth: undefined,
    maxHeight: undefined,
  });

  const {
    isExporting,
    exportProgress,
    downloadOriginal,
    exportImage,
    cancelExport,
  } = useExportImage();

  /**
   * Download the original file without any processing
   */
  const handleDownloadOriginal = async () => {
    if (!currentAsset) return;
    await downloadOriginal(currentAsset);
  };

  /**
   * Show export options modal
   */
  const handleShowExportOptions = () => {
    setShowExportModal(true);
  };

  /**
   * Handle export with current options
   */
  const handleExport = async () => {
    if (!currentAsset) return;

    if (exportOptions.format === "original") {
      await downloadOriginal(currentAsset);
    } else {
      await exportImage(currentAsset, exportOptions);
    }

    setShowExportModal(false);
  };

  /**
   * Handle open orginal image in new tab
   * todo)) Support RAW
   */
  const handleOpenOriginalInNewTab = () => {
    if (!currentAsset) return;

    let url: string | undefined;
    try {
      if (!currentAsset?.asset_id) {
        throw new Error("Asset ID is missing.");
      }
      url = getAssetService.getOriginalFileUrl(currentAsset.asset_id);
      if (!url) {
        throw new Error("Failed to get original file URL.");
      }
    } catch (error) {
      console.error("Error opening original image in new tab:", error);
      return;
    }
    window.open(url, "_blank");
  };

  // [TODO] Implement wiki
  const ToggleWikiPanel = () => {
    const modal = document.getElementById(
      "wiki_modal",
    ) as HTMLDialogElement | null;
    if (modal) {
      modal.showModal();
      console.log("wiki panel toggled");
    } else {
      console.error("Wiki modal not found");
    }
  };

  return (
    <>
      <div className="absolute top-0 left-0 right-0 bg-base-100/50 p-2 flex justify-between items-center z-10">
        <div>
          {isExporting && exportProgress && (
            <div className="flex items-center space-x-2 text-sm">
              <span className="loading loading-spinner loading-xs"></span>
              <span>
                {exportProgress.currentFile
                  ? `Exporting ${exportProgress.currentFile}...`
                  : `Processing... ${exportProgress.processed}%`}
              </span>
              <button className="btn btn-ghost btn-xs" onClick={cancelExport}>
                Cancel
              </button>
            </div>
          )}
        </div>
        <div className="flex items-center space-x-4">
          {wikiAvailable && (
            <div className="tooltip tooltip-bottom" data-tip="Wiki">
              <button
                className="btn btn-ghost btn-sm"
                onClick={ToggleWikiPanel}
              >
                <GlobeAsiaAustraliaIcon className="h-6 w-6" />
              </button>
            </div>
          )}

          <div
            className="tooltip tooltip-bottom"
            data-tip="Open original in new tab"
          >
            <button
              className="btn btn-ghost btn-sm"
              onClick={handleOpenOriginalInNewTab}
            >
              <ArrowTopRightOnSquareIcon className="h-6 w-6" />
            </button>
          </div>
          <div className="tooltip tooltip-bottom" data-tip="Info">
            <button className="btn btn-ghost btn-sm" onClick={onToggleInfo}>
              <InformationCircleIcon className="h-6 w-6" />
            </button>
          </div>
          <div className="tooltip tooltip-bottom" data-tip="Add to Favorite">
            <button className="btn btn-ghost btn-sm">
              <HeartIcon className="h-6 w-6" />
            </button>
          </div>
          <div className="tooltip tooltip-bottom" data-tip="Share">
            <button className="btn btn-ghost btn-sm">
              <ShareIcon className="h-6 w-6" />
            </button>
          </div>
          <div className="dropdown dropdown-end">
            <div
              className="tooltip tooltip-bottom"
              data-tip="Download / Export"
            >
              <button
                className="btn btn-ghost btn-sm"
                tabIndex={0}
                disabled={!currentAsset || isExporting}
              >
                <ArrowDownTrayIcon className="h-6 w-6" />
              </button>
            </div>
            <ul className="dropdown-content menu p-2 shadow bg-base-100 rounded-box w-52 z-20">
              <li>
                <button onClick={handleDownloadOriginal}>
                  Download Original
                </button>
              </li>
              <li>
                <button onClick={handleShowExportOptions}>
                  <Cog6ToothIcon className="h-4 w-4" />
                  Export Options...
                </button>
              </li>
            </ul>
          </div>
          <div className="dropdown dropdown-end">
            <div
              tabIndex={0}
              role="button"
              className="btn btn-ghost btn-sm m-1"
            >
              <EllipsisHorizontalIcon className="h-6 w-6" />
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu bg-base-100 rounded-box z-1 w-52 p-2 shadow-sm"
            >
              <li>
                <div>
                  <button className="text-error">
                    <TrashIcon className="h-6 w-6" />
                  </button>
                  Delete
                </div>
              </li>
              <li>
                <div>
                  <button>
                    <ArchiveBoxArrowDownIcon className="h-6 w-6" />
                  </button>
                  Add to Collections
                </div>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <dialog id="wiki_modal" className="modal">
        <div className="modal-box">
          <form method="dialog">
            {/* if there is a button in form, it will close the modal */}
            <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
              ✕
            </button>
          </form>
          <h3 className="font-bold text-lg">Wiki</h3>
          <p className="py-4">Press ESC key or click on ✕ button to close</p>
          <span className="loading loading-bars loading-xs"></span>
        </div>
      </dialog>

      {/* Export Options Modal */}
      {showExportModal && (
        <div className="modal modal-open">
          <div className="modal-box">
            <h3 className="font-bold text-lg mb-4">Export Options</h3>

            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">Format</span>
              </label>
              <select
                className="select select-bordered"
                value={exportOptions.format}
                onChange={(e) =>
                  setExportOptions((prev) => ({
                    ...prev,
                    format: e.target.value as ExportOptions["format"],
                  }))
                }
              >
                <option value="original">Original</option>
                <option value="jpeg">JPEG</option>
                <option value="png">PNG</option>
                <option value="webp">WebP</option>
              </select>
            </div>

            {exportOptions.format !== "original" && (
              <>
                <div className="form-control mb-4">
                  <label className="label">
                    <span className="label-text">
                      Quality: {Math.round(exportOptions.quality * 100)}%
                    </span>
                  </label>
                  <input
                    type="range"
                    min="10"
                    max="100"
                    value={exportOptions.quality * 100}
                    className="range range-sm"
                    onChange={(e) =>
                      setExportOptions((prev) => ({
                        ...prev,
                        quality: parseInt(e.target.value) / 100,
                      }))
                    }
                  />
                </div>

                <div className="grid grid-cols-2 gap-4 mb-4">
                  <div className="form-control">
                    <label className="label">
                      <span className="label-text">Max Width (px)</span>
                    </label>
                    <input
                      type="number"
                      placeholder="Auto"
                      className="input input-bordered input-sm"
                      value={exportOptions.maxWidth || ""}
                      onChange={(e) =>
                        setExportOptions((prev) => ({
                          ...prev,
                          maxWidth: e.target.value
                            ? parseInt(e.target.value)
                            : undefined,
                        }))
                      }
                    />
                  </div>
                  <div className="form-control">
                    <label className="label">
                      <span className="label-text">Max Height (px)</span>
                    </label>
                    <input
                      type="number"
                      placeholder="Auto"
                      className="input input-bordered input-sm"
                      value={exportOptions.maxHeight || ""}
                      onChange={(e) =>
                        setExportOptions((prev) => ({
                          ...prev,
                          maxHeight: e.target.value
                            ? parseInt(e.target.value)
                            : undefined,
                        }))
                      }
                    />
                  </div>
                </div>
              </>
            )}

            <div className="modal-action">
              <button
                className="btn btn-ghost"
                onClick={() => setShowExportModal(false)}
                disabled={isExporting}
              >
                Cancel
              </button>
              <button
                className="btn btn-primary"
                onClick={handleExport}
                disabled={isExporting}
              >
                {isExporting ? (
                  <>
                    <span className="loading loading-spinner loading-xs"></span>
                    Exporting...
                  </>
                ) : (
                  "Export"
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

export default FullScreenToolbar;
