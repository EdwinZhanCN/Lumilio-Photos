import { describe, expect, it } from "vite-plus/test";
import { translateDouble } from "@test/translateDouble";
import {
  MACRO_JOB_KINDS,
  buildDiagnosticText,
  getKindLabels,
  getQueueCopy,
  isMacroJobKind,
  queueStatus,
  queueStatusLabel,
  unknownKindLabel,
  type MacroJobKind,
  type QueueStatus,
} from "./queueVocabulary";

/** The closed macro-job catalog, member by member, as a key contract. */
const KIND_KEYS: Record<MacroJobKind, string> = {
  analyze_asset: "monitor.queueSummary.kinds.analyze_asset",
  backup_catalog: "monitor.queueSummary.kinds.backup_catalog",
  enrich_asset: "monitor.queueSummary.kinds.enrich_asset",
  generate_asset_derivatives: "monitor.queueSummary.kinds.generate_asset_derivatives",
  ingest_asset: "monitor.queueSummary.kinds.ingest_asset",
  rebuild_projection_batch: "monitor.queueSummary.kinds.rebuild_projection_batch",
  scan_repository_batch: "monitor.queueSummary.kinds.scan_repository_batch",
  transcode_media: "monitor.queueSummary.kinds.transcode_media",
};

const STATUS_KEYS: Record<QueueStatus, string> = {
  needsAttention: "monitor.queueSummary.status.needsAttention",
  working: "monitor.queueSummary.status.working",
  pendingWork: "monitor.queueSummary.status.pendingWork",
  settled: "monitor.queueSummary.status.settled",
  idle: "monitor.queueSummary.status.idle",
};

describe("macro job vocabulary", () => {
  it("labels exactly the closed macro-job set", () => {
    const labels = getKindLabels(translateDouble().t);
    expect(Object.keys(labels).sort()).toEqual([...MACRO_JOB_KINDS].sort());
  });

  it("routes every kind to its own key", () => {
    const labels = getKindLabels(translateDouble().t);
    for (const kind of MACRO_JOB_KINDS) {
      expect(labels[kind], kind).toBe(KIND_KEYS[kind]);
    }
  });

  it("recognizes members and rejects in-process tasks", () => {
    for (const kind of MACRO_JOB_KINDS) {
      expect(isMacroJobKind(kind), kind).toBe(true);
    }
    expect(isMacroJobKind("thumbnail_step")).toBe(false);
    expect(isMacroJobKind(undefined)).toBe(false);
  });

  it("keeps a kind the Web does not know readable", () => {
    const { t, calls } = translateDouble();
    expect(unknownKindLabel(t, "future_kind")).toBe("monitor.queueSummary.kinds.unknown");
    expect(calls[0]).toEqual({
      key: "monitor.queueSummary.kinds.unknown",
      options: {
        defaultValue: "Other background task · {{name}}",
        name: "future_kind",
      },
    });
  });

  it("falls back to a generic name when the kind is missing", () => {
    const { t, calls } = translateDouble();
    unknownKindLabel(t, undefined);
    expect(calls.map((call) => call.key)).toEqual([
      "common.unknown",
      "monitor.queueSummary.kinds.unknown",
    ]);
  });
});

describe("queue naming", () => {
  it("names the one canonical macro queue", () => {
    const copy = getQueueCopy(translateDouble().t);
    expect(Object.keys(copy)).toEqual(["catalog_macro"]);
    expect(copy.catalog_macro.name).toBe("monitor.queueSummary.queues.catalog_macro.name");
  });
});

describe("queueStatus", () => {
  it("ranks attention above all remaining work", () => {
    expect(
      queueStatus({ attention_jobs: 1, remaining_jobs: 5, running_jobs: 2, total_jobs: 10 }),
    ).toBe("needsAttention");
  });

  it("separates running remaining work from merely pending work", () => {
    expect(queueStatus({ remaining_jobs: 5, running_jobs: 2, total_jobs: 10 })).toBe("working");
    expect(queueStatus({ remaining_jobs: 5, running_jobs: 0, total_jobs: 10 })).toBe("pendingWork");
  });

  it("treats finished history as settled and an empty queue as idle", () => {
    expect(queueStatus({ total_jobs: 10, processed_jobs: 10 })).toBe("settled");
    expect(queueStatus({})).toBe("idle");
  });

  it("maps every status to its own label key", () => {
    const { t } = translateDouble();
    for (const status of Object.keys(STATUS_KEYS) as QueueStatus[]) {
      expect(queueStatusLabel(t, status), status).toBe(STATUS_KEYS[status]);
    }
  });
});

describe("buildDiagnosticText", () => {
  it("emits the operator block copied into reports", () => {
    expect(
      buildDiagnosticText("catalog_macro", {
        job_id: 42,
        kind: "generate_asset_derivatives",
        state: "retryable",
        attempt: 3,
        max_attempts: 50,
        last_error: "decode error",
      }),
    ).toBe(
      [
        "queue=catalog_macro",
        "job_id=42",
        "kind=generate_asset_derivatives",
        "state=retryable",
        "attempt=3/50",
        "created_at=",
        "scheduled_at=",
        "attempted_at=",
        "finalized_at=",
        "",
        "decode error",
      ].join("\n"),
    );
  });

  it("keeps every field present when the sample is sparse", () => {
    const text = buildDiagnosticText("catalog_macro", {});
    expect(text.split("\n").slice(0, 9)).toEqual([
      "queue=catalog_macro",
      "job_id=",
      "kind=",
      "state=",
      "attempt=0/0",
      "created_at=",
      "scheduled_at=",
      "attempted_at=",
      "finalized_at=",
    ]);
  });
});
