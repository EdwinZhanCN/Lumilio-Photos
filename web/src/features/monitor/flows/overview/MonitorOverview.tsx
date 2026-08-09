import { useState } from "react";
import { Activity } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PageHeader from "@/components/ui/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import { useRepositoryOptions } from "@/features/repositories";
import { CapabilitiesMonitor } from "./CapabilitiesMonitor";
import { LifecycleHistory } from "./LifecycleHistory";
import { MLMonitor } from "./MLMonitor";
import { QueueSummaryList } from "./QueueSummaryList";
import { StatMonitor } from "./StatMonitor";
import { StorageMonitor } from "./StorageMonitor";

type MonitorView = "queue" | "ml" | "capabilities" | "storage";

export default function MonitorOverview() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedView = searchParams.get("tab");
  const view: MonitorView =
    requestedView === "capabilities" || requestedView === "ml" || requestedView === "storage"
      ? requestedView
      : "queue";

  const [localRepoId, setLocalRepoId] = useState<string | undefined>(undefined);
  const { repositories } = useRepositoryOptions();

  const setView = (nextView: MonitorView) => {
    const params = new URLSearchParams(searchParams);

    if (nextView === "queue") {
      params.delete("tab");
    } else {
      params.set("tab", nextView);
    }

    setSearchParams(params, { replace: true });
  };

  if (user?.role !== "admin") {
    return (
      <div className="flex h-full items-center justify-center p-6">
        <div className="rounded-3xl border border-base-300 bg-base-100 p-8 text-center shadow-sm">
          <div className="text-lg font-semibold">{t("monitor.adminOnlyTitle")}</div>
          <p className="mt-2 text-sm opacity-70">{t("monitor.adminOnlyDescription")}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title={t("monitor.title")}
        subtitle={
          view === "queue"
            ? t("monitor.subtitles.queue")
            : view === "ml"
              ? t("monitor.subtitles.ml")
              : view === "capabilities"
                ? t("monitor.subtitles.capabilities")
                : t(
                    "monitor.subtitles.storage",
                    "Storage Location, repository, capacity, mount, and lifecycle health.",
                  )
        }
        icon={<Activity className="w-6 h-6 text-primary" />}
        className="flex-wrap gap-y-3"
      >
        <div className="flex flex-wrap items-center justify-end gap-2">
          {view === "ml" && (
            <select
              className="select select-bordered select-sm w-48"
              value={localRepoId ?? ""}
              onChange={(e) => setLocalRepoId(e.target.value || undefined)}
            >
              <option value="">
                {t("navbar.repository.all", {
                  defaultValue: "All repositories",
                })}
              </option>
              {repositories.map((repo) => (
                <option key={repo.id} value={repo.id}>
                  {repo.name || repo.path}
                </option>
              ))}
            </select>
          )}
          <div role="tablist" className="tabs tabs-box tabs-sm" aria-label={t("monitor.title")}>
            <button
              type="button"
              role="tab"
              aria-selected={view === "queue"}
              className={`tab ${view === "queue" ? "tab-active" : ""}`}
              onClick={() => setView("queue")}
            >
              {t("monitor.tabs.queue")}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={view === "ml"}
              className={`tab ${view === "ml" ? "tab-active" : ""}`}
              onClick={() => setView("ml")}
            >
              {t("monitor.tabs.ml")}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={view === "capabilities"}
              className={`tab ${view === "capabilities" ? "tab-active" : ""}`}
              onClick={() => setView("capabilities")}
            >
              {t("monitor.tabs.capabilities")}
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={view === "storage"}
              className={`tab ${view === "storage" ? "tab-active" : ""}`}
              onClick={() => setView("storage")}
            >
              {t("monitor.tabs.storage", "Storage")}
            </button>
          </div>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
        {view === "storage" ? (
          <>
            {/* 主从区按内容自然增高；Lifecycle 始终排在完整主从区之后 */}
            <div className="container mx-auto w-full p-4 pb-3">
              <StorageMonitor />
            </div>
            <div className="container mx-auto w-full p-4 pt-0 pb-6">
              <LifecycleHistory />
            </div>
          </>
        ) : (
          <div className="container mx-auto w-full space-y-4 p-4 pb-6">
            {view === "queue" ? (
              <>
                <StatMonitor />

                <QueueSummaryList />
              </>
            ) : view === "ml" ? (
              <MLMonitor localRepoId={localRepoId} />
            ) : (
              <CapabilitiesMonitor />
            )}
          </div>
        )}
      </div>
    </div>
  );
}
