import { expect, test } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { StatMonitor } from "./StatMonitor";

test("current file work stays separate from large delivery history", async () => {
  let requests = 0;
  const response = {
    deliveries: {
      queues: [{ name: "catalog_macro", total_jobs: 29935, remaining_jobs: 10 }],
      available: 1,
      scheduled: 2,
      running: 3,
      retryable: 4,
      completed: 29911,
      discarded: 9,
      cancelled: 5,
    },
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
  } satisfies components["schemas"]["handler.ProcessingMonitorResponse"];
  worker.use(
    http.get("*/api/v1/admin/monitor/processing", () => {
      requests += 1;
      return HttpResponse.json(response);
    }),
  );
  const screen = await renderWithProviders(<StatMonitor />);

  // Every work type keeps its exact pending count as text, and the tray tier is
  // derived from that count rather than from the canvas.
  const expectedTiers: [string, string, string][] = [
    [t("monitor.processing.pendingAssets"), "7", "oneThird"],
    [t("monitor.processing.pendingRepositories"), "1", "oneThird"],
    [t("monitor.processing.pendingProjections"), "6", "oneThird"],
    [t("monitor.processing.pendingOperations"), "0", "empty"],
  ];
  for (const [label, pending, tier] of expectedTiers) {
    const tray = screen.getByRole("region", { name: label });
    await expect.element(tray.getByText(pending, { exact: true })).toBeVisible();
    await expect.element(tray.element().querySelector("figure")).toHaveAttribute("data-tier", tier);
  }

  await expect
    .element(
      screen.getByRole("heading", {
        name: t("monitor.queueSummary.queues.catalog_macro.name"),
        exact: true,
      }),
    )
    .toBeVisible();
  expect(requests).toBe(1);
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.poll(() => requests).toBe(2);

  // Retry qualifiers and historical deliveries are available on demand.
  await expect.element(screen.getByText("29911", { exact: true })).not.toBeInTheDocument();
  await screen.getByRole("button", { name: t("monitor.delivery.details"), exact: true }).click();
  await expect
    .element(screen.getByText(t("monitor.processing.retryWaiting", { count: 2 }), { exact: true }))
    .toBeVisible();
  // Delivery history stays separately labelled and unchanged.
  await expect.element(screen.getByText("29911", { exact: true })).toBeVisible();
  const active = screen.getByText(t("monitor.delivery.active"), { exact: true });
  await expect
    .element(active.element().parentElement?.querySelector("dd") ?? null)
    .toHaveTextContent("10");
  await expect
    .element(screen.getByText(t("monitor.delivery.description"), { exact: true }))
    .toBeVisible();
});

test("cleared work never reads as all-clear while problems remain", async () => {
  const response = {
    deliveries: {
      queues: [],
      available: 0,
      scheduled: 0,
      running: 0,
      retryable: 0,
      completed: 1200,
      discarded: 2,
      cancelled: 0,
    },
    processing: {
      pending_assets: 0,
      failed_assets: 3,
      retry_waiting_stages: 0,
      pending_repositories: 0,
      failed_repositories: 1,
      pending_projections: 0,
      failed_projections: 0,
      pending_operations: 0,
      failed_operations: 0,
    },
  } satisfies components["schemas"]["handler.ProcessingMonitorResponse"];
  worker.use(http.get("*/api/v1/admin/monitor/processing", () => HttpResponse.json(response)));
  const screen = await renderWithProviders(<StatMonitor />);

  // Every tray is empty, because no work is pending…
  for (const label of [
    t("monitor.processing.pendingAssets"),
    t("monitor.processing.pendingRepositories"),
    t("monitor.processing.pendingProjections"),
    t("monitor.processing.pendingOperations"),
  ]) {
    const tray = screen.getByRole("region", { name: label });
    await expect.element(tray.getByText("0", { exact: true })).toBeVisible();
    await expect
      .element(tray.element().querySelector("figure"))
      .toHaveAttribute("data-tier", "empty");
  }

  // …but the problems are still named, with counts, next to the tray that owns them.
  const assetsTray = screen.getByRole("region", {
    name: t("monitor.processing.pendingAssets"),
  });
  await expect
    .element(
      assetsTray.getByText(t("monitor.processing.attentionCount", { count: 3 }), { exact: true }),
    )
    .toBeVisible();
  const repositoriesTray = screen.getByRole("region", {
    name: t("monitor.processing.pendingRepositories"),
  });
  await expect
    .element(
      repositoriesTray.getByText(t("monitor.processing.attentionCount", { count: 1 }), {
        exact: true,
      }),
    )
    .toBeVisible();

  await screen.getByRole("button", { name: t("monitor.delivery.details"), exact: true }).click();
  // A historical discarded delivery is delivery history, not a current failure.
  await expect.element(screen.getByText("2", { exact: true })).toBeVisible();
});

test("failed statistics request does not display zero work as success", async () => {
  worker.use(
    http.get("*/api/v1/admin/monitor/processing", () =>
      HttpResponse.json(
        { type: "about:blank", title: "Fixture failure", status: 500 },
        { status: 500, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
  );
  const screen = await renderWithProviders(<StatMonitor />);
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.stats.fetchError"));
});

test("retains the last work count on refresh failure and never labels it ready", async () => {
  let failed = false;
  worker.use(
    http.get("*/api/v1/admin/monitor/processing", () =>
      failed
        ? HttpResponse.json({ title: "Fixture failure" }, { status: 500 })
        : HttpResponse.json({
            deliveries: { queues: [] },
            processing: { pending_assets: 7, failed_assets: 0 },
          }),
    ),
  );
  const screen = await renderWithProviders(<StatMonitor />);
  const files = screen.getByRole("region", {
    name: t("monitor.processing.pendingAssets"),
    exact: true,
  });
  await expect.element(files.getByText("7", { exact: true })).toBeVisible();
  failed = true;
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.snapshot.stale"));
  await expect.element(files.getByText("7", { exact: true })).toBeVisible();
});

test("discarded delivery history never becomes a current file failure", async () => {
  worker.use(
    http.get("*/api/v1/admin/monitor/processing", () =>
      HttpResponse.json({
        deliveries: { discarded: 999, queues: [] },
        processing: { pending_assets: 0, failed_assets: 0 },
      }),
    ),
  );
  const screen = await renderWithProviders(<StatMonitor />);
  const files = screen.getByRole("region", {
    name: t("monitor.processing.pendingAssets"),
    exact: true,
  });
  await expect.element(files.getByText("0", { exact: true })).toBeVisible();
  await expect
    .element(files.element().querySelector("figure"))
    .toHaveAttribute("data-tier", "empty");
  await expect.element(screen.getByText("999", { exact: true })).not.toBeInTheDocument();
  const repositories = screen.getByRole("region", {
    name: t("monitor.processing.pendingRepositories"),
    exact: true,
  });
  await expect
    .element(repositories.getByText(t("monitor.processing.noData"), { exact: true }))
    .toBeVisible();
  await expect.element(repositories.element().querySelector("figure")).not.toBeInTheDocument();
});
