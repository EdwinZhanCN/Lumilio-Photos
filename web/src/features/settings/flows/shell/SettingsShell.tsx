import type { ComponentType } from "react";
import { useSearchParams } from "react-router-dom";
import { useAuth } from "@/features/auth";
import { useI18n } from "@/lib/i18n.tsx";
import {
  CloudIcon,
  InfoIcon,
  PaintbrushIcon,
  ServerIcon,
  SparklesIcon,
  UserCircle2Icon,
  Users2Icon,
} from "lucide-react";
import { SettingsPage } from "../../components/SettingsPage";
import AccountTab from "../account/AccountTab";
import AiTab from "../ai/AiTab";
import AppearanceTab from "../appearance/AppearanceTab";
import AboutTab from "../about/AboutTab";
import CloudTab from "../cloud/CloudTab";
import ServerTab from "../server/ServerTab";
import UsersTab from "../users/UsersTab";

type SettingsTabKey = "account" | "appearance" | "ai" | "cloud" | "server" | "users" | "about";

const DEFAULT_TAB: SettingsTabKey = "account";

type SettingsTabIcon = ComponentType<{ className?: string }>;

type SettingsTabMeta = {
  key: SettingsTabKey;
  label: string;
  title: string;
  description: string;
  icon: SettingsTabIcon;
};

export default function SettingsShell() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const isAdmin = user?.role === "admin";

  const tabs: Array<{ key: SettingsTabKey; label: string; icon: SettingsTabIcon }> = [
    {
      key: "account",
      label: t("settings.account.title", { defaultValue: "Account" }),
      icon: UserCircle2Icon,
    },
    { key: "appearance", label: t("settings.appearance"), icon: PaintbrushIcon },
    { key: "server", label: t("settings.server"), icon: ServerIcon },
    { key: "about", label: t("settings.about.title", "About"), icon: InfoIcon },
  ];
  if (isAdmin) {
    tabs.splice(2, 0, { key: "ai", label: t("settings.ai"), icon: SparklesIcon });
    tabs.push({
      key: "cloud",
      label: t("settings.cloud.title", { defaultValue: "Cloud Sync" }),
      icon: CloudIcon,
    });
    tabs.push({
      key: "users",
      label: t("settings.users.title", { defaultValue: "Users" }),
      icon: Users2Icon,
    });
  }

  const tabMeta: Record<SettingsTabKey, SettingsTabMeta> = {
    account: {
      key: "account",
      label: t("settings.account.title", { defaultValue: "Account" }),
      title: t("settings.account.title", { defaultValue: "Account" }),
      description: t("settings.account.pageDescription", {
        defaultValue: "Manage your profile, password, MFA, and passkeys.",
      }),
      icon: UserCircle2Icon,
    },
    appearance: {
      key: "appearance",
      label: t("settings.appearance"),
      title: t("settings.appearance"),
      description: t("settings.pageDescriptions.appearance", {
        defaultValue: "Adjust language, region, theme, and layout preferences.",
      }),
      icon: PaintbrushIcon,
    },
    ai: {
      key: "ai",
      label: t("settings.ai"),
      title: t("settings.ai"),
      description: t("settings.pageDescriptions.ai", {
        defaultValue: "Configure model providers and machine learning features.",
      }),
      icon: SparklesIcon,
    },
    cloud: {
      key: "cloud",
      label: t("settings.cloud.title", { defaultValue: "Cloud Sync" }),
      title: t("settings.cloud.title", { defaultValue: "Cloud Sync" }),
      description: t("settings.cloud.pageDescription", {
        defaultValue: "Manage connected cloud providers and credentials.",
      }),
      icon: CloudIcon,
    },
    server: {
      key: "server",
      label: t("settings.server"),
      title: t("settings.server"),
      description: t("settings.pageDescriptions.server", {
        defaultValue: "Review repository scope, health checks, and runtime details.",
      }),
      icon: ServerIcon,
    },
    users: {
      key: "users",
      label: t("settings.users.title", { defaultValue: "Users" }),
      title: t("settings.users.title", { defaultValue: "Users" }),
      description: t("settings.users.pageDescription", {
        defaultValue: "Inspect and manage user accounts and access.",
      }),
      icon: Users2Icon,
    },
    about: {
      key: "about",
      label: t("settings.about.title", "About"),
      title: t("settings.about.title", "About"),
      description: t("settings.about.pageDescription", {
        defaultValue: "Review legal terms, open-source licenses, and project information.",
      }),
      icon: InfoIcon,
    },
  };

  const requestedTab = searchParams.get("tab");
  const activeTab: SettingsTabKey = tabs.some((tab) => tab.key === requestedTab)
    ? (requestedTab as SettingsTabKey)
    : DEFAULT_TAB;
  const activeTabMeta = tabMeta[activeTab];

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
        return <AppearanceTab />;
      case "server":
        return <ServerTab />;
      case "ai":
        return <AiTab />;
      case "account":
        return <AccountTab />;
      case "cloud":
        return <CloudTab />;
      case "users":
        return <UsersTab />;
      case "about":
        return <AboutTab />;
      default:
        return <AppearanceTab />;
    }
  };

  return (
    <div className="space-y-4 pt-4 pb-4">
      {/* Full-bleed bar so the tab strip background spans the whole width,
          while the tab items stay aligned with the centered content. */}
      <div className="sticky top-0 z-30 w-full bg-base-200">
        <div
          role="tablist"
          aria-label={t("settings.title")}
          className="tabs tabs-box mx-auto max-w-7xl flex-wrap !bg-transparent px-4"
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
              <tab.icon className="size-4" />
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      <div className="mx-auto max-w-7xl px-4">
        <SettingsPage
          icon={<activeTabMeta.icon className="size-5" />}
          title={activeTabMeta.title}
          description={activeTabMeta.description}
        >
          {renderActiveTab()}
        </SettingsPage>
      </div>
    </div>
  );
}
