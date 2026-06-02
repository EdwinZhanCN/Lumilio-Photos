import type { ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { useAuth } from "@/features/auth";
import AccountSettings from "./Tabs/AccountSettings";
import UISettings from "./Tabs/UISettings";
import ServerSettings from "./Tabs/ServerSettings";
import AISettings from "./Tabs/AISettings";
import UsersSettings from "./Tabs/UsersSettings";
import CloudSettings from "./Tabs/CloudSettings";
import { useI18n } from "@/lib/i18n.tsx";
import {
  CloudIcon,
  PaintbrushIcon,
  ServerIcon,
  SparklesIcon,
  UserCircle2Icon,
  Users2Icon,
} from "lucide-react";

type SettingsTabKey =
  | "account"
  | "appearance"
  | "ai"
  | "cloud"
  | "server"
  | "users";

const DEFAULT_TAB: SettingsTabKey = "account";

function isSettingsTabKey(value: string | null): value is SettingsTabKey {
  return (
    value === "account" ||
    value === "appearance" ||
    value === "ai" ||
    value === "cloud" ||
    value === "server" ||
    value === "users"
  );
}

export default function SettingsTab() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const isAdmin = user?.role === "admin";

  const tabs: Array<{
    key: SettingsTabKey;
    label: string;
    icon: ReactNode;
  }> = [
    {
      key: "account",
      label: t("settings.account.title", { defaultValue: "Account" }),
      icon: <UserCircle2Icon className="size-4" />,
    },
    {
      key: "appearance",
      label: t("settings.appearance"),
      icon: <PaintbrushIcon className="size-4" />,
    },
    {
      key: "server",
      label: t("settings.server"),
      icon: <ServerIcon className="size-4" />,
    },
  ];
  if (isAdmin) {
    tabs.splice(2, 0, {
      key: "ai",
      label: t("settings.ai"),
      icon: <SparklesIcon className="size-4" />,
    });
    tabs.push({
      key: "cloud",
      label: t("settings.cloud.title", { defaultValue: "Cloud Sync" }),
      icon: <CloudIcon className="size-4" />,
    });
    tabs.push({
      key: "users",
      label: t("settings.users.title", { defaultValue: "Users" }),
      icon: <Users2Icon className="size-4" />,
    });
  }

  const requestedTab = isSettingsTabKey(searchParams.get("tab"))
    ? searchParams.get("tab")
    : DEFAULT_TAB;
  const activeTab = tabs.some((tab) => tab.key === requestedTab)
    ? requestedTab
    : DEFAULT_TAB;

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
      case "account":
        return <AccountSettings />;
      case "appearance":
        return <UISettings />;
      case "ai":
        return <AISettings />;
      case "cloud":
        return <CloudSettings />;
      case "server":
        return <ServerSettings />;
      case "users":
        return <UsersSettings />;
      default:
        return <AccountSettings />;
    }
  };

  return (
    <div className="space-y-4 p-4">
      <div
        role="tablist"
        aria-label={t("settings.title")}
        className="tabs tabs-box flex-wrap sticky top-0 z-30"
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
