import { readFileSync } from "node:fs";
import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { t } from "../support/i18n";

type BrowseItem = {
  asset?: { asset_id?: string; original_filename?: string };
  best_ts_ms?: number;
};

type SearchResponse = {
  top_items?: BrowseItem[];
  result_items?: BrowseItem[];
};

test("@smoke video semantic search opens the best matching frame", async ({
  page,
  workspace,
}) => {
  test.setTimeout(150_000);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/manage");
  await page
    .getByLabel(t("upload.UnifiedUploadSection.upload_target_label"))
    .selectOption({ label: workspace.repositoryName });
  await page.locator('input[type="file"]').setInputFiles({
    name: workspace.videoFilename,
    mimeType: "video/mp4",
    buffer: readFileSync(workspace.videoSource),
  });

  const accepted = page.waitForResponse(
    (response) =>
      /\/api\/v1\/assets(\/batch)?$/.test(new URL(response.url()).pathname) &&
      response.request().method() === "POST" &&
      response.ok(),
    { timeout: 60_000 },
  );
  await page
    .getByRole("button", {
      name: t("upload.UnifiedUploadSection.upload_button", { countLabel: " (1)" }),
    })
    .click();
  await accepted;

  let matchedItem: BrowseItem | undefined;
  await expect
    .poll(
      async () => {
        const response = await api<SearchResponse>("/api/v1/assets/search", {
          method: "POST",
          token: workspace.token,
          body: JSON.stringify({
            query: "ocean waves",
            filter: { repository_id: workspace.repositoryId },
            pagination: { limit: 20, offset: 0 },
            enhancement_mode: "auto",
            top_results_limit: 20,
            stack_mode: "collapsed",
          }),
        });
        matchedItem = [...(response.top_items ?? []), ...(response.result_items ?? [])].find(
          (item) => item.asset?.original_filename === workspace.videoFilename,
        );
        return matchedItem?.best_ts_ms ?? 0;
      },
      {
        message: "video frame embeddings should become searchable",
        timeout: 120_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toBeGreaterThan(0);

  const bestTsMs = matchedItem?.best_ts_ms;
  const assetId = matchedItem?.asset?.asset_id;
  expect(bestTsMs).toBeTruthy();
  expect(assetId).toBeTruthy();

  await new GalleryPage(page).scopeTo(workspace.repositoryName);
  await page.getByRole("button", { name: t("assets.searchAriaLabel") }).click();
  const uiSearchCompleted = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/v1/assets/search" &&
      response.request().method() === "POST" &&
      response.ok(),
    { timeout: 30_000 },
  );
  await page.getByRole("searchbox", { name: t("assets.searchAriaLabel") }).fill("ocean waves");
  await uiSearchCompleted;
  await expect(page.getByLabel(new RegExp(workspace.videoFilename, "i"))).toBeVisible({
    timeout: 30_000,
  });
  await page.getByLabel(new RegExp(workspace.videoFilename, "i")).click();

  await expect
    .poll(() => new URL(page.url()).searchParams.get("t_ms"))
    .toBe(String(bestTsMs));
  await expect(page).toHaveURL(new RegExp(`/assets/${assetId}`));

  const video = page.locator("video");
  await expect(video).toBeVisible({ timeout: 30_000 });
  await expect
    .poll(
      async () =>
        video.evaluate(
          (element, expectedSeconds) =>
            Math.abs((element as HTMLVideoElement).currentTime - expectedSeconds) < 0.5,
          (bestTsMs ?? 0) / 1_000,
        ),
      { timeout: 30_000 },
    )
    .toBe(true);
});
