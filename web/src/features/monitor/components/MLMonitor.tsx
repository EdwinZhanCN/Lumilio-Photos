import { useMemo } from "react";
import { Cpu, Database, Loader2, RefreshCcw, Workflow } from "lucide-react";
import { useAssetIndexingStats } from "@/features/settings/hooks/useAssetIndexing";
import { useI18n } from "@/lib/i18n.tsx";
import { useWorkingRepository } from "@/features/settings";

function formatCoveragePercent(coverage: number): string {
  return `${Math.round(coverage * 100)}%`;
}

export function MLMonitor() {
  const { t } = useI18n();
  const {
    repositoriesQuery,
    scopedRepositoryId,
    scopeLabel,
    scopeDescription,
  } = useWorkingRepository();
  const statsQuery = useAssetIndexingStats(scopedRepositoryId);
  const stats = statsQuery.stats;

  const taskCards = useMemo(
    () => [
      {
        key: "clip",
        label: t("settings.aiSettings.taskNames.clip"),
        stats: stats?.tasks.clip,
      },
      {
        key: "ocr",
        label: t("settings.aiSettings.taskNames.ocr"),
        stats: stats?.tasks.ocr,
      },
      {
        key: "caption",
        label: t("settings.aiSettings.taskNames.caption"),
        stats: stats?.tasks.caption,
      },
      {
        key: "face",
        label: t("settings.aiSettings.taskNames.face"),
        stats: stats?.tasks.face,
      },
    ],
    [stats, t],
  );

  const totalQueuedMLJobs = taskCards.reduce(
    (sum, task) => sum + (task.stats?.queuedJobs ?? 0),
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
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div className="space-y-2 max-w-xl">
          <span className="font-semibold">
            {t("settings.aiSettings.repositoryScopeLabel")}
          </span>
          <div className="rounded-xl border border-base-300 bg-base-100 px-4 py-3">
            <div className="font-medium">{scopeLabel}</div>
            <div className="mt-1 text-sm text-base-content/70">
              {scopeDescription}
            </div>
          </div>
        </div>

        <button
          className="btn btn-ghost btn-sm self-start lg:self-auto"
          onClick={() =>
            void Promise.all([
              repositoriesQuery.refetch(),
              statsQuery.refetch(),
            ])
          }
          disabled={repositoriesQuery.isFetching || statsQuery.isFetching}
        >
          <RefreshCcw
            className={`w-4 h-4 ${repositoriesQuery.isFetching || statsQuery.isFetching ? "animate-spin" : ""}`}
          />
          {t("settings.serverSettings.refresh")}
        </button>
      </div>

      <div className="stats stats-vertical xl:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-figure text-primary">
            <Database className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.existingPhotos")}</div>
          <div className="stat-value text-primary">
            {stats?.photoTotal ?? 0}
          </div>
          <div className="stat-desc">{t("monitor.ml.photoScope")}</div>
        </div>

        <div className="stat">
          <div className="stat-figure text-info">
            <Cpu className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.queuedMlJobs")}</div>
          <div className="stat-value text-info">{totalQueuedMLJobs}</div>
          <div className="stat-desc">{t("monitor.ml.taskWorkers")}</div>
        </div>

        <div className="stat">
          <div className="stat-figure text-secondary">
            <Workflow className="w-8 h-8" />
          </div>
          <div className="stat-title">{t("monitor.ml.reindexJobs")}</div>
          <div className="stat-value text-secondary">
            {stats?.reindexJobs ?? 0}
          </div>
          <div className="stat-desc">{t("monitor.ml.batchQueue")}</div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        {taskCards.map(({ key, label, stats: taskStats }) => {
          const indexedCount = taskStats?.indexedCount ?? 0;
          const queuedJobs = taskStats?.queuedJobs ?? 0;
          const coverage = taskStats?.coverage ?? 0;
          const remaining = Math.max(
            (stats?.photoTotal ?? 0) - indexedCount,
            0,
          );

          return (
            <section
              key={key}
              className="bg-base-100 rounded-lg shadow-sm p-4 space-y-4"
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-lg font-semibold">{label}</h2>
                  <p className="text-sm text-base-content/70">
                    {t("monitor.ml.coverageValue", {
                      indexed: indexedCount,
                      total: stats?.photoTotal ?? 0,
                      percent: formatCoveragePercent(coverage),
                    })}
                  </p>
                </div>
                <span className="badge badge-outline">
                  {formatCoveragePercent(coverage)}
                </span>
              </div>

              <progress
                className="progress progress-primary w-full"
                value={Math.round(coverage * 100)}
                max="100"
              />

              <div className="grid grid-cols-3 gap-3 text-sm">
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">
                    {t("monitor.ml.indexedAssets")}
                  </div>
                  <div className="mt-1 font-semibold">{indexedCount}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">
                    {t("monitor.ml.remainingAssets")}
                  </div>
                  <div className="mt-1 font-semibold">{remaining}</div>
                </div>
                <div className="rounded-lg border border-base-300 px-3 py-2">
                  <div className="text-base-content/60">
                    {t("monitor.ml.queuedJobs")}
                  </div>
                  <div className="mt-1 font-semibold">{queuedJobs}</div>
                </div>
              </div>
            </section>
          );
        })}
      </div>
    </div>
  );
}
