import { Activity } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import {
  CapabilitiesMonitor,
  MLMonitor,
  StatMonitor,
  TaskMonitor,
  QueueList,
} from "../components";

export default function Monitor() {
  const { t } = useI18n();
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedView = searchParams.get("tab");
  const view =
    requestedView === "capabilities" || requestedView === "ml"
      ? requestedView
      : "queue";

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
          <div className="text-lg font-semibold">
            {t("monitor.adminOnlyTitle")}
          </div>
          <p className="mt-2 text-sm opacity-70">
            {t("monitor.adminOnlyDescription")}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
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
      >
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
      </PageHeader>

      <div className="flex-1 flex flex-col min-h-0 container mx-auto p-4 space-y-4">
        {view === "queue" ? (
          <>
            <StatMonitor />

            <div className="flex-1 grid grid-cols-1 lg:grid-cols-2 gap-4 min-h-0">
              <div className="flex flex-col min-h-0">
                <h2 className="text-lg font-semibold mb-3 px-1">
                  {t("monitor.sections.activeQueues")}
                </h2>
                <div className="flex-1 min-h-0">
                  <QueueList />
                </div>
              </div>

              <div className="flex flex-col min-h-0">
                <h2 className="text-lg font-semibold mb-3 px-1">
                  {t("monitor.sections.recentJobs")}
                </h2>
                <div className="flex-1 min-h-0">
                  <TaskMonitor />
                </div>
              </div>
            </div>
          </>
        ) : view === "ml" ? (
          <MLMonitor />
        ) : (
          <CapabilitiesMonitor />
        )}
      </div>
    </div>
  );
}
