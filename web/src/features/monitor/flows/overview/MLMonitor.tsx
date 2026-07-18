import { useMemo, useState } from "react";
import { Bird, Cpu, Database, Loader2, RefreshCcw, Workflow } from "lucide-react";
import {
  useAssetIndexingStats,
  useRebuildAssetIndexes,
  extractRebuildResponseData,
} from "../../api/useAssetIndexing";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/features/notifications";

interface MLMonitorProps {
  localRepoId?: string;
}

function formatCoveragePercent(coverage: number): string {
  return `${Math.round(coverage * 100)}%`;
}

const ML_TASK_KEYS = ["semantic", "ocr", "face"] as const;
type MLTaskKey = (typeof ML_TASK_KEYS)[number];

function getTaskLabel(t: (key: string) => string, key: MLTaskKey) {
  switch (key) {
    case "semantic":
      return t("settings.aiSettings.taskNames.semantic");
    case "ocr":
      return t("settings.aiSettings.taskNames.ocr");
    case "face":
      return t("settings.aiSettings.taskNames.face");
  }
}

export function MLMonitor({ localRepoId }: MLMonitorProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const statsQuery = useAssetIndexingStats(localRepoId);
  const stats = statsQuery.stats;
  const rebuildMutation = useRebuildAssetIndexes();

  const [reindexModal, setReindexModal] = useState<{
    taskKey: string;
    taskLabel: string;
  } | null>(null);
  const [reindexAll, setReindexAll] = useState(false);

  const taskCards = useMemo(
    () =>
      ML_TASK_KEYS.map((key) => ({
        key,
        label: getTaskLabel(t, key),
        stats: stats?.tasks[key],
      })),
    [stats, t],
  );
  const bioTaskStats = stats?.tasks.bioclip;

  const totalQueuedMLJobs =
    taskCards.reduce((sum, task) => sum + (task.stats?.queuedJobs ?? 0), 0) +
    (bioTaskStats?.queuedJobs ?? 0);
  const rebuildingTasks = rebuildMutation.variables?.body?.tasks ?? [];
  const selectedReindexTask = reindexModal
    ? taskCards.find((task) => task.key === reindexModal.taskKey)
    : undefined;
  const selectedReindexTotal = selectedReindexTask?.stats?.totalCount ?? stats?.photoTotal ?? 0;
  const selectedReindexMissing = Math.max(
    selectedReindexTotal - (selectedReindexTask?.stats?.indexedCount ?? 0),
    0,
  );

  if (statsQuery.isLoading && !stats) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <Loader2 className="w-8 h-8 animate-spin mx-auto text-primary" />
        <p className="mt-3 text-sm opacity-60">{t("common.loading")}</p>
      </div>
    );
  }

  if (statsQuery.isError && !stats) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <div className="text-warning text-sm">{t("monitor.ml.loadError")}</div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => statsQuery.refetch()}
          disabled={statsQuery.isFetching}
        >
          <RefreshCcw className={`w-4 h-4 ${statsQuery.isFetching ? "animate-spin" : ""}`} />
          {t("settings.serverSettings.refresh")}
        </button>
      </div>

      <div className="stats stats-vertical xl:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-figure text-primary">
            <Database className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.existingPhotos")}</div>
          <div className="stat-value text-primary">{stats?.photoTotal ?? 0}</div>
        </div>

        <div className="stat">
          <div className="stat-figure text-info">
            <Cpu className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.queuedMlJobs")}</div>
          <div className="stat-value text-info">{totalQueuedMLJobs}</div>
        </div>

        <div className="stat">
          <div className="stat-figure text-secondary">
            <Workflow className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.reindexJobs")}</div>
          <div className="stat-value text-secondary">{stats?.reindexJobs ?? 0}</div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        {taskCards.map(({ key, label, stats: taskStats }) => {
          const indexedCount = taskStats?.indexedCount ?? 0;
          const queuedJobs = taskStats?.queuedJobs ?? 0;
          const totalCount = taskStats?.totalCount ?? stats?.photoTotal ?? 0;
          const coverage = taskStats?.coverage ?? 0;
          const remaining = Math.max(totalCount - indexedCount, 0);

          return (
            <section key={key} className="bg-base-100 rounded-lg shadow-sm p-4 space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold">{label}</h2>
                  <p className="text-sm text-base-content/70">
                    {t("monitor.ml.coverageValue", {
                      indexed: indexedCount,
                      total: totalCount,
                      percent: formatCoveragePercent(coverage),
                    })}
                  </p>
                </div>
                <span className="badge badge-outline">{formatCoveragePercent(coverage)}</span>
              </div>

              <progress
                className="progress progress-primary w-full"
                value={Math.round(coverage * 100)}
                max="100"
              />

              <div className="grid grid-cols-3 gap-3 text-sm">
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.indexedAssets")}</div>
                  <div className="mt-1 font-semibold">{indexedCount}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.remainingAssets")}</div>
                  <div className="mt-1 font-semibold">{remaining}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.queuedJobs")}</div>
                  <div className="mt-1 font-semibold">{queuedJobs}</div>
                </div>
              </div>

              <div className="flex items-center gap-2">
                <button
                  className="btn btn-ghost btn-sm"
                  onClick={() => {
                    setReindexAll(remaining === 0);
                    setReindexModal({ taskKey: key, taskLabel: label });
                  }}
                  disabled={rebuildMutation.isPending && rebuildingTasks.includes(key)}
                >
                  <RefreshCcw
                    className={`w-4 h-4 ${
                      rebuildMutation.isPending && rebuildingTasks.includes(key)
                        ? "animate-spin"
                        : ""
                    }`}
                  />
                  {t("monitor.ml.reindex")}
                </button>
                {remaining === 0 && (
                  <span className="text-xs text-base-content/40">
                    {t("monitor.ml.allIndexed", "All indexed")}
                  </span>
                )}
              </div>
            </section>
          );
        })}

        {(() => {
          const indexedCount = bioTaskStats?.indexedCount ?? 0;
          const queuedJobs = bioTaskStats?.queuedJobs ?? 0;
          const totalCount = bioTaskStats?.totalCount ?? 0;
          const coverage = bioTaskStats?.coverage ?? 0;
          const remaining = Math.max(totalCount - indexedCount, 0);

          return (
            <section className="bg-base-100 rounded-lg shadow-sm p-4 space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold flex items-center gap-2">
                    <Bird className="size-5 text-primary" />
                    {t("monitor.ml.bioAlbumCoverage")}
                  </h2>
                  <p className="text-sm text-base-content/70">
                    {t("monitor.ml.coverageValue", {
                      indexed: indexedCount,
                      total: totalCount,
                      percent: formatCoveragePercent(coverage),
                    })}
                  </p>
                </div>
                <span className="badge badge-outline">{formatCoveragePercent(coverage)}</span>
              </div>

              <progress
                className="progress progress-primary w-full"
                value={Math.round(coverage * 100)}
                max="100"
              />

              <div className="grid grid-cols-3 gap-3 text-sm">
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.indexedAssets")}</div>
                  <div className="mt-1 font-semibold">{indexedCount}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.remainingAssets")}</div>
                  <div className="mt-1 font-semibold">{remaining}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">{t("monitor.ml.queuedJobs")}</div>
                  <div className="mt-1 font-semibold">{queuedJobs}</div>
                </div>
              </div>

              <p className="text-xs text-base-content/60">{t("monitor.ml.bioAlbumHint")}</p>
            </section>
          );
        })()}
      </div>

      {reindexModal && (
        <div className="modal modal-open">
          <div className="modal-box max-w-sm">
            <h3 className="font-semibold text-lg">
              {t("monitor.ml.reindexModal.title", {
                task: reindexModal.taskLabel,
              })}
            </h3>
            <p className="py-4 text-sm text-base-content/70">
              {reindexAll
                ? t("monitor.ml.reindexModal.descriptionAll", {
                    count: selectedReindexTotal,
                  })
                : t("monitor.ml.reindexModal.descriptionMissing", {
                    count: selectedReindexMissing,
                  })}
            </p>
            {stats?.reindexJobs != null && stats.reindexJobs > 0 && (
              <div className="alert alert-warning mb-4 py-2 text-sm">
                {t("monitor.ml.reindexModal.existingJobsWarning", {
                  count: stats.reindexJobs,
                })}
              </div>
            )}
            <div className="form-control">
              <label className="label cursor-pointer justify-start gap-3">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm"
                  checked={reindexAll}
                  onChange={(e) => setReindexAll(e.target.checked)}
                />
                <span className="label-text">
                  {t("monitor.ml.reindexModal.reindexAllCheckbox")}
                </span>
              </label>
            </div>
            <div className="modal-action">
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => {
                  setReindexModal(null);
                  setReindexAll(false);
                }}
              >
                {t("monitor.ml.reindexModal.cancel")}
              </button>
              <button
                className="btn btn-primary btn-sm"
                onClick={async () => {
                  const result = await rebuildMutation.mutateAsync({
                    body: {
                      repository_id: localRepoId || undefined,
                      tasks: [reindexModal.taskKey],
                      missing_only: !reindexAll,
                    },
                  });
                  setReindexModal(null);
                  setReindexAll(false);

                  const data = extractRebuildResponseData(result);
                  const disabled = data?.disabled_tasks;
                  if (disabled && disabled.length > 0) {
                    const taskNames = disabled
                      .map((key) => getTaskLabel(t, key as MLTaskKey))
                      .join(", ");
                    showMessage(
                      "info",
                      t("monitor.ml.reindexModal.disabledTasksWarning", {
                        tasks: taskNames,
                      }),
                    );
                  }
                }}
              >
                {t("monitor.ml.reindexModal.confirm")}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
