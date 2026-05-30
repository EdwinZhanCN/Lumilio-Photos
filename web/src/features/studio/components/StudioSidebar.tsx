import { FileText, Frame, SlidersHorizontal } from "lucide-react";
import type { PanelType } from "../routes/Studio";
import { useI18n } from "@/lib/i18n.tsx";

type StudioSidebarProps = {
  activePanel: PanelType;
  setActivePanel: (panel: PanelType) => void;
};

export function StudioSidebar({
  activePanel,
  setActivePanel,
}: StudioSidebarProps) {
  const { t } = useI18n();
  const navItems = [
    { id: "exif", label: t("studio.nav.exif"), icon: FileText },
    {
      id: "develop",
      label: t("studio.nav.develop"),
      icon: SlidersHorizontal,
    },
    { id: "border", label: t("studio.nav.border", "Border"), icon: Frame },
  ];

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
        </ul>
      </div>
    </aside>
  );
}
