import { Activity, CheckCircle2, AlertTriangle, Loader2 } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, JobStatsResponse } from "../monitor.type";

export function StatMonitor() {
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
  const error = statsQuery.isError ? "Failed to fetch queue stats" : null;

  if (loading) {
    return (
      <div className="stats stats-vertical lg:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-title">Loading...</div>
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
          <div className="stat-title">Error</div>
          <div className="stat-value text-error text-lg">
            {error || "No data"}
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
        <div className="stat-title">Active Jobs</div>
        <div className="stat-value text-primary">{activeJobs}</div>
        <div className="stat-desc">
          {(stats.running ?? 0) > 0 && (
            <span className="inline-flex items-center gap-1 text-info">
              <Loader2 className="w-3 h-3 animate-spin" />
              {stats.running} running
            </span>
          )}
          {(stats.running ?? 0) === 0 && (stats.available ?? 0) > 0 && (
            <span className="text-neutral">{stats.available} available</span>
          )}
          {(stats.running ?? 0) === 0 &&
            (stats.available ?? 0) === 0 &&
            (stats.scheduled ?? 0) > 0 && (
              <span className="text-neutral">{stats.scheduled} scheduled</span>
            )}
        </div>
      </div>

      {/* Completed */}
      <div className="stat">
        <div className="stat-figure text-success">
          <CheckCircle2 className="w-8 h-8" />
        </div>
        <div className="stat-title">Completed</div>
        <div className="stat-value text-success">{stats.completed ?? 0}</div>
        <div className="stat-desc">
          {totalProcessed > 0 ? (
            <span>{successRate}% success rate</span>
          ) : (
            <span>No jobs processed yet</span>
          )}
        </div>
      </div>

      {/* Issues */}
      <div className="stat">
        <div className="stat-figure text-warning">
          <AlertTriangle className="w-8 h-8" />
        </div>
        <div className="stat-title">Issues</div>
        <div className="stat-value text-warning">{issueJobs}</div>
        <div className="stat-desc">
          {(stats.retryable ?? 0) > 0 && (
            <span className="text-warning">{stats.retryable} retryable</span>
          )}
          {(stats.retryable ?? 0) > 0 &&
            ((stats.cancelled ?? 0) > 0 || (stats.discarded ?? 0) > 0) && (
              <span className="mx-1">·</span>
            )}
          {(stats.cancelled ?? 0) > 0 && (
            <span className="text-error">{stats.cancelled} cancelled</span>
          )}
          {(stats.cancelled ?? 0) > 0 && (stats.discarded ?? 0) > 0 && (
            <span className="mx-1">·</span>
          )}
          {(stats.discarded ?? 0) > 0 && (
            <span className="text-error">{stats.discarded} discarded</span>
          )}
          {issueJobs === 0 && <span className="text-success">All healthy</span>}
        </div>
      </div>
    </div>
  );
}
