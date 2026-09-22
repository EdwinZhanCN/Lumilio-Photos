import { useId, useState } from "react";
import {
  ChartNoAxesColumn,
  ChevronDown,
  CircleAlert,
  FolderSync,
  ListChecks,
  RefreshCcw,
  Sparkles,
  Waypoints,
} from "lucide-react";
import { MonitorFrame } from "./MonitorFrame";
import { QueueSummaryList } from "./QueueSummaryList";
import { WorkLaneList, type WorkLane } from "./WorkLaneList";
import { useProcessingMonitor } from "../../api/useProcessingMonitor";
import { useI18n } from "@/lib/i18n.tsx";
import { ProcessingTray } from "../../modules/rive/ProcessingTray";

/**
 * Processing converges on two patterns: one animated tray for files, and one
 * list for every other kind of Catalog work. Queue deliveries stay a separate,
 * explicitly historical diagnostic below both.
 */
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

  const filesPending = processing.pending_assets;
  const filesFailed = processing.failed_assets ?? 0;
  const retryWaiting = processing.retry_waiting_stages ?? 0;

  const lanes: WorkLane[] = [
    {
      key: "repositories",
      icon: FolderSync,
      label: t("monitor.processing.pendingRepositories", "Repositories awaiting scan"),
      description: t(
        "monitor.processing.lanes.repositories",
        "Repositories whose files are being discovered or verified.",
      ),
      pending: processing.pending_repositories,
      failed: processing.failed_repositories,
      to: "/storage",
      linkLabel: t("monitor.processing.openStorage", "Open Storage"),
    },
    {
      key: "analysis",
      icon: Sparkles,
      label: t("monitor.processing.pendingAnalysis", "Files awaiting analysis"),
      description: t(
        "monitor.processing.lanes.analysis",
        "Optional Lumen analysis such as Image Semantic Analysis, OCR Text Recognition, and Person Recognition.",
      ),
      pending: processing.pending_analysis_assets,
      failed: processing.failed_analysis_assets,
      to: "/server-monitor?tab=ml",
      linkLabel: t("monitor.processing.openCoverage", "View coverage"),
    },
    {
      key: "reindex",
      icon: RefreshCcw,
      label: t("monitor.ml.pendingRebuilds", "Reindex requests"),
      description: t(
        "monitor.processing.lanes.reindex",
        "Accepted rebuild requests that have not been applied yet.",
      ),
      pending: processing.pending_reindex_requests,
    },
    {
      key: "projections",
      icon: Waypoints,
      label: t("monitor.processing.pendingProjections", "Pending projection work"),
      description: t(
        "monitor.processing.lanes.projections",
        "Events, places, and text search rebuilt from file facts.",
      ),
      pending: processing.pending_projections,
      failed: processing.failed_projections,
    },
    {
      key: "operations",
      icon: ListChecks,
      label: t("monitor.processing.pendingOperations", "Pending operations"),
      description: t(
        "monitor.processing.lanes.operations",
        "The latest ingest, reindex, and backup operation for each subject.",
      ),
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
      <section aria-labelledby={`${id}-files`} className="card bg-base-100">
        <div className="card-body grid min-h-52 grid-cols-[minmax(0,1fr)_minmax(8rem,34%)] gap-4 p-5 sm:p-6">
          <div className="flex min-w-0 flex-col justify-between gap-5">
            <div>
              <h2 id={`${id}-files`} className="text-base font-semibold text-primary">
                {t("monitor.processing.pendingAssets", "Files awaiting processing")}
              </h2>
              <p className="mt-1 max-w-md text-sm text-base-content/60">
                {t(
                  "monitor.processing.filesDescription",
                  "New or changed files moving through metadata, thumbnails, and transcoding.",
                )}
              </p>
            </div>
            <div>
              <p
                className={
                  filesPending == null
                    ? "text-xl font-semibold"
                    : "text-5xl font-semibold tabular-nums sm:text-6xl"
                }
              >
                {filesPending ?? t("monitor.processing.noData", "No data")}
              </p>
              {filesFailed > 0 && (
                <p className="mt-2 flex items-center gap-1.5 text-sm text-warning">
                  <CircleAlert className="size-4 shrink-0" aria-hidden />
                  {t("monitor.processing.attentionCount", "{{count}} needing attention", {
                    count: filesFailed,
                  })}
                </p>
              )}
              {retryWaiting > 0 && (
                <p className="mt-1 text-sm text-base-content/60">
                  {t("monitor.processing.retryWaiting", {
                    defaultValue: "Stages waiting to retry: {{count}}",
                    count: retryWaiting,
                  })}
                </p>
              )}
            </div>
          </div>
          {filesPending != null && <ProcessingTray pending={filesPending} />}
        </div>
      </section>

      <WorkLaneList lanes={lanes} />

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
              </div>
            )}
          </div>
        }
      />
    </MonitorFrame>
  );
}
