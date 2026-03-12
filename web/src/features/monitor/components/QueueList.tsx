import { Clock, Zap, Loader2, AlertCircle } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { ApiResult, QueueStatsDTO, QueueStatsResponse } from "../monitor.type";
import { useI18n } from "@/lib/i18n.tsx";

export function QueueList() {
  const { t } = useI18n();
  const queuesQuery = $api.useQuery(
    "get",
    "/api/v1/admin/river/queues",
    {},
    {
      refetchInterval: 10000,
      refetchIntervalInBackground: true,
      retry: false,
    },
  );

  const response = queuesQuery.data as ApiResult<QueueStatsResponse> | undefined;
  const queues: QueueStatsDTO[] = response?.data?.queues ?? [];
  const loading = queuesQuery.isLoading;
  const error = queuesQuery.isError ? t("monitor.queues.fetchError") : null;

  if (loading) {
    return (
        <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <Loader2 className="w-8 h-8 animate-spin mx-auto text-primary" />
        <p className="mt-3 text-sm opacity-60">{t("monitor.queues.loading")}</p>
      </div>
    );
  }

  if (error || queues.length === 0) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <AlertCircle className="w-8 h-8 mx-auto text-warning" />
        <div className="text-warning mt-2 text-sm">
          {error || t("monitor.queues.empty")}
        </div>
      </div>
    );
  }

  return (
    <div className="bg-base-100 rounded-lg shadow-sm h-full flex flex-col overflow-hidden">
      {/* Header - Sticky */}
      <div className="sticky top-0 z-10 bg-base-100 p-3 border-b border-base-300">
        <div className="flex items-center justify-between">
          <span className="text-sm font-semibold opacity-60">
            {t("monitor.queues.activeCount", { count: queues.length })}
          </span>
        </div>
      </div>

      {/* Queue List - Scrollable with DaisyUI List */}
      <div className="flex-1 overflow-y-auto">
        <ul className="list">
          {queues.map((queue) => {
            const lastUpdate = queue.updated_at ? new Date(queue.updated_at) : new Date();
            const now = new Date();
            const secondsAgo = Math.floor(
              (now.getTime() - lastUpdate.getTime()) / 1000,
            );

            let timeAgo = "";
            if (secondsAgo < 60) {
              timeAgo = t("monitor.queues.timeAgoSeconds", { count: secondsAgo });
            } else if (secondsAgo < 3600) {
              timeAgo = t("monitor.queues.timeAgoMinutes", {
                count: Math.floor(secondsAgo / 60),
              });
            } else {
              timeAgo = t("monitor.queues.timeAgoHours", {
                count: Math.floor(secondsAgo / 3600),
              });
            }

            // Determine if queue is stale (no update in 5+ minutes)
            const isStale = secondsAgo > 300;
            const isVeryActive = secondsAgo < 30;

            return (
              <li
                key={queue.name}
                className={`list-row hover:bg-base-200/50 transition-all duration-200 ${isStale ? "opacity-60" : ""
                  }`}
              >
                {/* Queue Icon */}
                <div
                  className={`flex-shrink-0 ${isVeryActive ? "text-success" : "text-primary"
                    }`}
                >
                  <Zap className="w-5 h-5" />
                </div>

                {/* Queue Name & Info - grows to fill space */}
                <div className="list-col-grow min-w-0">
                  <h4 className="font-semibold text-sm truncate">
                    {queue.name}
                  </h4>
                  {/* Last Activity */}
                  <div className="flex items-center gap-1 text-xs opacity-60 mt-1">
                    <Clock className="w-3 h-3" />
                    <span>{timeAgo}</span>
                  </div>
                </div>

                {/* Status Badge */}
                {isStale ? (
                  <div className="badge badge-ghost badge-xs gap-1 flex-shrink-0">
                    {t("monitor.queues.idle")}
                  </div>
                ) : (
                  <div className="badge badge-success badge-xs gap-1 flex-shrink-0 animate-pulse">
                    {t("monitor.queues.active")}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      </div>
    </div>
  );
}
