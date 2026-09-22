import { useMemo, useState } from "react";
import { ChevronDown, RefreshCcw, Sparkles } from "lucide-react";
import { MonitorFrame } from "./MonitorFrame";
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

const ML_TASK_KEYS = ["semantic", "video_semantic", "ocr", "face"] as const;
type MLTaskKey = (typeof ML_TASK_KEYS)[number];

function getTaskLabel(t: (key: string) => string, key: MLTaskKey) {
  switch (key) {
    case "semantic":
      return t("settings.aiSettings.taskNames.semantic");
    case "video_semantic":
      return t("settings.aiSettings.taskNames.videoSemantic");
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

  // The legacy API repeats the same global enrichment backlog in every lane.
  // Read it once; these are pending media, not five independent job queues.
  const pendingMedia = stats?.tasks.semantic.queuedJobs;
  const rebuildingTasks = rebuildMutation.variables?.body?.tasks ?? [];
  const selectedReindexTask = reindexModal
    ? taskCards.find((task) => task.key === reindexModal.taskKey)
    : undefined;
  const selectedReindexTotal = selectedReindexTask?.stats?.totalCount ?? stats?.photoTotal ?? 0;
  const selectedReindexMissing = Math.max(
    selectedReindexTotal - (selectedReindexTask?.stats?.indexedCount ?? 0),
    0,
  );

  const fields = [
    ...taskCards,
    { key: "bioclip" as const, label: t("monitor.ml.bioAlbumCoverage"), stats: bioTaskStats },
  ];

  return (
    <MonitorFrame
      hasData={!!stats}
      isLoading={statsQuery.isLoading}
      isFetching={statsQuery.isFetching}
      error={statsQuery.isError ? t("monitor.ml.loadError") : undefined}
      onRefresh={() => void statsQuery.refetch()}
      updatedAt={statsQuery.dataUpdatedAt}
    >
      <div className="monitor-weave py-3">
        {fields.map(({ key, label, stats: taskStats }) => {
          const percent = Math.min(100, Math.max(0, Math.round((taskStats?.coverage ?? 0) * 100)));
          const empty = !taskStats?.totalCount;
          return (
            <section key={key} aria-label={label} className="min-w-0">
              <span className="block min-h-10 text-sm font-medium">{label}</span>
              <span className="monitor-weave-field" aria-hidden="true">
                {Array.from({ length: 100 }, (_, i) => (
                  <span key={i} data-covered={!empty && i < percent} />
                ))}
              </span>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="text-xl font-medium tabular-nums">
                  {empty ? "—" : formatCoveragePercent(taskStats?.coverage ?? 0)}
                </span>
                {key !== "bioclip" && !empty && (
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    disabled={statsQuery.isError || rebuildMutation.isPending}
                    onClick={() => {
                      setReindexAll((taskStats?.indexedCount ?? 0) >= (taskStats?.totalCount ?? 0));
                      setReindexModal({ taskKey: key, taskLabel: label });
                    }}
                  >
                    <RefreshCcw
                      aria-hidden
                      className={`size-4 ${rebuildMutation.isPending && rebuildingTasks.includes(key) ? "motion-safe:animate-spin" : ""}`}
                    />
                    {t("monitor.ml.reindex")}
                  </button>
                )}
              </div>
              <span className="mt-1 block text-xs text-base-content/60 tabular-nums">
                {empty
                  ? t("monitor.ml.noApplicable", "No applicable content")
                  : `${taskStats?.indexedCount} / ${taskStats?.totalCount}`}
              </span>
            </section>
          );
        })}
      </div>
      <details className="collapse group rounded-none">
        <summary className="collapse-title flex min-h-0 items-center gap-2 p-0 py-2 text-sm text-base-content/60">
          <ChevronDown aria-hidden className="size-4 transition-transform group-open:rotate-180" />
          {t("monitor.ml.aboutCoverage", "About coverage")}
        </summary>
        <div className="collapse-content space-y-2 px-0 text-sm text-base-content/60">
          <div className="flex flex-wrap gap-5 pt-2">
            <span className="flex items-center gap-2">
              <i className="size-2.5 bg-primary" aria-hidden />
              {t("monitor.ml.covered", "Covered")}
            </span>
            <span className="flex items-center gap-2">
              <i className="size-2.5 bg-base-300" aria-hidden />
              {t("monitor.ml.uncovered", "Uncovered")}
            </span>
          </div>
          <p>
            {t(
              "monitor.ml.cellMeaning",
              "Each cell ≈ 1% coverage, not one file. Task totals differ.",
            )}
          </p>
          <p>{t("monitor.ml.bioAlbumHint")}</p>
        </div>
      </details>
      <section aria-label={t("monitor.ml.globalActivity", "Global activity")} className="space-y-3">
        <h2 className="text-base font-semibold">
          {t("monitor.ml.globalActivity", "Global activity")}
        </h2>
        <ul className="list rounded-box bg-base-100">
          <li className="list-row grid-cols-[auto_minmax(0,1fr)_auto] items-center">
            <div className="rounded-box bg-primary/10 p-3 text-primary">
              <Sparkles aria-hidden className="size-5" />
            </div>
            <span className="font-medium">
              {t("monitor.ml.pendingMedia", "Media awaiting analysis")}
            </span>
            <span className="text-xl font-semibold tabular-nums">{pendingMedia}</span>
          </li>
          <li className="list-row grid-cols-[auto_minmax(0,1fr)_auto] items-center">
            <div className="rounded-box bg-primary/10 p-3 text-primary">
              <RefreshCcw aria-hidden className="size-5" />
            </div>
            <span className="font-medium">
              {t("monitor.ml.pendingRebuilds", "Reindex requests")}
            </span>
            <span className="text-xl font-semibold tabular-nums">{stats?.reindexJobs}</span>
          </li>
        </ul>
      </section>
      {reindexModal && (
        <div className="modal modal-open z-modal">
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
                disabled={rebuildMutation.isPending}
                onClick={async () => {
                  try {
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
                  } catch {
                    showMessage(
                      "error",
                      t("monitor.ml.rebuildFailed", "Rebuild could not be started. Try again."),
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
    </MonitorFrame>
  );
}
