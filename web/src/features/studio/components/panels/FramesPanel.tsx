import { RectangleGroupIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";
import type {
  CatalogPluginSummary,
  InstalledPluginRecord,
  StudioPluginUiModule,
} from "@/features/studio/plugins/types";

type FramesPanelProps = {
  isGenerating: boolean;
  onGeneratePlugin: () => void;
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
};

export function FramesPanel({
  isGenerating,
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
}: FramesPanelProps) {
  const { t } = useI18n();

  if (!pluginRuntimeEnabled) {
    return (
      <div>
        <div className="flex items-center mb-4">
          <RectangleGroupIcon className="w-5 h-5 mr-2" />
          <h2 className="text-lg font-semibold">{t("studio.frames.title")}</h2>
        </div>
        <div className="rounded-lg bg-base-100 p-4 text-sm text-base-content/70">
          {t("studio.frames.plugin.runtimeDisabled")}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center mb-4">
        <RectangleGroupIcon className="w-5 h-5 mr-2" />
        <h2 className="text-lg font-semibold">{t("studio.frames.title")}</h2>
      </div>

      <div className="space-y-4">
        {catalogLoading && (
          <p className="text-sm text-base-content/70">
            {t("studio.frames.plugin.catalogLoading")}
          </p>
        )}

        {catalogError && (
          <p className="text-sm text-error">{catalogError}</p>
        )}

        {catalogPlugins.length > 0 && (
          <div className="rounded-lg bg-base-100 p-4 space-y-2">
            <h3 className="font-semibold text-sm">{t("studio.frames.plugin.market")}</h3>
            {catalogPlugins.map((plugin) => (
              <div
                key={`${plugin.id}@${plugin.latestVersion}`}
                className="flex items-center justify-between gap-2"
              >
                <div>
                  <p className="text-sm font-medium">{plugin.displayName}</p>
                  <p className="text-xs text-base-content/60">{plugin.id}@{plugin.latestVersion}</p>
                </div>
                {isPluginInstalled(plugin.id, plugin.latestVersion) ? (
                  <button
                    type="button"
                    className="btn btn-xs btn-outline"
                    onClick={() => onUninstallPlugin(plugin.id)}
                    disabled={isGenerating}
                  >
                    {t("studio.frames.plugin.uninstall")}
                  </button>
                ) : (
                  <button
                    type="button"
                    className="btn btn-xs btn-secondary"
                    onClick={() => onInstallPlugin(plugin.id, plugin.latestVersion)}
                    disabled={isGenerating}
                  >
                    {t("studio.frames.plugin.install")}
                  </button>
                )}
              </div>
            ))}
          </div>
        )}

        <div className="rounded-lg bg-base-100 p-4 space-y-3">
          <label className="label">{t("studio.frames.plugin.installed")}</label>
          <select
            className="select select-bordered w-full"
            value={selectedPluginId || ""}
            onChange={(e) => onSelectPlugin(e.target.value)}
            disabled={isGenerating || installedPlugins.length === 0}
          >
            <option value="">{t("studio.frames.plugin.selectPlaceholder")}</option>
            {installedPlugins.map((item) => (
              <option key={`${item.pluginId}@${item.version}`} value={item.pluginId}>
                {item.pluginId}@{item.version}
              </option>
            ))}
          </select>

          {pluginLoading && (
            <p className="text-sm text-base-content/70">
              {t("studio.frames.plugin.loading")}
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
              t("studio.frames.plugin.apply")
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
