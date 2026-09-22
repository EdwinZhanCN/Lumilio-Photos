import { expect, test } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { http, HttpResponse, worker } from "@test/msw";
import { t } from "@test/i18n";
import type { components } from "@/lib/http-commons/schema";
import { MLMonitor } from "./MLMonitor";

test("shows five coverage fields with local rebuild actions and no job backlog", async () => {
  const reads: string[] = [];
  const rebuilds: unknown[] = [];
  worker.use(
    http.get("*/api/v1/assets/indexing/stats", ({ request }) => {
      reads.push(new URL(request.url).searchParams.get("repository_id") ?? "");
      return HttpResponse.json({
        photo_total: 200,
        video_total: 0,
        reindex_jobs: 2,
        tasks: {
          semantic: { indexed_count: 120, total_count: 200, queued_jobs: 7 },
          video_semantic: { indexed_count: 0, total_count: 0, queued_jobs: 7 },
          face: { indexed_count: 200, total_count: 200, queued_jobs: 7 },
          ocr: { indexed_count: 0, total_count: 200, queued_jobs: 7 },
          bioclip: { indexed_count: 3, total_count: 10, queued_jobs: 7 },
        },
      } satisfies components["schemas"]["dto.AssetIndexingStatsResponseDTO"]);
    }),
    http.post("*/api/v1/assets/indexing/rebuild", async ({ request }) => {
      rebuilds.push(await request.json());
      return HttpResponse.json({ accepted: true, disabled_tasks: [] });
    }),
  );
  const screen = await renderWithProviders(<MLMonitor localRepoId="fixture-repo" />);
  const semantic = screen.getByRole("region", {
    name: t("settings.aiSettings.taskNames.semantic"),
    exact: true,
  });
  await expect.element(semantic.getByText("60%", { exact: true })).toBeVisible();
  const video = screen.getByRole("region", {
    name: t("settings.aiSettings.taskNames.videoSemantic"),
    exact: true,
  });
  await expect.element(video.getByText(t("monitor.ml.noApplicable"))).toBeVisible();
  // Coverage only: no backlog, cross-tab link, or hidden legend.
  expect(screen.container.querySelector("a, details")).toBeNull();
  await semantic.getByRole("button", { name: t("monitor.ml.reindex"), exact: true }).click();
  await expect
    .element(screen.getByText(t("monitor.ml.reindexModal.descriptionMissing", { count: 80 })))
    .toBeVisible();
  await expect
    .element(screen.getByText(t("monitor.ml.reindexModal.existingJobsWarning", { count: 2 })))
    .toBeVisible();
  await screen
    .getByRole("button", { name: t("monitor.ml.reindexModal.confirm"), exact: true })
    .click();
  await expect
    .poll(() => rebuilds)
    .toEqual([{ repository_id: "fixture-repo", tasks: ["semantic"], missing_only: true }]);
  expect(reads.every((scope) => scope === "fixture-repo")).toBe(true);
  await screen
    .getByRole("region", { name: t("settings.aiSettings.taskNames.face"), exact: true })
    .getByRole("button", { name: t("monitor.ml.reindex"), exact: true })
    .click();
  await expect.element(screen.getByRole("checkbox")).toBeChecked();
  await screen
    .getByRole("button", { name: t("monitor.ml.reindexModal.cancel"), exact: true })
    .click();
  await expect.element(video.getByRole("button")).not.toBeInTheDocument();
  const bio = screen.getByRole("region", { name: t("monitor.ml.bioAlbumCoverage"), exact: true });
  await expect.element(bio.getByRole("button")).not.toBeInTheDocument();
});

test("retains successful coverage and marks a failed refresh stale", async () => {
  let failed = false;
  worker.use(
    http.get("*/api/v1/assets/indexing/stats", () =>
      failed
        ? HttpResponse.json({ title: "Fixture failure" }, { status: 500 })
        : HttpResponse.json({
            photo_total: 200,
            tasks: { semantic: { indexed_count: 120, total_count: 200 } },
          }),
    ),
  );
  const screen = await renderWithProviders(<MLMonitor />);
  await expect.element(screen.getByText("60%", { exact: true })).toBeVisible();
  failed = true;
  await screen
    .getByRole("button", { name: t("settings.serverSettings.refresh"), exact: true })
    .click();
  await expect.element(screen.getByRole("alert")).toHaveTextContent(t("monitor.snapshot.stale"));
  await expect.element(screen.getByText("60%", { exact: true })).toBeVisible();
  await expect
    .element(
      screen
        .getByRole("region", { name: t("settings.aiSettings.taskNames.semantic"), exact: true })
        .getByRole("button", { name: t("monitor.ml.reindex"), exact: true }),
    )
    .toBeDisabled();
});
