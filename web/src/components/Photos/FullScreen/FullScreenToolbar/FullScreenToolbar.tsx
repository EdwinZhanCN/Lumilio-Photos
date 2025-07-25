import {
  InformationCircleIcon,
  ShareIcon,
  TrashIcon,
  ArrowDownTrayIcon,
  Cog6ToothIcon,
} from "@heroicons/react/24/outline";
import { useState } from "react";
import {
  useExportImage,
  ExportOptions,
} from "@/hooks/util-hooks/useExportImage.tsx";

interface FullScreenToolbarProps {
  onToggleInfo: () => void;
  currentAsset?: Asset;
}

const FullScreenToolbar = ({
  onToggleInfo,
  currentAsset,
}: FullScreenToolbarProps) => {
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
          <button className="btn btn-ghost btn-sm" onClick={onToggleInfo}>
            <InformationCircleIcon className="h-6 w-6" />
          </button>
          <button className="btn btn-ghost btn-sm">
            <ShareIcon className="h-6 w-6" />
          </button>
          <div className="dropdown dropdown-end">
            <button
              className="btn btn-ghost btn-sm"
              tabIndex={0}
              disabled={!currentAsset || isExporting}
            >
              <ArrowDownTrayIcon className="h-6 w-6" />
            </button>
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
          <button className="btn btn-ghost btn-sm text-error">
            <TrashIcon className="h-6 w-6" />
          </button>
        </div>
      </div>

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
