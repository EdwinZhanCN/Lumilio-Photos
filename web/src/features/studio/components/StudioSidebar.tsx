import {
  DocumentTextIcon,
  AdjustmentsHorizontalIcon,
  RectangleGroupIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
} from "@heroicons/react/24/outline";
import { PanelType } from "../routes/Studio";

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
  const navItems = [
    { id: "exif", label: "EXIF", icon: DocumentTextIcon },
    { id: "develop", label: "Develop", icon: AdjustmentsHorizontalIcon },
    { id: "frames", label: "Frames", icon: RectangleGroupIcon },
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
            <ArrowRightIcon className="w-5 h-5 flex-shrink-0" />
          ) : (
            <ArrowLeftIcon className="w-5 h-5 flex-shrink-0" />
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
