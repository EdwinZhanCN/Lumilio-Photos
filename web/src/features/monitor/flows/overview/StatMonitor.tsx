import { useId, useState } from "react";
import { ChartNoAxesColumn, ChevronDown, CircleAlert } from "lucide-react";
import { MonitorFrame } from "./MonitorFrame";
import { QueueSummaryList } from "./QueueSummaryList";
import { useProcessingMonitor } from "../../api/useProcessingMonitor";
import { useI18n } from "@/lib/i18n.tsx";
import { type WorkType } from "../../model/processingProgress";
import { ProcessingTray } from "../../modules/rive/ProcessingTray";

/** Current Catalog work stays separate from historical queue deliveries. */
export function StatMonitor() {
  const { t } = useI18n();
  const id = useId();
  const [showStatistics, setShowStatistics] = useState(false);
  const statsQuery = useProcessingMonitor();
  const stats = statsQuery.data?.deliveries;
  const processing = statsQuery.data?.processing;
  const refresh = () => void statsQuery.refetch();
  const frame = {
    isLoading: statsQuery.isLoading,
    isFetching: statsQuery.isFetching,
    hasData: !!processing,
    error:
      statsQuery.isError || (!statsQuery.isLoading && !processing)
        ? t("monitor.stats.fetchError")
        : undefined,
    onRefresh: refresh,
    updatedAt: statsQuery.dataUpdatedAt,
  };
  if (!stats || !processing) return <MonitorFrame {...frame} />;

  const trays: {
    type: WorkType;
    label: string;
    pending: number | undefined;
    failed: number | undefined;
  }[] = [
    {
      type: "assets",
      label: t("monitor.processing.pendingAssets", "Files awaiting processing"),
      pending: processing.pending_assets,
      failed: processing.failed_assets,
    },
    {
      type: "repositories",
      label: t("monitor.processing.pendingRepositories", "Repositories awaiting scan"),
      pending: processing.pending_repositories,
      failed: processing.failed_repositories,
    },
    {
      type: "projections",
      label: t("monitor.processing.pendingProjections", "Pending projection work"),
      pending: processing.pending_projections,
      failed: processing.failed_projections,
    },
    {
      type: "operations",
      label: t("monitor.processing.pendingOperations", "Pending operations"),
      pending: processing.pending_operations,
      failed: processing.failed_operations,
    },
  ];
  const deliveries = [
    [
      t("monitor.delivery.active", "Active deliveries"),
      (stats.available ?? 0) +
        (stats.scheduled ?? 0) +
        (stats.running ?? 0) +
        (stats.retryable ?? 0),
    ],
    [t("monitor.delivery.completed", "Completed deliveries"), stats.completed ?? 0],
    [t("monitor.delivery.discarded", "Discarded deliveries"), stats.discarded ?? 0],
    [t("monitor.delivery.cancelled", "Cancelled deliveries"), stats.cancelled ?? 0],
  ] as const;

  return (
    <MonitorFrame {...frame}>
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
        {trays.map((tray) => (
          <section
            key={tray.type}
            aria-labelledby={`${id}-${tray.type}`}
            className="card bg-base-100"
          >
            <div className="card-body grid min-h-44 grid-cols-[minmax(0,1fr)_38%] gap-3 p-5 sm:min-h-52 sm:p-6">
              <div className="flex min-w-0 flex-col justify-between gap-5">
                <h2 id={`${id}-${tray.type}`} className="text-base font-semibold text-primary">
                  {tray.label}
                </h2>
                <div>
                  <p
                    className={
                      tray.pending == null
                        ? "text-xl font-semibold"
                        : "text-4xl font-semibold tabular-nums sm:text-5xl"
                    }
                  >
                    {tray.pending ?? t("monitor.processing.noData", "No data")}
                  </p>
                  {(tray.failed ?? 0) > 0 && (
                    <p className="mt-2 flex items-center gap-1.5 text-sm text-warning">
                      <CircleAlert className="size-4 shrink-0" aria-hidden />
                      {t("monitor.processing.attentionCount", "{{count}} needing attention", {
                        count: tray.failed,
                      })}
                    </p>
                  )}
                </div>
              </div>
              {tray.pending != null && <ProcessingTray type={tray.type} pending={tray.pending} />}
            </div>
          </section>
        ))}
      </div>

      <QueueSummaryList
        queues={stats.queues ?? []}
        actions={
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            aria-expanded={showStatistics}
            aria-controls={`${id}-statistics`}
            onClick={() => setShowStatistics((open) => !open)}
          >
            <ChartNoAxesColumn className="size-4" aria-hidden />
            {t("monitor.delivery.details", "Statistics")}
            <ChevronDown className={`size-4 ${showStatistics ? "rotate-180" : ""}`} aria-hidden />
          </button>
        }
        details={
          <div id={`${id}-statistics`} hidden={!showStatistics}>
            {showStatistics && (
              <div className="rounded-box bg-base-100 p-5 sm:p-6">
                <dl className="grid grid-cols-2 gap-5 sm:grid-cols-4">
                  {deliveries.map(([label, value]) => (
                    <div key={label}>
                      <dt className="text-sm text-base-content/60">{label}</dt>
                      <dd className="mt-1 text-lg font-semibold tabular-nums">{value}</dd>
                    </div>
                  ))}
                </dl>
                <p className="mt-4 text-sm text-base-content/60">
                  {t(
                    "monitor.delivery.description",
                    "One file or scan can produce many deliveries. These records do not measure completed files or current file failures.",
                  )}
                </p>
                {(processing.retry_waiting_stages ?? 0) > 0 && (
                  <p className="mt-2 text-sm">
                    {t("monitor.processing.retryWaiting", {
                      defaultValue: "Stages waiting to retry: {{count}}",
                      count: processing.retry_waiting_stages,
                    })}
                  </p>
                )}
              </div>
            )}
          </div>
        }
      />
    </MonitorFrame>
  );
}
