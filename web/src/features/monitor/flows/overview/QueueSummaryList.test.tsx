import { describe, expect, it, vi } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { QueueSummaryList } from "./QueueSummaryList";

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

function serveSummary() {
  worker.use(
    http.get("/api/v1/admin/river/queue-summary", () => HttpResponse.json(summaryResponse)),
  );
}

describe("QueueSummaryList", () => {
  it("renders each queue as a processing area with aggregate metrics", async () => {
    serveSummary();
    const screen = await renderWithProviders(<QueueSummaryList />);

    await expect.element(screen.getByRole("heading", { name: "Previews" })).toBeInTheDocument();
    await expect
      .element(screen.getByText("Creates previews used throughout the gallery."))
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
      .element(screen.getByText("thumbnail failed: decode error"))
      .toBeInTheDocument();

    await screen.getByRole("button", { name: "Copy error" }).click();

    await vi.waitFor(() => {
      expect(clipboardWriteText).toHaveBeenCalledWith(
        expect.stringContaining("queue=thumbnail_asset"),
      );
    });
    expect(clipboardWriteText).toHaveBeenCalledWith(expect.stringContaining("job_id=42"));
    expect(clipboardWriteText).toHaveBeenCalledWith(
      expect.stringContaining("thumbnail failed: decode error"),
    );
  });
});
