import { PanelType } from "@/pages/Studio";
import { ExifDataDisplay } from "./panels/ExifDataDisplay";
import { DevelopPanel } from "./panels/DevelopPanel";
import { FramesPanel } from "./panels/FramesPanel";
import {
  BorderOptions,
  BorderParams,
} from "@/hooks/wasm-hooks/useGenerateBorder";

type StudioToolsPanelProps = {
  selectedFile: File | null;
  activePanel: PanelType;
  isExtracting: boolean;
  exifProgress: { numberProcessed: number; total: number } | null;
  exifToDisplay: Record<string, any> | null;
  onExtractExif: () => void;
  isGeneratingBorders: boolean;
  borderProgress: { numberProcessed: number; total: number } | null;
  onGenerateBorders: (
    option: BorderOptions,
    param: BorderParams[BorderOptions],
  ) => Promise<void>;
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
  const currentProgress = isExtracting ? exifProgress : borderProgress;

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
                Processing: {currentProgress.numberProcessed} /{" "}
                {currentProgress.total}
              </p>
              <progress
                className="progress progress-primary w-full"
                value={currentProgress.numberProcessed}
                max={currentProgress.total}
              ></progress>
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
