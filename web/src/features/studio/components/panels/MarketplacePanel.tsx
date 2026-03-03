import { useMemo, useState } from "react";
import { Blocks } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type {
  CatalogPluginSummary,
  InstalledPluginRecord,
} from "@/features/studio/plugins/types";

type MarketplacePanelProps = {
  isGenerating: boolean;
  pluginRuntimeEnabled: boolean;
  installedPlugins: InstalledPluginRecord[];
  catalogPlugins: CatalogPluginSummary[];
  onInstallPlugin: (pluginId: string, version: string) => void;
  onUninstallPlugin: (pluginId: string) => void;
  isPluginInstalled: (pluginId: string, version?: string) => boolean;
  catalogLoading: boolean;
  catalogError: string | null;
};

function matchesSearch(
  plugin: CatalogPluginSummary,
  searchText: string,
): boolean {
  if (!searchText) return true;
  const query = searchText.trim().toLowerCase();
  if (!query) return true;

  return (
    plugin.displayName.toLowerCase().includes(query) ||
    plugin.id.toLowerCase().includes(query) ||
    (plugin.description || "").toLowerCase().includes(query)
  );
}

export function MarketplacePanel({
  isGenerating,
  pluginRuntimeEnabled,
  installedPlugins,
  catalogPlugins,
  onInstallPlugin,
  onUninstallPlugin,
  isPluginInstalled,
  catalogLoading,
  catalogError,
}: MarketplacePanelProps) {
  const { t } = useI18n();
  const [searchText, setSearchText] = useState("");
  const [installedOnly, setInstalledOnly] = useState(false);

  const filteredPlugins = useMemo(() => {
    return catalogPlugins.filter((plugin) => {
      if (!matchesSearch(plugin, searchText)) {
        return false;
      }

      if (!installedOnly) {
        return true;
      }

      return isPluginInstalled(plugin.id, plugin.latestVersion);
    });
  }, [catalogPlugins, searchText, installedOnly, isPluginInstalled]);

  if (!pluginRuntimeEnabled) {
    return (
      <div>
        <div className="flex items-center mb-4">
          <Blocks className="w-5 h-5 mr-2" />
          <h2 className="text-lg font-semibold">{t("studio.marketplace.title")}</h2>
        </div>
        <div className="rounded-lg bg-base-100 p-4 text-sm text-base-content/70">
          {t("studio.marketplace.runtimeDisabled")}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center mb-4">
        <Blocks className="w-5 h-5 mr-2" />
        <h2 className="text-lg font-semibold">{t("studio.marketplace.title")}</h2>
      </div>

      <div className="space-y-4">
        <div className="rounded-lg bg-base-100 p-4 space-y-3">
          <input
            type="text"
            className="input input-bordered w-full"
            placeholder={t("studio.marketplace.searchPlaceholder")}
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
          />

          <label className="label cursor-pointer justify-start gap-2">
            <input
              type="checkbox"
              className="checkbox checkbox-sm"
              checked={installedOnly}
              onChange={(event) => setInstalledOnly(event.target.checked)}
            />
            <span className="text-sm">{t("studio.marketplace.installedOnly")}</span>
          </label>

          <p className="text-xs text-base-content/60">
            {t("studio.marketplace.summary", {
              visible: filteredPlugins.length,
              total: catalogPlugins.length,
              installed: installedPlugins.length,
            })}
          </p>
        </div>

        {catalogLoading && (
          <p className="text-sm text-base-content/70">
            {t("studio.marketplace.catalogLoading")}
          </p>
        )}

        {catalogError && <p className="text-sm text-error">{catalogError}</p>}

        {!catalogLoading && filteredPlugins.length === 0 && (
          <div className="rounded-lg bg-base-100 p-4 text-sm text-base-content/70">
            {t("studio.marketplace.empty")}
          </div>
        )}

        {filteredPlugins.length > 0 && (
          <div className="space-y-3">
            {filteredPlugins.map((plugin) => {
              const installed = isPluginInstalled(plugin.id, plugin.latestVersion);
              return (
                <div
                  key={`${plugin.id}@${plugin.latestVersion}`}
                  className="rounded-lg bg-base-100 p-4 space-y-2"
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <p className="text-sm font-medium">{plugin.displayName}</p>
                      <p className="text-xs text-base-content/60">
                        {plugin.id}@{plugin.latestVersion}
                      </p>
                    </div>
                    <span className="badge badge-outline text-[10px]">
                      {installed
                        ? t("studio.marketplace.statusInstalled")
                        : t("studio.marketplace.statusNotInstalled")}
                    </span>
                  </div>

                  {plugin.description && (
                    <p className="text-xs text-base-content/70">{plugin.description}</p>
                  )}

                  <div className="pt-1">
                    {installed ? (
                      <button
                        type="button"
                        className="btn btn-xs btn-outline"
                        onClick={() => onUninstallPlugin(plugin.id)}
                        disabled={isGenerating}
                      >
                        {t("studio.marketplace.uninstall")}
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="btn btn-xs btn-secondary"
                        onClick={() => onInstallPlugin(plugin.id, plugin.latestVersion)}
                        disabled={isGenerating}
                      >
                        {t("studio.marketplace.install")}
                      </button>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
