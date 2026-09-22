import { useState, type ReactNode } from "react";
import { ChevronLeft, RefreshCcw } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/features/notifications";
import {
  useProcessingStageItems,
  useRetryProcessingItem,
  useRetryProcessingStage,
  type ProcessingItem,
  type ProcessingStage,
  type ProcessingSummary,
} from "../../api/useProcessing";
import {
  STAGE_TASK,
  STATUS_BADGE,
  isRetryableStageId,
  reasonLabel,
  sourceLabel,
  stageLabel,
  statusLabel,
} from "../../model/stageVocabulary";
import { formatRelativeTime } from "../../model/time";
import { ProcessingTray } from "../../modules/rive/ProcessingTray";

const PAGE = 10;

function Stat({ label, value, tone = "" }: { label: string; value: ReactNode; tone?: string }) {
  return (
    <div className={`flex flex-col gap-0.5 rounded-box bg-base-200 p-3 ${tone}`}>
      <dt className="text-xs text-base-content/60">{label}</dt>
      <dd className="text-xl font-semibold tabular-nums">{value}</dd>
    </div>
  );
}

function useRelative() {
  const { t, i18n } = useI18n();
  return (value?: string) => formatRelativeTime(value, t, i18n.resolvedLanguage || i18n.language);
}

/** Idle panel: the Rive tray for media in progress and six overall totals. */
export function OverviewPanel({ overview }: { overview: ProcessingSummary["overview"] }) {
  const { t, i18n } = useI18n();
  const relative = useRelative();
  const formatter = new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language);
  const format = (value: number) => formatter.format(value);
  const inProgress = overview?.media_in_progress ?? 0;
  return (
    <section aria-labelledby="processing-overview" className="flex flex-col gap-4">
      <h2 id="processing-overview" className="text-base font-semibold">
        {t("monitor.processing.overview.title", "Overview")}
      </h2>
      <div className="mx-auto w-full max-w-56">
        <ProcessingTray pending={inProgress} />
      </div>
      <dl className="grid grid-cols-2 gap-2">
        <Stat
          label={t("monitor.processing.overview.mediaTotal", "Media")}
          value={format(overview?.media_total ?? 0)}
        />
        <Stat
          label={t("monitor.processing.overview.inProgress", "In progress")}
          value={format(inProgress)}
        />
        <Stat
          label={t("monitor.processing.overview.running", "Running now")}
          value={format(overview?.running ?? 0)}
        />
        <Stat
          label={t("monitor.processing.overview.failed", "Failed")}
          value={format(overview?.failed_media ?? 0)}
          tone={(overview?.failed_media ?? 0) > 0 ? "outline outline-2 outline-warning/70" : ""}
        />
        <Stat
          label={t("monitor.processing.overview.catalog", "Catalog updates")}
          value={format(overview?.catalog_pending ?? 0)}
        />
        <Stat
          label={t("monitor.processing.overview.lastActivity", "Last activity")}
          value={<span className="text-base">{relative(overview?.last_activity_at)}</span>}
        />
      </dl>
    </section>
  );
}

function ItemList({
  stage,
  state,
  title,
  onRetry,
  retryingId,
}: {
  stage: ProcessingStage;
  state: "failed" | "queued";
  title: string;
  onRetry?: (item: ProcessingItem) => void;
  retryingId?: string;
}) {
  const { t } = useI18n();
  const relative = useRelative();
  const [limit, setLimit] = useState(PAGE);
  const query = useProcessingStageItems(stage.id, state, limit);
  const items = query.data?.items ?? [];
  if (items.length === 0 && !query.isLoading) return null;
  return (
    <section className="flex flex-col gap-1">
      <h3 className="text-sm font-semibold">{title}</h3>
      <ul className="flex flex-col">
        {items.map((item) => (
          <li
            key={item.subject_id}
            className="flex items-start gap-3 border-t border-base-200 py-2.5 first:border-t-0"
          >
            <div className="flex min-w-0 flex-1 flex-col gap-0.5">
              <span className="truncate text-sm font-medium">{item.label}</span>
              {state === "failed" && (
                <span className="text-sm">{reasonLabel(t, item.reason_code)}</span>
              )}
              <span className="text-xs text-base-content/60">
                {item.attempts
                  ? `${t("monitor.processing.detail.attempts", "{{count}} attempts", { count: item.attempts })} · `
                  : ""}
                {relative(item.updated_at)}
              </span>
            </div>
            {onRetry && item.asset_id && (
              <button
                type="button"
                className="btn btn-ghost btn-xs"
                disabled={retryingId === item.asset_id}
                onClick={() => onRetry(item)}
              >
                {t("monitor.processing.detail.retry", "Retry")}
              </button>
            )}
          </li>
        ))}
      </ul>
      {query.data?.next_cursor && (
        <button
          type="button"
          className="btn btn-ghost btn-sm self-start"
          onClick={() => setLimit((current) => Math.min(current + PAGE, 50))}
        >
          {t("monitor.processing.detail.more", "Show more")}
        </button>
      )}
    </section>
  );
}

/** Selected stage: counts, facts, failed items with retry, then queued items. */
export function StageDetailPanel({
  stage,
  onBack,
}: {
  stage: ProcessingStage;
  onBack: () => void;
}) {
  const { t, i18n } = useI18n();
  const showMessage = useMessage();
  const relative = useRelative();
  const formatter = new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language);
  const format = (value: number) => formatter.format(value);
  const retryStage = useRetryProcessingStage();
  const retryItem = useRetryProcessingItem();
  const status = stage.status ?? "idle";
  const task = stage.id ? STAGE_TASK[stage.id] : undefined;
  const sources = Object.entries(stage.sources ?? {});

  const facts: [string, string][] = [];
  if (stage.done != null)
    facts.push([t("monitor.processing.detail.done", "Done"), format(stage.done)]);
  if (sources.length > 0) {
    facts.push([
      t("monitor.processing.detail.sources", "Requested by"),
      sources.map(([kind, count]) => `${sourceLabel(t, kind)} ${format(count)}`).join(" · "),
    ]);
  }
  if (stage.oldest_queued_at)
    facts.push([
      t("monitor.processing.detail.oldest", "Oldest waiting"),
      relative(stage.oldest_queued_at),
    ]);
  facts.push([
    t("monitor.processing.detail.lastActivity", "Last activity"),
    relative(stage.last_activity_at),
  ]);

  const retryAll = async () => {
    if (!isRetryableStageId(stage.id)) return;
    try {
      const result = await retryStage.mutateAsync({ params: { path: { stage: stage.id } } });
      showMessage(
        "success",
        t("monitor.processing.detail.retryAccepted", "{{count}} queued for retry", {
          count: result.accepted ?? 0,
        }),
      );
    } catch {
      showMessage(
        "error",
        t("monitor.processing.detail.retryFailed", "Retry could not be started."),
      );
    }
  };

  const retryOne = async (item: ProcessingItem) => {
    if (!task || !item.asset_id) return;
    try {
      await retryItem.mutateAsync({
        params: { path: { id: item.asset_id } },
        body: { tasks: [task] },
      });
    } catch {
      showMessage(
        "error",
        t("monitor.processing.detail.retryFailed", "Retry could not be started."),
      );
    }
  };

  return (
    <section aria-labelledby="processing-stage-detail" className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <button
          type="button"
          className="btn btn-ghost btn-sm btn-square"
          onClick={onBack}
          aria-label={t("monitor.processing.detail.back", "Back to overview")}
        >
          <ChevronLeft className="size-4" aria-hidden />
        </button>
        <h2 id="processing-stage-detail" className="flex-1 text-base font-semibold">
          {stageLabel(t, stage.id)}
        </h2>
        <span className={`badge badge-sm ${STATUS_BADGE[status]}`}>{statusLabel(t, status)}</span>
      </div>
      <dl className="grid grid-cols-2 gap-2">
        <Stat
          label={t("monitor.processing.detail.queued", "Queued")}
          value={format(stage.queued ?? 0)}
        />
        <Stat
          label={t("monitor.processing.detail.running", "Running")}
          value={format(stage.running ?? 0)}
        />
        <Stat
          label={t("monitor.processing.detail.retrying", "Retrying")}
          value={format(stage.retrying ?? 0)}
        />
        <Stat
          label={t("monitor.processing.detail.failed", "Failed")}
          value={format(stage.failed ?? 0)}
          tone={(stage.failed ?? 0) > 0 ? "outline outline-2 outline-warning/70" : ""}
        />
      </dl>
      <dl className="flex flex-col text-sm">
        {facts.map(([label, value]) => (
          <div key={label} className="flex justify-between gap-3 border-t border-base-200 py-1.5">
            <dt className="text-base-content/60">{label}</dt>
            <dd className="text-right">{value}</dd>
          </div>
        ))}
      </dl>
      {(stage.failed ?? 0) > 0 && stage.retryable && (
        <button
          type="button"
          className="btn btn-primary btn-sm"
          disabled={retryStage.isPending}
          onClick={() => void retryAll()}
        >
          <RefreshCcw
            className={`size-4 ${retryStage.isPending ? "motion-safe:animate-spin" : ""}`}
            aria-hidden
          />
          {t("monitor.processing.detail.retryAll", "Retry all failed")}
        </button>
      )}
      {(stage.failed ?? 0) > 0 && (
        <ItemList
          stage={stage}
          state="failed"
          title={t("monitor.processing.detail.failedItems", "Failed")}
          onRetry={task ? (item) => void retryOne(item) : undefined}
          retryingId={retryItem.isPending ? retryItem.variables?.params.path.id : undefined}
        />
      )}
      {(stage.queued ?? 0) + (stage.retrying ?? 0) + (stage.running ?? 0) > 0 && (
        <ItemList
          stage={stage}
          state="queued"
          title={t("monitor.processing.detail.queuedItems", "Waiting")}
        />
      )}
    </section>
  );
}
