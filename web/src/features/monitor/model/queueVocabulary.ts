import type { TFunction } from "i18next";
import type { QueueErrorSampleDTO, QueueSummaryDTO } from "../types";

/**
 * Vocabulary for the closed macro-job catalog.
 *
 * River runs exactly one `catalog_macro` queue with eight durable kinds
 * ([BACKEND.md](../../../../../docs/BACKEND.md), Queues And Processing).
 * Fine-grained tasks are an in-process DAG and never appear as River jobs, so
 * this set is closed: a kind outside it is a contract change, not a display
 * detail. `getKindLabels` covers every member and `unknownKindLabel` exists only
 * so a server that is ahead of the Web stays readable instead of blank.
 */

export const MACRO_JOB_KINDS = [
  "ingest_asset",
  "scan_repository_batch",
  "analyze_asset",
  "generate_asset_derivatives",
  "transcode_media",
  "enrich_asset",
  "rebuild_projection_batch",
  "backup_catalog",
] as const;

export type MacroJobKind = (typeof MACRO_JOB_KINDS)[number];

export const CANONICAL_QUEUE_NAMES = ["catalog_macro"] as const;

export type QueueCopy = {
  name: string;
  description: string;
};

/** Queue names the product names itself; unknown queue names stay server-owned. */
export function getQueueCopy(t: TFunction): Record<string, QueueCopy> {
  return {
    catalog_macro: {
      name: t("monitor.queueSummary.queues.catalog_macro.name", {
        defaultValue: "Catalog processing",
      }),
      description: t("monitor.queueSummary.queues.catalog_macro.description", {
        defaultValue: "Runs bounded catalog pipeline stages with shared resource limits.",
      }),
    },
  };
}

/** Display label for every member of the closed macro-job set. */
export function getKindLabels(t: TFunction): Record<string, string> {
  return {
    analyze_asset: t("monitor.queueSummary.kinds.analyze_asset", {
      defaultValue: "Read media information",
    }),
    backup_catalog: t("monitor.queueSummary.kinds.backup_catalog", {
      defaultValue: "Create catalog backup",
    }),
    enrich_asset: t("monitor.queueSummary.kinds.enrich_asset", {
      defaultValue: "Run image and media enrichment",
    }),
    generate_asset_derivatives: t("monitor.queueSummary.kinds.generate_asset_derivatives", {
      defaultValue: "Build previews",
    }),
    ingest_asset: t("monitor.queueSummary.kinds.ingest_asset", {
      defaultValue: "Import uploaded file",
    }),
    rebuild_projection_batch: t("monitor.queueSummary.kinds.rebuild_projection_batch", {
      defaultValue: "Rebuild catalog projection",
    }),
    scan_repository_batch: t("monitor.queueSummary.kinds.scan_repository_batch", {
      defaultValue: "Scan Repository batch",
    }),
    transcode_media: t("monitor.queueSummary.kinds.transcode_media", {
      defaultValue: "Prepare playable media",
    }),
  };
}

export function isMacroJobKind(value: string | undefined): value is MacroJobKind {
  return value != null && (MACRO_JOB_KINDS as readonly string[]).includes(value);
}

export function unknownKindLabel(t: TFunction, kind: string | undefined): string {
  return t("monitor.queueSummary.kinds.unknown", {
    defaultValue: "Other background task · {{name}}",
    name: kind || t("common.unknown", { defaultValue: "Unknown" }),
  });
}

/** Delivery states an operator can act on, as reported by an error sample. */
export function getErrorStateLabels(t: TFunction): Record<string, string> {
  return {
    cancelled: t("monitor.queueSummary.errorStates.cancelled"),
    discarded: t("monitor.queueSummary.errorStates.discarded"),
    retryable: t("monitor.queueSummary.errorStates.retryable"),
  };
}

/**
 * Semantic status of a delivery queue. Tone and badge classes stay in the flow:
 * this module owns the rule, not the styling.
 */
export type QueueStatus = "needsAttention" | "working" | "pendingWork" | "settled" | "idle";

export function queueStatus(queue: QueueSummaryDTO): QueueStatus {
  if ((queue.attention_jobs ?? 0) > 0) return "needsAttention";
  if ((queue.remaining_jobs ?? 0) > 0) {
    return (queue.running_jobs ?? 0) > 0 ? "working" : "pendingWork";
  }
  if ((queue.total_jobs ?? 0) > 0) return "settled";
  return "idle";
}

export function queueStatusLabel(t: TFunction, status: QueueStatus): string {
  switch (status) {
    case "needsAttention":
      return t("monitor.queueSummary.status.needsAttention");
    case "working":
      return t("monitor.queueSummary.status.working");
    case "pendingWork":
      return t("monitor.queueSummary.status.pendingWork");
    case "settled":
      return t("monitor.queueSummary.status.settled");
    case "idle":
      return t("monitor.queueSummary.status.idle");
  }
}

/**
 * Operator-only diagnostic block. Queue samples may keep sanitized technical
 * detail that ordinary UI copy never uses, so this text is copied verbatim into
 * a report and is not localized.
 */
export function buildDiagnosticText(queueName: string, sample: QueueErrorSampleDTO): string {
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
