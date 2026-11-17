import type { TFunction } from "i18next";
import type { components } from "@/lib/http-commons/schema";

type ProcessingStage = components["schemas"]["processing.StageSummary"];
type ProcessingStageId = NonNullable<ProcessingStage["id"]>;

type RetryableStageId =
  | "metadata"
  | "thumbnails"
  | "video"
  | "analysis"
  | "events"
  | "places"
  | "text_search";

const RETRYABLE_STAGES: readonly string[] = [
  "metadata",
  "thumbnails",
  "video",
  "analysis",
  "events",
  "places",
  "text_search",
];

/** Narrows a stage to the ids the stage-retry route accepts. */
export function isRetryableStageId(id: ProcessingStageId | undefined): id is RetryableStageId {
  return id !== undefined && RETRYABLE_STAGES.includes(id);
}

/** Pipeline task accepted by the per-file reprocess endpoint, per stage. */
export const STAGE_TASK: Partial<Record<ProcessingStageId, string>> = {
  metadata: "analyze",
  thumbnails: "derivatives",
  video: "transcode",
  analysis: "enrich",
};

export function stageLabel(t: TFunction, id: ProcessingStageId | undefined): string {
  switch (id) {
    case "import":
      return t("monitor.processing.stages.import", "Import");
    case "scan":
      return t("monitor.processing.stages.scan", "Scan");
    case "metadata":
      return t("monitor.processing.stages.metadata", "Metadata");
    case "thumbnails":
      return t("monitor.processing.stages.thumbnails", "Thumbnails");
    case "video":
      return t("monitor.processing.stages.video", "Video");
    case "analysis":
      return t("monitor.processing.stages.analysis", "Analysis");
    case "events":
      return t("monitor.processing.stages.events", "Events");
    case "places":
      return t("monitor.processing.stages.places", "Places");
    case "text_search":
      return t("monitor.processing.stages.textSearch", "Text search");
    case "backup":
      return t("monitor.processing.stages.backup", "Backup");
    default:
      return t("common.unknown", "Unknown");
  }
}

export function unitLabel(t: TFunction, unit: ProcessingStage["unit"], count: number): string {
  switch (unit) {
    case "repositories":
      return t("monitor.processing.units.repositories", "Repositories", { count });
    case "updates":
      return t("monitor.processing.units.updates", "updates", { count });
    case "runs":
      return t("monitor.processing.units.runs", "runs", { count });
    default:
      return t("monitor.processing.units.files", "files", { count });
  }
}

export function statusLabel(t: TFunction, status: ProcessingStage["status"]): string {
  switch (status) {
    case "attention":
      return t("monitor.processing.status.attention", "Needs attention");
    case "working":
      return t("monitor.processing.status.working", "In progress");
    case "retrying":
      return t("monitor.processing.status.retrying", "Retrying");
    case "waiting":
      return t("monitor.processing.status.waiting", "Waiting");
    default:
      return t("monitor.processing.status.idle", "Idle");
  }
}

export const STATUS_BADGE: Record<NonNullable<ProcessingStage["status"]>, string> = {
  attention: "badge-warning",
  working: "badge-info",
  retrying: "badge-info badge-soft",
  waiting: "badge-ghost",
  idle: "badge-ghost",
};

export function reasonLabel(t: TFunction, code: string | undefined): string {
  switch (code) {
    case "unsupported_media":
      return t("monitor.processing.reasons.unsupported", "Format not supported");
    case "processing_retry_exhausted":
      return t("monitor.processing.reasons.exhausted", "Failed after every retry");
    default:
      return t("monitor.processing.reasons.failed", "Processing failed");
  }
}

export function sourceLabel(t: TFunction, kind: string): string {
  switch (kind) {
    case "reindex":
      return t("monitor.processing.sources.reindex", "Reindex");
    case "retry":
      return t("monitor.processing.sources.retry", "Retry");
    case "reprocess":
      return t("monitor.processing.sources.reprocess", "Reprocess");
    default:
      return kind;
  }
}

/** The one-line summary under a card's number: failures first, then work. */
export function cardLine(t: TFunction, stage: ProcessingStage): string {
  if ((stage.failed ?? 0) > 0) {
    return t("monitor.processing.line.failed", "{{count}} failed", { count: stage.failed });
  }
  if ((stage.retrying ?? 0) > 0 && !(stage.running ?? 0)) {
    return t("monitor.processing.line.retrying", "{{count}} retrying", { count: stage.retrying });
  }
  return t("monitor.processing.line.running", "{{count}} running", { count: stage.running ?? 0 });
}
