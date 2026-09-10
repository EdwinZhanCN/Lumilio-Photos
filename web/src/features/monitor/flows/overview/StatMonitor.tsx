import { Activity, AlertTriangle, FolderSync, Layers } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";

export function StatMonitor() {
  const { t } = useI18n();
  const statsQuery = $api.useQuery(
    "get",
    "/api/v1/admin/river/stats",
    {},
    {
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      retry: false,
    },
  );
  const stats = statsQuery.data;
  const processing = stats?.processing;

  if (statsQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 p-4">
        <span className="loading loading-spinner loading-sm" />
        {t("common.loading")}
      </div>
    );
  }
  if (statsQuery.isError || !stats || !processing) {
    return (
      <div role="alert" className="alert alert-error">
        {t("monitor.stats.fetchError")}
      </div>
    );
  }

  const activeDeliveries =
    (stats.available ?? 0) + (stats.scheduled ?? 0) + (stats.running ?? 0) + (stats.retryable ?? 0);
  const cards = [
    {
      title: t("monitor.processing.pendingAssets", "Files awaiting processing"),
      value: processing.pending_assets ?? 0,
      description: t("monitor.processing.retryWaiting", {
        defaultValue: "Stages waiting to retry: {{count}}",
        count: processing.retry_waiting_stages ?? 0,
      }),
      Icon: Activity,
    },
    {
      title: t("monitor.processing.failedAssets", "Files needing attention"),
      value: processing.failed_assets ?? 0,
      description: t(
        "monitor.processing.retryHint",
        "Retry these files after resolving the cause.",
      ),
      Icon: AlertTriangle,
    },
    {
      title: t("monitor.processing.pendingRepositories", "Repositories awaiting scan"),
      value: processing.pending_repositories ?? 0,
      description: t("monitor.processing.failedRepositories", {
        defaultValue: "Scans needing attention: {{count}}",
        count: processing.failed_repositories ?? 0,
      }),
      Icon: FolderSync,
    },
    {
      title: t("monitor.processing.pendingProjections", "Pending projection work"),
      value: processing.pending_projections ?? 0,
      description: t("monitor.processing.failedProjections", {
        defaultValue: "Projections needing attention: {{count}}",
        count: processing.failed_projections ?? 0,
      }),
      Icon: Layers,
    },
  ];

  return (
    <div className="space-y-4">
      <dl className="stats stats-vertical xl:stats-horizontal shadow-sm w-full">
        {cards.map(({ title, value, description, Icon }) => (
          <div className="stat min-w-0" key={title}>
            <div className="stat-figure text-primary">
              <Icon className="h-6 w-6" />
            </div>
            <dt className="stat-title whitespace-normal">{title}</dt>
            <dd className="stat-value">{value}</dd>
            <dd className="stat-desc whitespace-normal">{description}</dd>
          </div>
        ))}
      </dl>
      <p className="text-sm opacity-70">
        {t("monitor.processing.operations", {
          defaultValue:
            "Import, reindex and backup operations: {{pending}} pending · {{failed}} needing attention",
          pending: processing.pending_operations ?? 0,
          failed: processing.failed_operations ?? 0,
        })}
      </p>
      <div className="rounded-box border border-base-300 p-4 space-y-2">
        <h2 className="font-semibold">{t("monitor.delivery.title", "Queue delivery records")}</h2>
        <p className="text-sm opacity-70">
          {t(
            "monitor.delivery.description",
            "One file or scan can produce many deliveries. These records do not measure completed files or current file failures.",
          )}
        </p>
        <dl className="flex flex-wrap gap-x-8 gap-y-2 text-sm">
          <div>
            <dt className="inline">{t("monitor.delivery.active", "Active deliveries")}: </dt>
            <dd className="inline font-medium">{activeDeliveries}</dd>
          </div>
          <div>
            <dt className="inline">{t("monitor.delivery.completed", "Completed deliveries")}: </dt>
            <dd className="inline font-medium">{stats.completed ?? 0}</dd>
          </div>
          <div>
            <dt className="inline">{t("monitor.delivery.discarded", "Discarded deliveries")}: </dt>
            <dd className="inline font-medium">{stats.discarded ?? 0}</dd>
          </div>
          <div>
            <dt className="inline">{t("monitor.delivery.cancelled", "Cancelled deliveries")}: </dt>
            <dd className="inline font-medium">{stats.cancelled ?? 0}</dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
