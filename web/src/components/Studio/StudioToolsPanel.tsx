import { PanelType } from "@/pages/Studio";
import { ExifDataDisplay } from "./panels/ExifDataDisplay";
import { DevelopPanel } from "./panels/DevelopPanel";
import { FramesPanel } from "./panels/FramesPanel";
import {
  BorderOptions,
  BorderParams,
  BorderGenerationProgress,
} from "@/hooks/util-hooks/useGenerateBorder";
import { ExifExtractionProgress } from "@/hooks/util-hooks/useExtractExifdata";

type StudioToolsPanelProps = {
  selectedFile: File | null;
  activePanel: PanelType;
  isExtracting: boolean;
  exifProgress: ExifExtractionProgress;
  exifToDisplay: Record<string, any> | null;
  onExtractExif: () => void;
  isGeneratingBorders: boolean;
  borderProgress: BorderGenerationProgress;
  onGenerateBorders: (
    option: BorderOptions,
    param: BorderParams[BorderOptions],
  ) => Promise<void>;
  onCancelGeneration?: () => void;
  isCancelling?: boolean;
  onCancelExtraction?: () => void;
  isCancellingExif?: boolean;
};

export function StudioToolsPanel({
  selectedFile,
  activePanel,
  isExtracting,
  exifProgress,
  exifToDisplay,
  onExtractExif,
  isGeneratingBorders,
  borderProgress,
  onGenerateBorders,
  onCancelGeneration,
  isCancelling = false,
  onCancelExtraction,
  isCancellingExif = false,
}: StudioToolsPanelProps) {
  const renderPanelContent = () => {
    switch (activePanel) {
      case "exif":
        return (
          <ExifDataDisplay exifData={exifToDisplay} isLoading={isExtracting} />
        );
      case "develop":
        return <DevelopPanel />;
      case "frames":
        return (
          <FramesPanel
            isGenerating={isGeneratingBorders}
            onGenerate={onGenerateBorders}
          />
        );
      default:
        return null;
    }
  };

  const isLoading = isExtracting || isGeneratingBorders;
  const currentProgress =
    activePanel === "exif"
      ? exifProgress
      : activePanel === "frames"
        ? borderProgress
        : null;

  return (
    <div className="bg-base-200 border-l border-base-content/10 w-80 overflow-y-auto">
      {selectedFile ? (
        <div className="p-4">
          {activePanel === "exif" && !exifToDisplay && !isExtracting && (
            <div className="mb-4">
              <button
                onClick={onExtractExif}
                className="btn btn-secondary w-full"
                disabled={isExtracting}
              >
                Extract Metadata
              </button>
            </div>
          )}

          {isLoading && currentProgress && (
            <div className="mb-4 text-center">
              <p className="text-sm mb-1">
                Processing: {currentProgress.processed} /{" "}
                {currentProgress.total}
              </p>
              <progress
                className="progress progress-primary w-full"
                value={currentProgress.processed}
                max={currentProgress.total}
              ></progress>
              {currentProgress.error && (
                <p className="text-xs text-error mt-1">
                  {currentProgress.error}
                </p>
              )}
              {/* Show appropriate cancel button based on current operation */}
              {isExtracting && onCancelExtraction && (
                <button
                  onClick={onCancelExtraction}
                  className="btn btn-xs btn-outline btn-error mt-2"
                  disabled={isCancellingExif}
                >
                  {isCancellingExif ? "Cancelling..." : "Cancel Extraction"}
                </button>
              )}
              {isGeneratingBorders && onCancelGeneration && (
                <button
                  onClick={onCancelGeneration}
                  className="btn btn-xs btn-outline btn-error mt-2"
                  disabled={isCancelling}
                >
                  {isCancelling ? "Cancelling..." : "Cancel Generation"}
                </button>
              )}
            </div>
          )}

          {renderPanelContent()}
        </div>
      ) : (
        <div className="p-4 text-center text-base-content/70 h-full flex items-center justify-center">
          <p>Select an image to begin.</p>
        </div>
      )}
    </div>
  );
}
