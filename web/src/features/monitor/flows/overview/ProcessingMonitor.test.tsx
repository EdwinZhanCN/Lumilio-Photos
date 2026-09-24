import { expect, test } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { ProcessingMonitor } from "./ProcessingMonitor";

type Stage = components["schemas"]["processing.StageSummary"];

function stage(
  id: Stage["id"],
  group: "media" | "catalog",
  counts: Partial<Pick<Stage, "queued" | "running" | "retrying" | "failed" | "done">> = {},
  status: Stage["status"] = "idle",
): Stage {
  const queued = counts.queued ?? 0;
  const running = counts.running ?? 0;
  const retrying = counts.retrying ?? 0;
  return {
    id,
    group,
    unit:
      id === "scan"
        ? "repositories"
        : id === "backup"
          ? "runs"
          : group === "media"
            ? "files"
            : "updates",
    status,
    queued,
    running,
    retrying,
    failed: counts.failed ?? 0,
    remaining: queued + running + retrying,
    done: counts.done,
    retryable: !["import", "scan", "backup"].includes(id ?? ""),
  };
}

const summary = {
  generated_at: "2026-09-22T20:26:00Z",
  overview: {
    media_total: 2000,
    media_in_progress: 248,
    running: 3,
    failed_media: 2,
    catalog_pending: 1,
  },
  stages: [
    stage("import", "media"),
    stage("scan", "media"),
    stage("metadata", "media", { queued: 0, failed: 2, done: 1998 }, "attention"),
    stage("thumbnails", "media", { queued: 114, running: 3, done: 1880 }, "working"),
    stage("video", "media", { done: 218 }),
    stage("analysis", "media", { queued: 96, done: 1640 }, "waiting"),
    stage("events", "catalog", { queued: 1 }, "waiting"),
    stage("places", "catalog"),
    stage("text_search", "catalog"),
    stage("backup", "catalog"),
  ],
} satisfies components["schemas"]["processing.Summary"];

const failedItems = {
  items: [
    {
      subject_id: "asset-1",
      asset_id: "asset-1",
      label: "IMG_4471.HEIC",
      reason_code: "unsupported_media",
      attempts: 1,
      updated_at: "2026-09-22T20:10:00Z",
    },
    {
      subject_id: "asset-2",
      asset_id: "asset-2",
      label: "clip_0412.mov",
      reason_code: "processing_retry_exhausted",
      attempts: 8,
      updated_at: "2026-09-22T20:00:00Z",
    },
  ],
} satisfies components["schemas"]["processing.ItemsPage"];

function card(screen: Awaited<ReturnType<typeof renderWithProviders>>, key: string) {
  return screen.getByRole("button", { name: t(key), exact: true });
}

test("every stage is a card, and failures never read as idle even with nothing remaining", async () => {
  worker.use(http.get("*/api/v1/admin/processing", () => HttpResponse.json(summary)));
  const screen = await renderWithProviders(<ProcessingMonitor />);

  const metadata = card(screen, "monitor.processing.stages.metadata");
  await expect.element(metadata).toHaveTextContent(t("monitor.processing.status.attention"));
  await expect
    .element(metadata)
    .toHaveTextContent(t("monitor.processing.line.failed", { count: 2 }));
  await expect.element(metadata).not.toHaveTextContent(t("monitor.processing.status.idle"));
  await expect
    .element(card(screen, "monitor.processing.stages.thumbnails"))
    .toHaveTextContent("117");
  expect(screen.container.querySelectorAll("[aria-pressed]")).toHaveLength(10);

  // Nothing selected: the panel is the overview, with the tray and totals.
  await expect
    .element(screen.getByRole("heading", { name: t("monitor.processing.overview.title") }))
    .toBeVisible();
  await expect.element(screen.getByText("248", { exact: true })).toBeVisible();
  expect(screen.container.querySelectorAll("figure")).toHaveLength(1);
});

test("selecting a stage turns the panel into its detail and Back restores the overview", async () => {
  const retries: string[] = [];
  const reprocess: unknown[] = [];
  worker.use(
    http.get("*/api/v1/admin/processing", () => HttpResponse.json(summary)),
    http.get("*/api/v1/admin/processing/stages/metadata/items", ({ request }) => {
      const state = new URL(request.url).searchParams.get("state");
      return HttpResponse.json(state === "failed" ? failedItems : { items: [] });
    }),
    http.post("*/api/v1/admin/processing/stages/:stage/retry", ({ params }) => {
      retries.push(String(params.stage));
      return HttpResponse.json({ accepted: 2, remaining: 0 });
    }),
    http.post("*/api/v1/assets/:id/reprocess", async ({ params, request }) => {
      reprocess.push({ id: params.id, body: await request.json() });
      return HttpResponse.json({ asset_id: params.id, receipt_id: "r", status: "queued" });
    }),
  );
  const screen = await renderWithProviders(<ProcessingMonitor />);

  const metadata = card(screen, "monitor.processing.stages.metadata");
  await metadata.click();
  await expect.element(metadata).toHaveAttribute("aria-pressed", "true");
  await expect
    .element(
      screen.getByRole("heading", { name: t("monitor.processing.stages.metadata"), level: 2 }),
    )
    .toBeVisible();
  await expect.element(screen.getByText("IMG_4471.HEIC")).toBeVisible();
  await expect.element(screen.getByText(t("monitor.processing.reasons.unsupported"))).toBeVisible();
  await expect.element(screen.getByText(t("monitor.processing.reasons.exhausted"))).toBeVisible();
  await expect.element(screen.getByText("1,998", { exact: true })).toBeVisible();

  await screen
    .getByRole("button", { name: t("monitor.processing.detail.retry"), exact: true })
    .first()
    .click();
  await expect.poll(() => reprocess).toEqual([{ id: "asset-1", body: { tasks: ["analyze"] } }]);

  await screen.getByRole("button", { name: t("monitor.processing.detail.retryAll") }).click();
  await expect.poll(() => retries).toEqual(["metadata"]);

  await screen.getByRole("button", { name: t("monitor.processing.detail.back") }).click();
  await expect
    .element(screen.getByRole("heading", { name: t("monitor.processing.overview.title") }))
    .toBeVisible();
  await expect.element(metadata).toHaveAttribute("aria-pressed", "false");
});

test("a stage that cannot be retried offers no retry", async () => {
  worker.use(
    http.get("*/api/v1/admin/processing", () =>
      HttpResponse.json({
        ...summary,
        stages: summary.stages.map((item) =>
          item.id === "backup" ? { ...item, failed: 1, status: "attention" } : item,
        ),
      }),
    ),
    http.get("*/api/v1/admin/processing/stages/backup/items", () =>
      HttpResponse.json({
        items: [
          {
            subject_id: "r1",
            label: "catalog",
            reason_code: "processing_failed",
            updated_at: "2026-09-22T20:00:00Z",
          },
        ],
      }),
    ),
  );
  const screen = await renderWithProviders(<ProcessingMonitor />);
  await card(screen, "monitor.processing.stages.backup").click();
  await expect.element(screen.getByText(t("monitor.processing.reasons.failed"))).toBeVisible();
  await expect
    .element(screen.getByRole("button", { name: t("monitor.processing.detail.retryAll") }))
    .not.toBeInTheDocument();
  await expect
    .element(
      screen.getByRole("button", { name: t("monitor.processing.detail.retry"), exact: true }),
    )
    .not.toBeInTheDocument();
});

test("a failed refresh keeps the last snapshot and says it is stale", async () => {
  let failed = false;
  worker.use(
    http.get("*/api/v1/admin/processing", () =>
      failed
        ? HttpResponse.json({ title: "Fixture failure" }, { status: 500 })
        : HttpResponse.json(summary),
    ),
  );
  const screen = await renderWithProviders(<ProcessingMonitor />);
  await expect.element(screen.getByText("248", { exact: true })).toBeVisible();
  failed = true;
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.snapshot.stale"));
  await expect.element(screen.getByText("248", { exact: true })).toBeVisible();
});

test("diagnostics load only when opened and keep delivery records apart", async () => {
  let diagnosticsReads = 0;
  worker.use(
    http.get("*/api/v1/admin/processing", () => HttpResponse.json(summary)),
    http.get("*/api/v1/admin/processing/diagnostics", () => {
      diagnosticsReads += 1;
      return HttpResponse.json({
        completed: 29911,
        queues: [{ name: "catalog_macro", total_jobs: 29935, remaining_jobs: 10 }],
      });
    }),
  );
  const screen = await renderWithProviders(<ProcessingMonitor />);
  await expect.element(screen.getByText("248", { exact: true })).toBeVisible();
  expect(diagnosticsReads).toBe(0);
  await screen
    .getByRole("button", { name: t("monitor.processing.diagnostics.title"), exact: true })
    .click();
  const dialog = screen.getByRole("dialog");
  await expect.element(dialog.getByText("29911", { exact: true })).toBeVisible();
  await expect
    .element(
      dialog.getByRole("heading", { name: t("monitor.queueSummary.queues.catalog_macro.name") }),
    )
    .toBeVisible();
  expect(diagnosticsReads).toBeGreaterThan(0);
});
