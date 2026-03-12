import { Activity, CheckCircle2, AlertTriangle, Loader2 } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, JobStatsResponse } from "../monitor.type";
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

  const response = statsQuery.data as ApiResult<JobStatsResponse> | undefined;
  const stats = response?.data ?? null;
  const loading = statsQuery.isLoading;
  const error = statsQuery.isError ? t("monitor.stats.fetchError") : null;

  if (loading) {
    return (
      <div className="stats stats-vertical lg:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-title">{t("common.loading")}</div>
          <div className="stat-value">
            <span className="loading loading-spinner loading-md"></span>
          </div>
        </div>
      </div>
    );
  }

  if (error || !stats) {
    return (
      <div className="stats stats-vertical lg:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-title">{t("common.error")}</div>
          <div className="stat-value text-error text-lg">
            {error || t("monitor.stats.noData")}
          </div>
        </div>
      </div>
    );
  }

  const activeJobs = (stats.available ?? 0) + (stats.scheduled ?? 0) + (stats.running ?? 0);
  const issueJobs = (stats.retryable ?? 0) + (stats.cancelled ?? 0) + (stats.discarded ?? 0);
  const totalProcessed = (stats.completed ?? 0) + (stats.cancelled ?? 0) + (stats.discarded ?? 0);
  const successRate =
    totalProcessed > 0
      ? (((stats.completed ?? 0) / totalProcessed) * 100).toFixed(1)
      : "0.0";

  return (
    <div className="stats stats-vertical lg:stats-horizontal shadow-sm w-full">
      {/* Active Jobs */}
      <div className="stat">
        <div className="stat-figure text-primary">
          <Activity className="w-8 h-8" />
        </div>
        <div className="stat-title">{t("monitor.stats.activeJobs")}</div>
        <div className="stat-value text-primary">{activeJobs}</div>
        <div className="stat-desc">
          {(stats.running ?? 0) > 0 && (
            <span className="inline-flex items-center gap-1 text-info">
              <Loader2 className="w-3 h-3 animate-spin" />
              {t("monitor.stats.runningCount", { count: stats.running })}
            </span>
          )}
          {(stats.running ?? 0) === 0 && (stats.available ?? 0) > 0 && (
            <span className="text-neutral">
              {t("monitor.stats.availableCount", { count: stats.available })}
            </span>
          )}
          {(stats.running ?? 0) === 0 &&
            (stats.available ?? 0) === 0 &&
            (stats.scheduled ?? 0) > 0 && (
              <span className="text-neutral">
                {t("monitor.stats.scheduledCount", { count: stats.scheduled })}
              </span>
            )}
        </div>
      </div>

      {/* Completed */}
      <div className="stat">
        <div className="stat-figure text-success">
          <CheckCircle2 className="w-8 h-8" />
        </div>
        <div className="stat-title">{t("monitor.stats.completed")}</div>
        <div className="stat-value text-success">{stats.completed ?? 0}</div>
        <div className="stat-desc">
          {totalProcessed > 0 ? (
            <span>{t("monitor.stats.successRate", { rate: successRate })}</span>
          ) : (
            <span>{t("monitor.stats.noProcessedJobs")}</span>
          )}
        </div>
      </div>

      {/* Issues */}
      <div className="stat">
        <div className="stat-figure text-warning">
          <AlertTriangle className="w-8 h-8" />
        </div>
        <div className="stat-title">{t("monitor.stats.issues")}</div>
        <div className="stat-value text-warning">{issueJobs}</div>
        <div className="stat-desc">
          {(stats.retryable ?? 0) > 0 && (
            <span className="text-warning">
              {t("monitor.stats.retryableCount", { count: stats.retryable })}
            </span>
          )}
          {(stats.retryable ?? 0) > 0 &&
            ((stats.cancelled ?? 0) > 0 || (stats.discarded ?? 0) > 0) && (
              <span className="mx-1">·</span>
            )}
          {(stats.cancelled ?? 0) > 0 && (
            <span className="text-error">
              {t("monitor.stats.cancelledCount", { count: stats.cancelled })}
            </span>
          )}
          {(stats.cancelled ?? 0) > 0 && (stats.discarded ?? 0) > 0 && (
            <span className="mx-1">·</span>
          )}
          {(stats.discarded ?? 0) > 0 && (
            <span className="text-error">
              {t("monitor.stats.discardedCount", { count: stats.discarded })}
            </span>
          )}
          {issueJobs === 0 && (
            <span className="text-success">{t("monitor.stats.allHealthy")}</span>
          )}
        </div>
      </div>
    </div>
  );
}
