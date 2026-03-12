import { Plug } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type {
  InstalledPluginRecord,
  StudioPluginUiModule,
} from "@/features/studio/plugins/types";

type PluginsWorkspacePanelProps = {
  isGenerating: boolean;
  onGeneratePlugin: () => void;
  pluginRuntimeEnabled: boolean;
  installedPlugins: InstalledPluginRecord[];
  selectedPluginId: string | null;
  pluginUiModule: StudioPluginUiModule | null;
  pluginParams: Record<string, unknown>;
  onPluginParamsChange: (next: Record<string, unknown>) => void;
  pluginLoading: boolean;
  pluginError: string | null;
  onOpenMarketplace: () => void;
};

export function PluginsWorkspacePanel({
  isGenerating,
  onGeneratePlugin,
  pluginRuntimeEnabled,
  installedPlugins,
  selectedPluginId,
  pluginUiModule,
  pluginParams,
  onPluginParamsChange,
  pluginLoading,
  pluginError,
  onOpenMarketplace,
}: PluginsWorkspacePanelProps) {
  const { t } = useI18n();

  if (!pluginRuntimeEnabled) {
    return (
      <div>
        <div className="flex items-center mb-4">
          <Plug className="w-5 h-5 mr-2" />
          <h2 className="text-lg font-semibold">{t("studio.plugins.title")}</h2>
        </div>
        <div className="rounded-lg bg-base-100 p-4 text-sm text-base-content/70">
          {t("studio.plugins.runtimeDisabled")}
        </div>
      </div>
    );
  }

  const hasInstalledPlugins = installedPlugins.length > 0;

  return (
    <div>
      <div className="flex items-center mb-4">
        <Plug className="w-5 h-5 mr-2" />
        <h2 className="text-lg font-semibold">{t("studio.plugins.title")}</h2>
      </div>

      {!hasInstalledPlugins ? (
        <div className="rounded-lg bg-base-100 p-4 space-y-3">
          <p className="text-sm text-base-content/70">{t("studio.plugins.emptyInstalled")}</p>
          <button
            type="button"
            className="btn btn-sm btn-outline"
            onClick={onOpenMarketplace}
          >
            {t("studio.plugins.openMarketplace")}
          </button>
        </div>
      ) : (
        <div className="rounded-lg bg-base-100 p-4 space-y-3">
          {pluginLoading && (
            <p className="text-sm text-base-content/70">
              {t("studio.plugins.loading")}
            </p>
          )}

          {pluginError && <p className="text-sm text-error">{pluginError}</p>}

          {pluginUiModule && !pluginLoading && (
            <pluginUiModule.Panel
              value={pluginParams}
              onChange={onPluginParamsChange}
              disabled={isGenerating}
            />
          )}

          <button
            type="button"
            className="btn btn-primary w-full"
            onClick={onGeneratePlugin}
            disabled={isGenerating || !pluginUiModule || !selectedPluginId}
          >
            {isGenerating ? (
              <span className="loading loading-spinner"></span>
            ) : (
              t("studio.plugins.apply")
            )}
          </button>
        </div>
      )}
    </div>
  );
}
