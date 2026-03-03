import {
  Blocks,
  ChevronLeft,
  ChevronRight,
  FileText,
  Plug,
  SlidersHorizontal,
} from "lucide-react";
import { PanelType } from "../routes/Studio";
import { useI18n } from "@/lib/i18n.tsx";

type StudioSidebarProps = {
  activePanel: PanelType;
  setActivePanel: (panel: PanelType) => void;
  isCollapsed: boolean;
  onToggle: () => void;
};

export function StudioSidebar({
  activePanel,
  setActivePanel,
  isCollapsed,
  onToggle,
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
    { id: "plugins", label: t("studio.nav.plugins"), icon: Plug },
  ];

  return (
    <div
      className={`bg-base-200 border-r border-base-content/10 flex flex-col ${isCollapsed ? "w-14" : "w-44"} transition-all duration-300`}
    >
      <div className="p-2">
        <button
          className="btn btn-sm btn-ghost w-full justify-center"
          onClick={onToggle}
        >
          {isCollapsed ? (
            <ChevronRight className="w-5 h-5 flex-shrink-0" />
          ) : (
            <ChevronLeft className="w-5 h-5 flex-shrink-0" />
          )}
        </button>
      </div>
      <div className="p-2">
        {navItems.map((item) => (
          <button
            key={item.id}
            className={`btn btn-sm w-full mb-2 ${activePanel === item.id ? "btn-primary" : "btn-ghost"} ${isCollapsed ? "justify-center px-0" : "justify-start"}`}
            onClick={() => setActivePanel(item.id as PanelType)}
          >
            <item.icon className="w-5 h-5 flex-shrink-0" />
            {!isCollapsed && <span className="ml-2">{item.label}</span>}
          </button>
        ))}
      </div>
    </div>
  );
}
