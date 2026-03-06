import { Activity } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import {
  CapabilitiesMonitor,
  StatMonitor,
  TaskMonitor,
  QueueList,
} from "../components";

export default function Monitor() {
  const { t } = useI18n();
  const [searchParams, setSearchParams] = useSearchParams();
  const view =
    searchParams.get("tab") === "capabilities" ? "capabilities" : "queue";

  const setView = (nextView: "queue" | "capabilities") => {
    const params = new URLSearchParams(searchParams);

    if (nextView === "queue") {
      params.delete("tab");
    } else {
      params.set("tab", nextView);
    }

    setSearchParams(params, { replace: true });
  };

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title={t("monitor.title")}
        subtitle={
          view === "queue"
            ? t("monitor.subtitles.queue")
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
        ) : (
          <CapabilitiesMonitor />
        )}
      </div>
    </div>
  );
}
