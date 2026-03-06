import {
  PaintBrushIcon,
  ArrowUpTrayIcon,
  ServerStackIcon,
  CpuChipIcon,
  SparklesIcon,
} from "@heroicons/react/24/outline";
import type { ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import UISettings from "./Tabs/UISettings";
import UploadSettings from "./Tabs/UploadSettings";
import ServerSettings from "./Tabs/ServerSettings";
import PerformanceSettings from "./Tabs/PerformanceSettings";
import AISettings from "./Tabs/AISettings";
import { useI18n } from "@/lib/i18n.tsx";

type SettingsTabKey = "appearance" | "upload" | "ai" | "performance" | "server";

const DEFAULT_TAB: SettingsTabKey = "appearance";

function isSettingsTabKey(value: string | null): value is SettingsTabKey {
  return (
    value === "appearance" ||
    value === "upload" ||
    value === "ai" ||
    value === "performance" ||
    value === "server"
  );
}

export default function SettingsTab() {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = isSettingsTabKey(searchParams.get("tab"))
    ? searchParams.get("tab")
    : DEFAULT_TAB;

  const tabs: Array<{
    key: SettingsTabKey;
    label: string;
    icon: ReactNode;
  }> = [
    {
      key: "appearance",
      label: t("settings.appearance"),
      icon: <PaintBrushIcon className="size-4" />,
    },
    {
      key: "upload",
      label: t("settings.upload"),
      icon: <ArrowUpTrayIcon className="size-4" />,
    },
    {
      key: "ai",
      label: t("settings.ai"),
      icon: <SparklesIcon className="size-4" />,
    },
    {
      key: "performance",
      label: t("settings.performance"),
      icon: <CpuChipIcon className="size-4" />,
    },
    {
      key: "server",
      label: t("settings.server"),
      icon: <ServerStackIcon className="size-4" />,
    },
  ];

  const setActiveTab = (nextTab: SettingsTabKey) => {
    const nextParams = new URLSearchParams(searchParams);
    if (nextTab === DEFAULT_TAB) {
      nextParams.delete("tab");
    } else {
      nextParams.set("tab", nextTab);
    }
    setSearchParams(nextParams, { replace: true });
  };

  const renderActiveTab = () => {
    switch (activeTab) {
      case "appearance":
        return <UISettings />;
      case "upload":
        return <UploadSettings />;
      case "ai":
        return <AISettings />;
      case "performance":
        return <PerformanceSettings />;
      case "server":
        return <ServerSettings />;
      default:
        return <UISettings />;
    }
  };

  return (
    <div className="space-y-4">
      <div
        role="tablist"
        aria-label={t("settings.title")}
        className="tabs tabs-box flex-wrap"
      >
        {tabs.map((tab) => (
          <button
            key={tab.key}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.key}
            className={`tab gap-2 ${activeTab === tab.key ? "tab-active" : ""}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      <div className="rounded-2xl border border-base-300 bg-base-100 p-6">
        {renderActiveTab()}
      </div>
    </div>
  );
}
