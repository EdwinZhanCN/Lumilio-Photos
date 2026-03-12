import { Blocks, FileText, Plug, SlidersHorizontal } from "lucide-react";
import type { PanelType } from "../routes/Studio";
import { useI18n } from "@/lib/i18n.tsx";
import type { InstalledPluginRecord } from "@/features/studio/plugins/types";

type StudioSidebarProps = {
  activePanel: PanelType;
  setActivePanel: (panel: PanelType) => void;
  pluginRuntimeEnabled: boolean;
  installedPlugins: InstalledPluginRecord[];
  selectedPluginId: string | null;
  onSelectPlugin: (pluginId: string) => void;
  isPluginNavDisabled?: boolean;
};

export function StudioSidebar({
  activePanel,
  setActivePanel,
  pluginRuntimeEnabled,
  installedPlugins,
  selectedPluginId,
  onSelectPlugin,
  isPluginNavDisabled = false,
}: StudioSidebarProps) {
  const { t } = useI18n();
  const navItems = [
    { id: "exif", label: t("studio.nav.exif"), icon: FileText },
    {
      id: "develop",
      label: t("studio.nav.develop"),
      icon: SlidersHorizontal,
    },
    { id: "marketplace", label: t("studio.nav.marketplace"), icon: Blocks },
  ];
  const hasPluginChildren = pluginRuntimeEnabled && installedPlugins.length > 0;
  const isPluginsActive = activePanel === "plugins";

  const handleSelectPlugin = (pluginId: string) => {
    setActivePanel("plugins");
    onSelectPlugin(pluginId);
  };

  return (
    <aside className="w-40 shrink-0 overflow-y-auto border-r border-base-content/10 bg-base-200">
      <div className="select-none mx-2">
        <ul className="menu rounded-box my-2 gap-2 w-full">
          {navItems.map((item) => (
            <li key={item.id}>
              <button
                type="button"
                className={activePanel === item.id ? "menu-active" : undefined}
                onClick={() => setActivePanel(item.id as PanelType)}
              >
                <item.icon className="size-5 flex-shrink-0" />
                {item.label}
              </button>
            </li>
          ))}

          <li>
            {hasPluginChildren ? (
              <details open={isPluginsActive}>
                <summary
                  className={isPluginsActive ? "active" : undefined}
                  onClick={() => setActivePanel("plugins")}
                >
                  <Plug className="size-5 flex-shrink-0" />
                  {t("studio.nav.plugins")}
                </summary>
                <ul>
                  {installedPlugins.map((item) => (
                    <li key={`${item.pluginId}@${item.version}`}>
                      <button
                        type="button"
                        className={
                          selectedPluginId === item.pluginId
                            ? "menu-active"
                            : undefined
                        }
                        onClick={() => handleSelectPlugin(item.pluginId)}
                        disabled={isPluginNavDisabled}
                        title={`${item.pluginId}`}
                      >
                        <span className="truncate">{item.pluginId}</span>
                      </button>
                    </li>
                  ))}
                </ul>
              </details>
            ) : (
              <button
                type="button"
                className={isPluginsActive ? "menu-active" : undefined}
                onClick={() => setActivePanel("plugins")}
              >
                <Plug className="size-5 flex-shrink-0" />
                {t("studio.nav.plugins")}
              </button>
            )}
          </li>
        </ul>
      </div>
    </aside>
  );
}
