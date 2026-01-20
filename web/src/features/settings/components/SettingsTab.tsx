import {
  CursorArrowRippleIcon,
  ServerStackIcon,
  CpuChipIcon,
} from "@heroicons/react/24/solid";
import UISettings from "./Tabs/UISettings";
import ServerSettings from "./Tabs/ServerSettings";
import PerformanceSettings from "./Tabs/PerformanceSettings";
import { useI18n } from "@/lib/i18n.tsx";

export default function SettingsTab() {
  const { t } = useI18n();
  return (
    <div>
      {/* name of each tab group should be unique */}
      <div className="tabs tabs-lift">
        <label className="tab gap-1 cursor-pointer">
          <input type="radio" name="my_tabs_4" defaultChecked />
          <CursorArrowRippleIcon className="size-4" />
          {t("settings.ui")}
        </label>
        <div className="tab-content bg-base-100 border-base-300 p-6">
          <UISettings />
        </div>

        <label className="tab gap-1 cursor-pointer">
          <input type="radio" name="my_tabs_4" />
          <CpuChipIcon className="size-4" />
          Performance
        </label>
        <div className="tab-content bg-base-100 border-base-300 p-6">
          <PerformanceSettings />
        </div>

        <label className="tab gap-1 cursor-pointer">
          <input type="radio" name="my_tabs_4" />
          <ServerStackIcon className="size-4" />
          {t("settings.server")}
        </label>
        <div className="tab-content bg-base-100 border-base-300 p-6">
          <ServerSettings />
        </div>
      </div>
    </div>
  );
}
