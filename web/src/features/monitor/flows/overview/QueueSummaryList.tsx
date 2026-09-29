import { useId, useMemo, useState, type ReactNode } from "react";
import { CheckCircle2, ChevronDown, Copy, Layers3, Workflow } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { copyText } from "@/lib/clipboard";
import { useI18n } from "@/lib/i18n.tsx";
import {
  buildDiagnosticText,
  getErrorStateLabels,
  getKindLabels,
  getQueueCopy,
  queueStatus,
  queueStatusLabel,
  unknownKindLabel,
  type QueueStatus,
} from "../../model/queueVocabulary";
import { formatDuration, formatRelativeTime } from "../../model/time";
import type { QueueErrorSampleDTO, QueueSummaryDTO } from "../../types";

type QueuePresentation = {
  icon: LucideIcon;
  tone: string;
};

/**
 * Icon and tone are presentation, so they stay with the flow; the queue and
 * macro-kind vocabulary lives in `model/queueVocabulary.ts`.
 */
const QUEUE_PRESENTATION: Record<string, QueuePresentation> = {
  catalog_macro: { icon: Workflow, tone: "text-primary bg-primary/10" },
};

const STATUS_BADGE: Record<QueueStatus, string> = {
  needsAttention: "badge-warning",
  working: "badge-info",
  pendingWork: "badge-info",
  settled: "badge-success",
  idle: "badge-ghost",
};

function getPresentation(name?: string): QueuePresentation {
  return (
    (name ? QUEUE_PRESENTATION[name] : undefined) ?? {
      icon: Layers3,
      tone: "text-base-content bg-base-200",
    }
  );
}

export function QueueSummaryList({
  queues,
  actions,
  details,
}: {
  queues: QueueSummaryDTO[];
  actions?: ReactNode;
  details?: ReactNode;
}) {
  const { t } = useI18n();
  return (
    <section className="space-y-3">
      <header className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-base font-semibold">{t("monitor.queueSummary.title")}</h2>
        {actions}
      </header>
      {details}
      <QueueActivityList queues={queues} />
    </section>
  );
}

function QueueActivityList({ queues }: { queues: QueueSummaryDTO[] }) {
  const { t, i18n } = useI18n();
  const showMessage = useMessage();
  const id = useId();
  const [expandedQueues, setExpandedQueues] = useState<Set<string>>(new Set());
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const numberFormatter = useMemo(
    () => new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language),
    [i18n.language, i18n.resolvedLanguage],
  );
  const queueCopy = useMemo(() => getQueueCopy(t), [t]);
  const kindLabels = useMemo(() => getKindLabels(t), [t]);
  const errorStateLabels = useMemo(() => getErrorStateLabels(t), [t]);

  const toggleQueue = (queueName: string) => {
    setExpandedQueues((current) => {
      const next = new Set(current);
      if (next.has(queueName)) {
        next.delete(queueName);
      } else {
        next.add(queueName);
      }
      return next;
    });
  };

  const copyDiagnostic = async (queueName: string, sample: QueueErrorSampleDTO) => {
    const key = `${queueName}:${sample.job_id}`;
    try {
      await copyText(buildDiagnosticText(queueName, sample));
      setCopiedKey(key);
      window.setTimeout(() => {
        setCopiedKey((current) => (current === key ? null : current));
      }, 1600);
    } catch {
      setCopiedKey(null);
      showMessage("error", t("common.copyFailed", { defaultValue: "Copy failed." }));
    }
  };

  if (queues.length === 0) {
    return (
      <div className="card flex-row items-center gap-2 bg-base-100 p-5 text-sm text-base-content/60">
        <CheckCircle2 className="size-4" aria-hidden />
        {t("monitor.queueSummary.emptyTitle")}
      </div>
    );
  }

  return (
    <div className="card overflow-hidden bg-base-100">
      <ul className="list">
        {queues.map((queue) => {
          const queueName = queue.name ?? "";
          const copy = queueCopy[queueName];
          const displayName =
            copy?.name ??
            t("monitor.queueSummary.queues.unknown.name", {
              defaultValue: "Other processing · {{name}}",
              name: queueName || t("common.unknown", { defaultValue: "Unknown" }),
            });
          const presentation = getPresentation(queueName);
          const Icon = presentation.icon;
          const status = queueStatus(queue);
          const statusLabel = queueStatusLabel(t, status);
          const isExpanded = expandedQueues.has(queueName);
          const processed = queue.processed_jobs ?? 0;
          const total = queue.total_jobs ?? 0;
          const averageLatency = formatDuration(queue.average_latency_ms);
          const averageRuntime = formatDuration(queue.average_runtime_ms);
          const latestActivity = formatRelativeTime(
            queue.latest_activity_at,
            t,
            i18n.resolvedLanguage || i18n.language,
          );
          const oldestRemaining = queue.oldest_remaining_at
            ? formatRelativeTime(
                queue.oldest_remaining_at,
                t,
                i18n.resolvedLanguage || i18n.language,
              )
            : null;
          const errorSamples = queue.error_samples ?? [];

          return (
            <li
              key={queueName}
              className="list-row grid-cols-[auto_minmax(0,1fr)] items-center sm:grid-cols-[auto_minmax(0,1fr)_auto]"
            >
              <div
                className={`flex size-9 shrink-0 items-center justify-center rounded-box ${presentation.tone}`}
              >
                <Icon className="size-5" aria-hidden />
              </div>
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-semibold">{displayName}</h3>
                  <span className={`badge badge-sm ${STATUS_BADGE[status]}`}>{statusLabel}</span>
                </div>
                <p className="mt-1 text-xs text-base-content/60">
                  {t("monitor.queueSummary.metrics.latestActivity", { value: latestActivity })}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm col-start-2 row-start-2 justify-self-start sm:col-start-3 sm:row-start-1 sm:justify-self-end"
                aria-expanded={isExpanded}
                aria-controls={`${id}-${queueName}`}
                onClick={() => toggleQueue(queueName)}
              >
                {(queue.attention_jobs ?? 0) > 0
                  ? t("monitor.queueSummary.reviewErrors", { count: queue.attention_jobs ?? 0 })
                  : t("monitor.queueSummary.details", "Details")}
                <ChevronDown className={`size-4 ${isExpanded ? "rotate-180" : ""}`} aria-hidden />
              </button>
              <div
                id={`${id}-${queueName}`}
                className={`collapse list-col-wrap col-span-full row-start-3 rounded-none sm:row-start-2 ${isExpanded ? "collapse-open" : "collapse-close"}`}
              >
                <div className="collapse-content px-0">
                  {isExpanded && (
                    <>
                      <dl className="grid grid-cols-2 gap-4 border-t border-base-200 py-4 sm:grid-cols-3 lg:grid-cols-6">
                        <Metric
                          label={t("monitor.queueSummary.metrics.total")}
                          value={numberFormatter.format(total)}
                        />
                        <Metric
                          label={t("monitor.queueSummary.metrics.processed")}
                          value={numberFormatter.format(processed)}
                        />
                        <Metric
                          label={t("monitor.queueSummary.metrics.remaining")}
                          value={numberFormatter.format(queue.remaining_jobs ?? 0)}
                        />
                        <Metric
                          label={t("monitor.queueSummary.metrics.attention")}
                          value={numberFormatter.format(queue.attention_jobs ?? 0)}
                          tone={(queue.attention_jobs ?? 0) > 0 ? "text-warning" : ""}
                        />
                        <Metric
                          label={t("monitor.queueSummary.metrics.averageRuntime")}
                          value={averageRuntime ?? t("monitor.queueSummary.metrics.notEnoughData")}
                        />
                        <Metric
                          label={t("monitor.queueSummary.metrics.averageLatency")}
                          value={averageLatency ?? t("monitor.queueSummary.metrics.notEnoughData")}
                        />
                      </dl>
                      {oldestRemaining && (
                        <p className="mb-3 text-xs text-base-content/60">
                          {t("monitor.queueSummary.metrics.oldestRemaining", {
                            value: oldestRemaining,
                          })}
                        </p>
                      )}
                      {(queue.attention_jobs ?? 0) > 0 && (
                        <ul className="list border-t border-base-200">
                          {errorSamples.length === 0 ? (
                            <li className="list-row text-sm text-base-content/60">
                              {t("monitor.queueSummary.noErrorSamples")}
                            </li>
                          ) : (
                            errorSamples.map((sample) => {
                              const sampleKey = `${queueName}:${sample.job_id}`;
                              const errorTime =
                                sample.attempted_at ?? sample.finalized_at ?? sample.created_at;
                              const errorLabel =
                                errorStateLabels[sample.state ?? ""] ??
                                t("monitor.queueSummary.errorStates.unknown");
                              const kindLabel =
                                kindLabels[sample.kind ?? ""] ?? unknownKindLabel(t, sample.kind);

                              return (
                                <li key={sampleKey} className="p-0">
                                  <details className="collapse group/error rounded-none">
                                    <summary className="collapse-title flex min-h-0 items-start gap-2 px-0 py-3 text-sm">
                                      <ChevronDown
                                        className="size-4 shrink-0 group-open/error:rotate-180"
                                        aria-hidden
                                      />
                                      <span className="min-w-0 flex-1">
                                        <span className="block font-medium sm:inline">
                                          {kindLabel}
                                        </span>
                                        <span className="mt-1 flex items-center gap-2 sm:ml-3 sm:mt-0 sm:inline-flex">
                                          <span className="badge badge-warning badge-sm">
                                            {errorLabel}
                                          </span>
                                          <span className="text-xs text-base-content/60">
                                            #{sample.job_id}
                                          </span>
                                        </span>
                                      </span>
                                    </summary>
                                    <div className="collapse-content">
                                      <div className="flex flex-wrap items-center justify-between gap-2">
                                        <p className="flex flex-wrap gap-3 text-xs text-base-content/60">
                                          <span>
                                            {t("monitor.queueSummary.errorAttempt", {
                                              current: sample.attempt ?? 0,
                                              max: sample.max_attempts ?? 0,
                                            })}
                                          </span>
                                          <span>
                                            {formatRelativeTime(
                                              errorTime,
                                              t,
                                              i18n.resolvedLanguage || i18n.language,
                                            )}
                                          </span>
                                        </p>
                                        <button
                                          type="button"
                                          className="btn btn-xs btn-ghost gap-1 self-start"
                                          onClick={() => void copyDiagnostic(queueName, sample)}
                                        >
                                          <Copy className="h-3.5 w-3.5" />
                                          {copiedKey === sampleKey
                                            ? t("monitor.queueSummary.copied")
                                            : t("monitor.queueSummary.copyError")}
                                        </button>
                                      </div>

                                      <p className="mt-2 whitespace-pre-wrap break-words text-sm text-error">
                                        {sample.last_error ||
                                          t("monitor.queueSummary.errorFallback")}
                                      </p>
                                    </div>
                                  </details>
                                </li>
                              );
                            })
                          )}
                        </ul>
                      )}
                    </>
                  )}
                </div>
              </div>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function Metric({ label, value, tone = "" }: { label: string; value: string; tone?: string }) {
  return (
    <div className="stat min-w-0 p-0">
      <dt className="stat-title text-xs">{label}</dt>
      <dd className={`stat-value text-base ${tone}`}>{value}</dd>
    </div>
  );
}
