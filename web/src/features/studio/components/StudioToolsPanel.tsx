import type { PanelType } from "../routes/Studio";
import { ExifDataDisplay } from "./panels/ExifDataDisplay";
import { DevelopPanel } from "./panels/DevelopPanel";
import { BorderPanel } from "@/features/studio/tools/border";
import type { ExifExtractionProgress } from "@/hooks/util-hooks/useExtractExifdata";
import { useI18n } from "@/lib/i18n.tsx";

export type StudioFrameProgress = {
  processed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

type StudioToolsPanelProps = {
  selectedFile: File | null;
  activePanel: PanelType;
  isExtracting: boolean;
  exifProgress: ExifExtractionProgress;
  exifToDisplay: Record<string, any> | null;
  onExtractExif: () => void;
  isGeneratingTool: boolean;
  toolProgress: StudioFrameProgress;
  onGenerateTool: () => Promise<void>;
  toolParams: Record<string, unknown>;
  onToolParamsChange: (next: Record<string, unknown>) => void;
  onCancelToolGeneration?: () => void;
  isCancellingTool?: boolean;
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
  isGeneratingTool,
  toolProgress,
  onGenerateTool,
  toolParams,
  onToolParamsChange,
  onCancelToolGeneration,
  isCancellingTool = false,
  onCancelExtraction,
  isCancellingExif = false,
}: StudioToolsPanelProps) {
  const { t } = useI18n();

  const isLoading = isExtracting || isGeneratingTool;
  const currentProgress =
    activePanel === "exif"
      ? exifProgress
      : activePanel === "border"
        ? toolProgress
        : null;

  const renderPanelContent = () => {
    switch (activePanel) {
      case "exif":
        return (
          <ExifDataDisplay exifData={exifToDisplay} isLoading={isExtracting} />
        );
      case "develop":
        return <DevelopPanel />;
      case "border":
        return (
          <BorderPanel
            value={toolParams}
            onChange={onToolParamsChange}
            disabled={isGeneratingTool}
          />
        );
      default:
        return null;
    }
  };

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
                {t("studio.exif.extract")}
              </button>
            </div>
          )}

          {isLoading && currentProgress && (
            <div className="mb-4 text-center">
              <p className="text-sm mb-1">
                {t("studio.processing", {
                  processed: currentProgress.processed,
                  total: currentProgress.total,
                })}
              </p>
              <progress
                className="progress progress-primary w-full"
                value={currentProgress.processed}
                max={currentProgress.total}
              ></progress>
              {currentProgress.error && (
                <p className="text-xs text-error mt-1">{currentProgress.error}</p>
              )}

              {isExtracting && onCancelExtraction && (
                <button
                  onClick={onCancelExtraction}
                  className="btn btn-xs btn-outline btn-error mt-2"
                  disabled={isCancellingExif}
                >
                  {isCancellingExif
                    ? t("studio.cancelling")
                    : t("studio.exif.cancel")}
                </button>
              )}

              {isGeneratingTool && onCancelToolGeneration && (
                <button
                  onClick={onCancelToolGeneration}
                  className="btn btn-xs btn-outline btn-error mt-2"
                  disabled={isCancellingTool}
                >
                  {isCancellingTool
                    ? t("studio.cancelling")
                    : t("studio.tools.cancel", "Cancel")}
                </button>
              )}
            </div>
          )}

          {activePanel === "border" && !isGeneratingTool && (
            <div className="mb-4">
              <button
                onClick={onGenerateTool}
                className="btn btn-primary w-full"
                disabled={isGeneratingTool}
              >
                {isGeneratingTool ? (
                  <span className="loading loading-spinner"></span>
                ) : (
                  t("studio.tools.apply", "Apply Border")
                )}
              </button>
            </div>
          )}

          {renderPanelContent()}
        </div>
      ) : (
        <div className="p-4 text-center text-base-content/70 h-full flex items-center justify-center">
          <p>{t("studio.emptyHint")}</p>
        </div>
      )}
    </div>
  );
}
