import { useState } from "react";
import { Activity } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PageHeader from "@/components/ui/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import { useRepositoryOptions } from "@/features/repositories";
import { CapabilitiesMonitor, MLMonitor, StatMonitor, QueueSummaryList } from "../components";

export default function Monitor() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedView = searchParams.get("tab");
  const view = requestedView === "capabilities" || requestedView === "ml" ? requestedView : "queue";

  const [localRepoId, setLocalRepoId] = useState<string | undefined>(undefined);
  const { repositories } = useRepositoryOptions();

  const setView = (nextView: "queue" | "ml" | "capabilities") => {
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
              : t("monitor.subtitles.capabilities")
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
          <div role="tablist" className="tabs tabs-box">
            <button
              role="tab"
              className={`tab ${view === "queue" ? "tab-active" : ""}`}
              onClick={() => setView("queue")}
            >
              {t("monitor.tabs.queue")}
            </button>
            <button
              role="tab"
              className={`tab ${view === "ml" ? "tab-active" : ""}`}
              onClick={() => setView("ml")}
            >
              {t("monitor.tabs.ml")}
            </button>
            <button
              role="tab"
              className={`tab ${view === "capabilities" ? "tab-active" : ""}`}
              onClick={() => setView("capabilities")}
            >
              {t("monitor.tabs.capabilities")}
            </button>
          </div>
        </div>
      </PageHeader>

      <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
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
      </div>
    </div>
  );
}
