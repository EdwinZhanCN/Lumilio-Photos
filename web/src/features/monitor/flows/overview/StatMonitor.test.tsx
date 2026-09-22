import { expect, test } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { StatMonitor } from "./StatMonitor";

type ProcessingMonitorResponse = components["schemas"]["handler.ProcessingMonitorResponse"];

function lane(screen: Awaited<ReturnType<typeof renderWithProviders>>, key: string) {
  return screen.getByRole("listitem", { name: t(key), exact: true });
}

test("files animate in one tray while other work is an exact list", async () => {
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
      pending_analysis_assets: 5,
      failed_analysis_assets: 0,
      pending_reindex_requests: 2,
    },
  } satisfies ProcessingMonitorResponse;
  worker.use(
    http.get("*/api/v1/admin/monitor/processing", () => {
      requests += 1;
      return HttpResponse.json(response);
    }),
  );
  const screen = await renderWithProviders(<StatMonitor />);

  // Files keep their exact count as text; the tray tier derives from that count.
  const files = screen.getByRole("region", {
    name: t("monitor.processing.pendingAssets"),
    exact: true,
  });
  await expect.element(files.getByText("7", { exact: true })).toBeVisible();
  await expect
    .element(files.element().querySelector("figure"))
    .toHaveAttribute("data-tier", "oneThird");
  // Retry waits are a file-work fact, shown beside the files they delay.
  await expect
    .element(files.getByText(t("monitor.processing.retryWaiting", { count: 2 }), { exact: true }))
    .toBeVisible();
  // Exactly one tray: every other kind of work is a list row, never a canvas.
  expect(screen.container.querySelectorAll("figure")).toHaveLength(1);

  const expectedLanes: [string, string, string][] = [
    ["monitor.processing.pendingRepositories", "1", "monitor.processing.status.working"],
    ["monitor.processing.pendingAnalysis", "5", "monitor.processing.status.working"],
    ["monitor.ml.pendingRebuilds", "2", "monitor.processing.status.working"],
    ["monitor.processing.pendingProjections", "6", "monitor.processing.status.working"],
    ["monitor.processing.pendingOperations", "0", "monitor.processing.status.clear"],
  ];
  for (const [label, pending, status] of expectedLanes) {
    const row = lane(screen, label);
    await expect.element(row.getByText(pending, { exact: true })).toBeVisible();
    await expect.element(row.getByText(t(status), { exact: true })).toBeVisible();
  }
  await expect
    .element(
      lane(screen, "monitor.processing.pendingAnalysis").getByRole("link", {
        name: t("monitor.processing.openCoverage"),
      }),
    )
    .toHaveAttribute("href", "/server-monitor?tab=ml");

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

  // Historical deliveries are available on demand and stay separately labelled.
  await expect.element(screen.getByText("29911", { exact: true })).not.toBeInTheDocument();
  await screen.getByRole("button", { name: t("monitor.delivery.details"), exact: true }).click();
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
      pending_analysis_assets: 0,
      failed_analysis_assets: 4,
      pending_reindex_requests: 0,
    },
  } satisfies ProcessingMonitorResponse;
  worker.use(http.get("*/api/v1/admin/monitor/processing", () => HttpResponse.json(response)));
  const screen = await renderWithProviders(<StatMonitor />);

  // The tray is empty, because no file work is pending…
  const files = screen.getByRole("region", {
    name: t("monitor.processing.pendingAssets"),
    exact: true,
  });
  await expect.element(files.getByText("0", { exact: true })).toBeVisible();
  await expect
    .element(files.element().querySelector("figure"))
    .toHaveAttribute("data-tier", "empty");
  // …but the problems are still named, with counts, next to the work that owns them.
  await expect
    .element(files.getByText(t("monitor.processing.attentionCount", { count: 3 }), { exact: true }))
    .toBeVisible();
  const repositories = lane(screen, "monitor.processing.pendingRepositories");
  await expect
    .element(
      repositories.getByText(t("monitor.processing.attentionCount", { count: 1 }), { exact: true }),
    )
    .toBeVisible();
  await expect
    .element(repositories.getByText(t("monitor.processing.status.attention"), { exact: true }))
    .toBeVisible();
  const analysis = lane(screen, "monitor.processing.pendingAnalysis");
  await expect
    .element(
      analysis.getByText(t("monitor.processing.attentionCount", { count: 4 }), { exact: true }),
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

test("missing counts read as no data rather than clear", async () => {
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
  const repositories = lane(screen, "monitor.processing.pendingRepositories");
  await expect
    .element(repositories.getByText(t("monitor.processing.noData"), { exact: true }))
    .toBeVisible();
  await expect
    .element(repositories.getByText(t("monitor.processing.status.clear"), { exact: true }))
    .not.toBeInTheDocument();
});
