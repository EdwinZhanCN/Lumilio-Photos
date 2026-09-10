import { expect, test } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { StatMonitor } from "./StatMonitor";

test("current file work stays separate from large delivery history", async () => {
  const response = {
    available: 1,
    scheduled: 2,
    running: 3,
    retryable: 4,
    completed: 29911,
    discarded: 9,
    cancelled: 5,
    processing: {
      pending_assets: 7,
      failed_assets: 0,
      retry_waiting_stages: 2,
      pending_repositories: 1,
      failed_repositories: 0,
      pending_projections: 6,
      failed_projections: 0,
      pending_operations: 0,
      failed_operations: 0,
    },
  } satisfies components["schemas"]["handler.JobStatsResponse"];
  worker.use(http.get("*/api/v1/admin/river/stats", () => HttpResponse.json(response)));
  const screen = await renderWithProviders(<StatMonitor />);
  const pending = screen.getByText(t("monitor.processing.pendingAssets"), { exact: true });
  await expect.element(pending).toBeVisible();
  await expect
    .element(pending.element().parentElement?.querySelector("dd") ?? null)
    .toHaveTextContent("7");
  const failed = screen.getByText(t("monitor.processing.failedAssets"), { exact: true });
  await expect
    .element(failed.element().parentElement?.querySelector("dd") ?? null)
    .toHaveTextContent("0");
  await expect.element(screen.getByText("29911", { exact: true })).toBeVisible();
  const active = screen.getByText(`${t("monitor.delivery.active")}:`, { exact: true });
  await expect
    .element(active.element().parentElement?.querySelector("dd") ?? null)
    .toHaveTextContent("10");
  await expect
    .element(screen.getByText(t("monitor.delivery.description"), { exact: true }))
    .toBeVisible();
});

test("failed statistics request does not display zero work as success", async () => {
  worker.use(
    http.get("*/api/v1/admin/river/stats", () =>
      HttpResponse.json(
        { type: "about:blank", title: "Fixture failure", status: 500 },
        { status: 500, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
  );
  const screen = await renderWithProviders(<StatMonitor />);
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.stats.fetchError"));
});
