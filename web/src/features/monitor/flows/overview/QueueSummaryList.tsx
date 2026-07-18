import { useMemo, useState } from "react";
import {
  AlertCircle,
  Brain,
  CheckCircle2,
  ChevronDown,
  Clock,
  Copy,
  FileSearch,
  Fingerprint,
  FolderSearch,
  ImageIcon,
  Layers3,
  Loader2,
  MapPinned,
  RefreshCw,
  ScanSearch,
  Sparkles,
  TextSearch,
  UsersRound,
  Video,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import type { QueueErrorSampleDTO, QueueSummaryDTO } from "../../types";

type QueuePresentation = {
  icon: LucideIcon;
  tone: string;
};

type TFunction = (key: string, options?: Record<string, unknown>) => string;

type QueueCopy = {
  name: string;
  description: string;
};

const QUEUE_PRESENTATION: Record<string, QueuePresentation> = {
  ingest_asset: { icon: ImageIcon, tone: "text-primary bg-primary/10" },
  discover_asset: { icon: FileSearch, tone: "text-info bg-info/10" },
  metadata_asset: { icon: ScanSearch, tone: "text-secondary bg-secondary/10" },
  thumbnail_asset: { icon: Sparkles, tone: "text-success bg-success/10" },
  transcode_asset: { icon: Video, tone: "text-accent bg-accent/10" },
  retry_asset: { icon: RefreshCw, tone: "text-warning bg-warning/10" },
  reindex_assets: { icon: Brain, tone: "text-primary bg-primary/10" },
  rebuild_location_clusters: { icon: MapPinned, tone: "text-info bg-info/10" },
  scan_repository: { icon: FolderSearch, tone: "text-info bg-info/10" },
  detect_stacks: { icon: Layers3, tone: "text-secondary bg-secondary/10" },
  match_live_photo: { icon: Layers3, tone: "text-success bg-success/10" },
  process_semantic: { icon: Brain, tone: "text-primary bg-primary/10" },
  process_bioclip: { icon: Sparkles, tone: "text-success bg-success/10" },
  process_ocr: { icon: TextSearch, tone: "text-accent bg-accent/10" },
  process_face: { icon: UsersRound, tone: "text-secondary bg-secondary/10" },
  classify_zeroshot: { icon: Brain, tone: "text-primary bg-primary/10" },
  process_phash: { icon: Fingerprint, tone: "text-warning bg-warning/10" },
};

function humanizeQueueName(name?: string): string {
  if (!name) return "";
  return name.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

function getPresentation(name?: string): QueuePresentation {
  return (
    (name ? QUEUE_PRESENTATION[name] : undefined) ?? {
      icon: Layers3,
      tone: "text-base-content bg-base-200",
    }
  );
}

function getQueueCopy(t: TFunction): Record<string, QueueCopy> {
  return {
    classify_zeroshot: {
      name: t("monitor.queueSummary.queues.classify_zeroshot.name"),
      description: t("monitor.queueSummary.queues.classify_zeroshot.description"),
    },
    detect_stacks: {
      name: t("monitor.queueSummary.queues.detect_stacks.name"),
      description: t("monitor.queueSummary.queues.detect_stacks.description"),
    },
    discover_asset: {
      name: t("monitor.queueSummary.queues.discover_asset.name"),
      description: t("monitor.queueSummary.queues.discover_asset.description"),
    },
    ingest_asset: {
      name: t("monitor.queueSummary.queues.ingest_asset.name"),
      description: t("monitor.queueSummary.queues.ingest_asset.description"),
    },
    match_live_photo: {
      name: t("monitor.queueSummary.queues.match_live_photo.name"),
      description: t("monitor.queueSummary.queues.match_live_photo.description"),
    },
    metadata_asset: {
      name: t("monitor.queueSummary.queues.metadata_asset.name"),
      description: t("monitor.queueSummary.queues.metadata_asset.description"),
    },
    process_bioclip: {
      name: t("monitor.queueSummary.queues.process_bioclip.name"),
      description: t("monitor.queueSummary.queues.process_bioclip.description"),
    },
    process_face: {
      name: t("monitor.queueSummary.queues.process_face.name"),
      description: t("monitor.queueSummary.queues.process_face.description"),
    },
    process_ocr: {
      name: t("monitor.queueSummary.queues.process_ocr.name"),
      description: t("monitor.queueSummary.queues.process_ocr.description"),
    },
    process_phash: {
      name: t("monitor.queueSummary.queues.process_phash.name"),
      description: t("monitor.queueSummary.queues.process_phash.description"),
    },
    process_semantic: {
      name: t("monitor.queueSummary.queues.process_semantic.name"),
      description: t("monitor.queueSummary.queues.process_semantic.description"),
    },
    rebuild_location_clusters: {
      name: t("monitor.queueSummary.queues.rebuild_location_clusters.name"),
      description: t("monitor.queueSummary.queues.rebuild_location_clusters.description"),
    },
    reindex_assets: {
      name: t("monitor.queueSummary.queues.reindex_assets.name"),
      description: t("monitor.queueSummary.queues.reindex_assets.description"),
    },
    retry_asset: {
      name: t("monitor.queueSummary.queues.retry_asset.name"),
      description: t("monitor.queueSummary.queues.retry_asset.description"),
    },
    scan_repository: {
      name: t("monitor.queueSummary.queues.scan_repository.name"),
      description: t("monitor.queueSummary.queues.scan_repository.description"),
    },
    thumbnail_asset: {
      name: t("monitor.queueSummary.queues.thumbnail_asset.name"),
      description: t("monitor.queueSummary.queues.thumbnail_asset.description"),
    },
    transcode_asset: {
      name: t("monitor.queueSummary.queues.transcode_asset.name"),
      description: t("monitor.queueSummary.queues.transcode_asset.description"),
    },
  };
}

function getKindLabels(t: TFunction): Record<string, string> {
  return {
    classify_zeroshot: t("monitor.queueSummary.kinds.classify_zeroshot"),
    detect_stacks: t("monitor.queueSummary.kinds.detect_stacks"),
    discover_asset: t("monitor.queueSummary.kinds.discover_asset"),
    ingest_asset: t("monitor.queueSummary.kinds.ingest_asset"),
    match_live_photo: t("monitor.queueSummary.kinds.match_live_photo"),
    metadata_asset: t("monitor.queueSummary.kinds.metadata_asset"),
    process_bioclip: t("monitor.queueSummary.kinds.process_bioclip"),
    process_face: t("monitor.queueSummary.kinds.process_face"),
    process_ocr: t("monitor.queueSummary.kinds.process_ocr"),
    process_phash: t("monitor.queueSummary.kinds.process_phash"),
    process_semantic: t("monitor.queueSummary.kinds.process_semantic"),
    rebuild_location_clusters: t("monitor.queueSummary.kinds.rebuild_location_clusters"),
    reindex_assets: t("monitor.queueSummary.kinds.reindex_assets"),
    retry_asset: t("monitor.queueSummary.kinds.retry_asset"),
    scan_repository: t("monitor.queueSummary.kinds.scan_repository"),
    schedule_repository_scans: t("monitor.queueSummary.kinds.schedule_repository_scans"),
    thumbnail_asset: t("monitor.queueSummary.kinds.thumbnail_asset"),
    transcode_asset: t("monitor.queueSummary.kinds.transcode_asset"),
  };
}

function getErrorStateLabels(t: TFunction): Record<string, string> {
  return {
    cancelled: t("monitor.queueSummary.errorStates.cancelled"),
    discarded: t("monitor.queueSummary.errorStates.discarded"),
    retryable: t("monitor.queueSummary.errorStates.retryable"),
  };
}

function formatDuration(ms?: number | null): string | null {
  if (ms == null) return null;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(ms < 10000 ? 1 : 0)}s`;
  if (ms < 3600000) return `${(ms / 60000).toFixed(ms < 600000 ? 1 : 0)}m`;
  return `${(ms / 3600000).toFixed(1)}h`;
}

function formatRelativeTime(
  timestamp: string | undefined,
  t: (key: string, options?: Record<string, unknown>) => string,
  language?: string,
): string {
  if (!timestamp) return t("monitor.queueSummary.time.never");

  const date = new Date(timestamp);
  const secondsAgo = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000));

  if (secondsAgo < 60) {
    return t("monitor.queueSummary.time.justNow");
  }
  if (secondsAgo < 3600) {
    return t("monitor.queueSummary.time.minutesAgo", {
      count: Math.floor(secondsAgo / 60),
    });
  }
  if (secondsAgo < 86400) {
    return t("monitor.queueSummary.time.hoursAgo", {
      count: Math.floor(secondsAgo / 3600),
    });
  }
  if (secondsAgo < 604800) {
    return t("monitor.queueSummary.time.daysAgo", {
      count: Math.floor(secondsAgo / 86400),
    });
  }
  return date.toLocaleDateString(language);
}

function statusForQueue(queue: QueueSummaryDTO, t: TFunction) {
  if ((queue.attention_jobs ?? 0) > 0) {
    return {
      label: t("monitor.queueSummary.status.needsAttention"),
      className: "badge-warning",
    };
  }
  if ((queue.remaining_jobs ?? 0) > 0) {
    return {
      label:
        (queue.running_jobs ?? 0) > 0
          ? t("monitor.queueSummary.status.working")
          : t("monitor.queueSummary.status.pendingWork"),
      className: "badge-info",
    };
  }
  if ((queue.total_jobs ?? 0) > 0) {
    return {
      label: t("monitor.queueSummary.status.settled"),
      className: "badge-success",
    };
  }
  return {
    label: t("monitor.queueSummary.status.idle"),
    className: "badge-ghost",
  };
}

function buildDiagnosticText(queueName: string, sample: QueueErrorSampleDTO) {
  return [
    `queue=${queueName}`,
    `job_id=${sample.job_id ?? ""}`,
    `kind=${sample.kind ?? ""}`,
    `state=${sample.state ?? ""}`,
    `attempt=${sample.attempt ?? 0}/${sample.max_attempts ?? 0}`,
    `created_at=${sample.created_at ?? ""}`,
    `scheduled_at=${sample.scheduled_at ?? ""}`,
    `attempted_at=${sample.attempted_at ?? ""}`,
    `finalized_at=${sample.finalized_at ?? ""}`,
    "",
    sample.last_error ?? "",
  ].join("\n");
}

export function QueueSummaryList() {
  const { t, i18n } = useI18n();
  const [expandedQueues, setExpandedQueues] = useState<Set<string>>(new Set());
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const summaryQuery = $api.useQuery(
    "get",
    "/api/v1/admin/river/queue-summary",
    {
      params: {
        query: {
          error_limit: 5,
        },
      },
    },
    {
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      retry: false,
    },
  );

  const response = summaryQuery.data;
  const queues = response?.queues ?? [];
  const generatedAt = response?.generated_at;
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
    await navigator.clipboard.writeText(buildDiagnosticText(queueName, sample));
    setCopiedKey(key);
    window.setTimeout(() => {
      setCopiedKey((current) => (current === key ? null : current));
    }, 1600);
  };

  if (summaryQuery.isLoading) {
    return (
      <div className="flex min-h-64 items-center justify-center rounded-lg border border-base-300 bg-base-100 p-6 text-center shadow-sm">
        <div>
          <Loader2 className="mx-auto h-8 w-8 animate-spin text-primary" />
          <p className="mt-3 text-sm opacity-60">{t("monitor.queueSummary.loading")}</p>
        </div>
      </div>
    );
  }

  if (summaryQuery.isError) {
    return (
      <div className="flex min-h-64 items-center justify-center rounded-lg border border-base-300 bg-base-100 p-6 text-center shadow-sm">
        <div>
          <AlertCircle className="mx-auto h-8 w-8 text-error" />
          <div className="mt-2 text-sm text-error">{t("monitor.queueSummary.fetchError")}</div>
        </div>
      </div>
    );
  }

  if (queues.length === 0) {
    return (
      <div className="flex min-h-64 items-center justify-center rounded-lg border border-base-300 bg-base-100 p-8 text-center shadow-sm">
        <div>
          <CheckCircle2 className="mx-auto h-10 w-10 text-success" />
          <h2 className="mt-3 text-base font-semibold">{t("monitor.queueSummary.emptyTitle")}</h2>
          <p className="mt-1 text-sm opacity-60">{t("monitor.queueSummary.emptyDescription")}</p>
        </div>
      </div>
    );
  }

  return (
    <section className="overflow-hidden rounded-lg border border-base-300 bg-base-100 shadow-sm">
      <div className="flex flex-col gap-2 border-b border-base-300 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-base font-semibold">{t("monitor.queueSummary.title")}</h2>
          <p className="text-sm opacity-60">{t("monitor.queueSummary.subtitle")}</p>
        </div>
        <div className="flex items-center gap-2 text-xs opacity-60">
          <Clock className="h-3.5 w-3.5" />
          <span>
            {t("monitor.queueSummary.updated", {
              time: formatRelativeTime(generatedAt, t, i18n.resolvedLanguage || i18n.language),
            })}
          </span>
        </div>
      </div>

      <div className="divide-y divide-base-300">
        {queues.map((queue) => {
          const queueName = queue.name ?? "";
          const copy = queueCopy[queueName];
          const displayName = copy?.name ?? humanizeQueueName(queueName);
          const description =
            copy?.description ?? t("monitor.queueSummary.queues.default.description");
          const presentation = getPresentation(queueName);
          const Icon = presentation.icon;
          const status = statusForQueue(queue, t);
          const isExpanded = expandedQueues.has(queueName);
          const processed = queue.processed_jobs ?? 0;
          const total = queue.total_jobs ?? 0;
          const progress = total > 0 ? Math.round((processed / total) * 100) : 0;
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
            <article key={queueName} className="px-4 py-4">
              <div className="flex flex-col gap-4 xl:flex-row xl:items-start">
                <div className="flex min-w-0 flex-1 gap-3">
                  <div
                    className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-lg ${presentation.tone}`}
                  >
                    <Icon className="h-5 w-5" />
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="truncate text-sm font-semibold">{displayName}</h3>
                      <span className={`badge badge-sm ${status.className}`}>{status.label}</span>
                    </div>
                    <p className="mt-1 text-sm opacity-60">{description}</p>

                    <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-base-200">
                      <div
                        className={`h-full rounded-full ${
                          (queue.attention_jobs ?? 0) > 0 ? "bg-warning" : "bg-primary"
                        }`}
                        style={{ width: `${progress}%` }}
                      />
                    </div>

                    <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs opacity-60">
                      <span>
                        {t("monitor.queueSummary.metrics.latestActivity", {
                          value: latestActivity,
                        })}
                      </span>
                      {oldestRemaining && (
                        <span>
                          {t("monitor.queueSummary.metrics.oldestRemaining", {
                            value: oldestRemaining,
                          })}
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:w-[36rem]">
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
                </div>
              </div>

              {(queue.attention_jobs ?? 0) > 0 && (
                <div className="mt-4">
                  <button
                    type="button"
                    className="btn btn-xs btn-ghost gap-1"
                    aria-expanded={isExpanded}
                    onClick={() => toggleQueue(queueName)}
                  >
                    <ChevronDown
                      className={`h-4 w-4 transition-transform ${isExpanded ? "rotate-180" : ""}`}
                    />
                    {t("monitor.queueSummary.reviewErrors", {
                      count: queue.attention_jobs ?? 0,
                    })}
                  </button>

                  {isExpanded && (
                    <div className="mt-3 space-y-2">
                      {errorSamples.length === 0 ? (
                        <div className="rounded-lg border border-warning/30 bg-warning/10 px-3 py-2 text-sm text-warning">
                          {t("monitor.queueSummary.noErrorSamples")}
                        </div>
                      ) : (
                        errorSamples.map((sample) => {
                          const sampleKey = `${queueName}:${sample.job_id}`;
                          const errorTime =
                            sample.attempted_at ?? sample.finalized_at ?? sample.created_at;
                          const errorLabel =
                            errorStateLabels[sample.state ?? ""] ??
                            t("monitor.queueSummary.errorStates.unknown");
                          const kindLabel =
                            kindLabels[sample.kind ?? ""] ?? humanizeQueueName(sample.kind);

                          return (
                            <div
                              key={sampleKey}
                              className="rounded-lg border border-warning/30 bg-warning/5 p-3"
                            >
                              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                                <div className="min-w-0">
                                  <div className="flex flex-wrap items-center gap-2">
                                    <span className="text-sm font-medium">{kindLabel}</span>
                                    <span className="badge badge-warning badge-sm">
                                      {errorLabel}
                                    </span>
                                    <span className="text-xs opacity-50">#{sample.job_id}</span>
                                  </div>
                                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs opacity-60">
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
                                  </div>
                                </div>

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

                              <p className="mt-2 line-clamp-3 break-words rounded bg-base-100/70 px-2 py-1.5 text-xs text-error">
                                {sample.last_error || t("monitor.queueSummary.errorFallback")}
                              </p>
                            </div>
                          );
                        })
                      )}
                    </div>
                  )}
                </div>
              )}
            </article>
          );
        })}
      </div>
    </section>
  );
}

function Metric({ label, value, tone = "" }: { label: string; value: string; tone?: string }) {
  return (
    <div className="rounded-lg bg-base-200/60 px-3 py-2">
      <div className="text-[0.68rem] font-semibold uppercase tracking-wide opacity-50">{label}</div>
      <div className={`mt-1 truncate text-sm font-semibold ${tone}`}>{value}</div>
    </div>
  );
}
