import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { QueueSummaryList } from "./QueueSummaryList";

const mocks = vi.hoisted(() => ({
  useQuery: vi.fn(),
  queryResult: {} as any,
}));

const translations: Record<string, string> = {
  "monitor.queueSummary.copyError": "Copy error",
  "monitor.queueSummary.copied": "Copied",
  "monitor.queueSummary.errorAttempt": "Attempt {{current}} / {{max}}",
  "monitor.queueSummary.errorFallback": "No error message was recorded.",
  "monitor.queueSummary.errorStates.retryable": "Will retry",
  "monitor.queueSummary.errorStates.unknown": "Needs attention",
  "monitor.queueSummary.kinds.thumbnail_asset": "Build previews",
  "monitor.queueSummary.metrics.attention": "Needs attention",
  "monitor.queueSummary.metrics.averageLatency": "Avg total time",
  "monitor.queueSummary.metrics.averageRuntime": "Avg work time",
  "monitor.queueSummary.metrics.latestActivity": "Latest activity {{value}}",
  "monitor.queueSummary.metrics.notEnoughData": "-",
  "monitor.queueSummary.metrics.oldestRemaining": "Oldest remaining {{value}}",
  "monitor.queueSummary.metrics.processed": "Processed",
  "monitor.queueSummary.metrics.remaining": "Remaining",
  "monitor.queueSummary.metrics.total": "Total",
  "monitor.queueSummary.queues.default.description": "Background work for the photo library.",
  "monitor.queueSummary.queues.thumbnail_asset.description": "Creates previews used throughout the gallery.",
  "monitor.queueSummary.queues.thumbnail_asset.name": "Previews",
  "monitor.queueSummary.reviewErrors_other": "Review {{count}} issues",
  "monitor.queueSummary.status.needsAttention": "Needs attention",
  "monitor.queueSummary.subtitle": "Grouped by the part of the photo library pipeline doing the work.",
  "monitor.queueSummary.time.justNow": "just now",
  "monitor.queueSummary.time.minutesAgo_other": "{{count}} minutes ago",
  "monitor.queueSummary.time.never": "never",
  "monitor.queueSummary.title": "Processing activity",
  "monitor.queueSummary.updated": "Updated {{time}}",
};

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    i18n: { language: "en", resolvedLanguage: "en" },
    t: (key: string, options?: Record<string, unknown>) => {
      const pluralKey =
        typeof options?.count === "number"
          ? `${key}_${options.count === 1 ? "one" : "other"}`
          : key;
      let value =
        translations[pluralKey] ??
        translations[key] ??
        (options?.defaultValue as string | undefined) ??
        key;

      for (const [name, replacement] of Object.entries(options ?? {})) {
        value = value.replaceAll(`{{${name}}}`, String(replacement));
      }
      return value;
    },
  }),
}));

vi.mock("@/lib/http-commons/queryClient", () => ({
  $api: {
    useQuery: mocks.useQuery,
  },
}));

const now = new Date("2026-06-12T12:00:00.000Z").toISOString();
const oneMinuteAgo = new Date("2026-06-12T11:59:00.000Z").toISOString();

const summaryResponse = {
  generated_at: now,
  queues: [
    {
      name: "thumbnail_asset",
      total_jobs: 100,
      processed_jobs: 80,
      remaining_jobs: 20,
      running_jobs: 1,
      attention_jobs: 2,
      average_latency_ms: 5000,
      average_runtime_ms: 1200,
      latest_activity_at: now,
      oldest_remaining_at: oneMinuteAgo,
      error_samples: [
        {
          job_id: 42,
          kind: "thumbnail_asset",
          state: "retryable",
          attempt: 3,
          max_attempts: 50,
          created_at: oneMinuteAgo,
          scheduled_at: now,
          attempted_at: now,
          last_error: "thumbnail failed: decode error",
        },
      ],
    },
  ],
};

let clipboardWriteText: ReturnType<typeof vi.fn>;

describe("QueueSummaryList", () => {
  beforeEach(() => {
    mocks.queryResult = {
      data: summaryResponse,
      isError: false,
      isLoading: false,
    };
    mocks.useQuery.mockImplementation(() => mocks.queryResult);
    clipboardWriteText = vi.fn().mockResolvedValue(undefined);

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: clipboardWriteText,
      },
    });
  });

  afterEach(() => {
    cleanup();
  });

  it("renders each queue as a processing area with aggregate metrics", () => {
    render(<QueueSummaryList />);

    expect(screen.getByText("Previews")).toBeInTheDocument();
    expect(
      screen.getByText("Creates previews used throughout the gallery."),
    ).toBeInTheDocument();
    expect(screen.getByText("Total")).toBeInTheDocument();
    expect(screen.getByText("100")).toBeInTheDocument();
    expect(screen.getByText("Processed")).toBeInTheDocument();
    expect(screen.getByText("80")).toBeInTheDocument();
    expect(screen.getByText("Remaining")).toBeInTheDocument();
    expect(screen.getByText("20")).toBeInTheDocument();
    expect(screen.getAllByText("Needs attention").length).toBeGreaterThan(0);
    expect(screen.getByText("2")).toBeInTheDocument();
  });

  it("expands queue issues and copies diagnostic details", async () => {
    render(<QueueSummaryList />);

    fireEvent.click(screen.getByRole("button", { name: "Review 2 issues" }));

    expect(screen.getByText("Build previews")).toBeInTheDocument();
    expect(screen.getByText("thumbnail failed: decode error")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Copy error" }));

    await waitFor(() => {
      expect(clipboardWriteText).toHaveBeenCalledWith(
        expect.stringContaining("queue=thumbnail_asset"),
      );
    });
    expect(clipboardWriteText).toHaveBeenCalledWith(
      expect.stringContaining("job_id=42"),
    );
    expect(clipboardWriteText).toHaveBeenCalledWith(
      expect.stringContaining("thumbnail failed: decode error"),
    );
  });
});
