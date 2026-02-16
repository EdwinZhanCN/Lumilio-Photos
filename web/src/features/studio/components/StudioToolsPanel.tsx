import { PanelType } from "../routes/Studio";
import { ExifDataDisplay } from "./panels/ExifDataDisplay";
import { DevelopPanel } from "./panels/DevelopPanel";
import { FramesPanel } from "./panels/FramesPanel";
import { ExifExtractionProgress } from "@/hooks/util-hooks/useExtractExifdata";
import { useI18n } from "@/lib/i18n.tsx";
import type {
  CatalogPluginSummary,
  InstalledPluginRecord,
  StudioPluginUiModule,
} from "@/features/studio/plugins/types";

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
  isGeneratingPlugin: boolean;
  pluginProgress: StudioFrameProgress;
  onGeneratePlugin: () => Promise<void>;
  pluginRuntimeEnabled: boolean;
  installedPlugins: InstalledPluginRecord[];
  catalogPlugins: CatalogPluginSummary[];
  selectedPluginId: string | null;
  onSelectPlugin: (pluginId: string) => void;
  onInstallPlugin: (pluginId: string, version: string) => void;
  onUninstallPlugin: (pluginId: string) => void;
  isPluginInstalled: (pluginId: string, version?: string) => boolean;
  pluginUiModule: StudioPluginUiModule | null;
  pluginParams: Record<string, unknown>;
  onPluginParamsChange: (next: Record<string, unknown>) => void;
  pluginLoading: boolean;
  pluginError: string | null;
  catalogLoading: boolean;
  catalogError: string | null;
  onCancelPluginGeneration?: () => void;
  isCancellingPlugin?: boolean;
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
  isGeneratingPlugin,
  pluginProgress,
  onGeneratePlugin,
  pluginRuntimeEnabled,
  installedPlugins,
  catalogPlugins,
  selectedPluginId,
  onSelectPlugin,
  onInstallPlugin,
  onUninstallPlugin,
  isPluginInstalled,
  pluginUiModule,
  pluginParams,
  onPluginParamsChange,
  pluginLoading,
  pluginError,
  catalogLoading,
  catalogError,
  onCancelPluginGeneration,
  isCancellingPlugin = false,
  onCancelExtraction,
  isCancellingExif = false,
}: StudioToolsPanelProps) {
  const { t } = useI18n();

  const isLoading = isExtracting || isGeneratingPlugin;
  const currentProgress =
    activePanel === "exif"
      ? exifProgress
      : activePanel === "frames"
        ? pluginProgress
        : null;

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
            isGenerating={isGeneratingPlugin}
            onGeneratePlugin={onGeneratePlugin}
            pluginRuntimeEnabled={pluginRuntimeEnabled}
            installedPlugins={installedPlugins}
            catalogPlugins={catalogPlugins}
            selectedPluginId={selectedPluginId}
            onSelectPlugin={onSelectPlugin}
            onInstallPlugin={onInstallPlugin}
            onUninstallPlugin={onUninstallPlugin}
            isPluginInstalled={isPluginInstalled}
            pluginUiModule={pluginUiModule}
            pluginParams={pluginParams}
            onPluginParamsChange={onPluginParamsChange}
            pluginLoading={pluginLoading}
            pluginError={pluginError}
            catalogLoading={catalogLoading}
            catalogError={catalogError}
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

              {isGeneratingPlugin && (
                <button
                  onClick={onCancelPluginGeneration}
                  className="btn btn-xs btn-outline btn-error mt-2"
                  disabled={isCancellingPlugin}
                >
                  {isCancellingPlugin
                    ? t("studio.cancelling")
                    : t("studio.frames.plugin.cancel")}
                </button>
              )}
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
