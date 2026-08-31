import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { toast } from "sonner";
import i18n from "@/lib/i18n";
import type { QueueSummaryResponse } from "../../types";
import { QueueSummaryList } from "./QueueSummaryList";

const now = new Date("2026-06-12T12:00:00.000Z").toISOString();
const oneMinuteAgo = new Date("2026-06-12T11:59:00.000Z").toISOString();

const summaryResponse = {
  generated_at: now,
  queues: [
    {
      name: "catalog_macro",
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
          kind: "generate_asset_derivatives",
          state: "retryable",
          attempt: 3,
          max_attempts: 50,
          created_at: oneMinuteAgo,
          scheduled_at: now,
          attempted_at: now,
          last_error: "derivative generation failed: decode error",
        },
      ],
    },
  ],
} satisfies QueueSummaryResponse;

function serveSummary(response: QueueSummaryResponse = summaryResponse) {
  worker.use(http.get("/api/v1/admin/river/queue-summary", () => HttpResponse.json(response)));
}

describe("QueueSummaryList", () => {
  beforeEach(async () => {
    await i18n.changeLanguage("en");
  });

  afterEach(async () => {
    await i18n.changeLanguage("en");
  });

  it("renders each queue as a processing area with aggregate metrics", async () => {
    serveSummary();
    const screen = await renderWithProviders(<QueueSummaryList />);

    await expect
      .element(screen.getByRole("heading", { name: "Catalog processing" }))
      .toBeInTheDocument();
    await expect
      .element(
        screen.getByText("Runs bounded catalog pipeline stages with shared resource limits."),
      )
      .toBeInTheDocument();
    await expect.element(screen.getByText("Total", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("100", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("Processed", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("80", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("Remaining", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("20", { exact: true })).toBeInTheDocument();
    await expect.element(screen.getByText("Needs attention").first()).toBeInTheDocument();
    await expect.element(screen.getByText("2", { exact: true })).toBeInTheDocument();
  });

  it("expands queue issues and copies diagnostic details", async () => {
    serveSummary();
    const clipboardWriteText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: clipboardWriteText },
    });

    const screen = await renderWithProviders(<QueueSummaryList />);

    await screen.getByRole("button", { name: "Review 2 issues" }).click();

    await expect.element(screen.getByText("Build previews")).toBeInTheDocument();
    await expect
      .element(screen.getByText("derivative generation failed: decode error"))
      .toBeInTheDocument();

    await screen.getByRole("button", { name: "Copy error" }).click();

    await vi.waitFor(() => {
      expect(clipboardWriteText).toHaveBeenCalledWith(
        expect.stringContaining("queue=catalog_macro"),
      );
    });
    expect(clipboardWriteText).toHaveBeenCalledWith(expect.stringContaining("job_id=42"));
    expect(clipboardWriteText).toHaveBeenCalledWith(
      expect.stringContaining("derivative generation failed: decode error"),
    );
  });

  it("reports copy failures instead of leaving an unhandled rejection", async () => {
    serveSummary();
    const toastError = vi.spyOn(toast, "error").mockImplementation(() => "copy-error");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: undefined,
    });
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      value: vi.fn(() => false),
    });
    const screen = await renderWithProviders(<QueueSummaryList />);

    await screen.getByRole("button", { name: "Review 2 issues" }).click();
    await screen.getByRole("button", { name: "Copy error" }).click();

    await vi.waitFor(() => {
      expect(toastError).toHaveBeenCalledWith("Copy failed.", expect.any(Object));
    });
  });

  it("localizes the canonical macro queue", async () => {
    const queueNames = ["catalog_macro"] as const;
    serveSummary({
      generated_at: now,
      queues: queueNames.map((name) => ({
        name,
        total_jobs: 0,
        processed_jobs: 0,
        remaining_jobs: 0,
        running_jobs: 0,
        attention_jobs: 0,
        error_samples: [],
      })),
    });
    await i18n.changeLanguage("zh");

    const screen = await renderWithProviders(<QueueSummaryList />);

    for (const name of queueNames) {
      await expect
        .element(
          screen.getByRole("heading", {
            name: i18n.t(`monitor.queueSummary.queues.${name}.name`),
            exact: true,
          }),
        )
        .toBeInTheDocument();
    }
  });
});
