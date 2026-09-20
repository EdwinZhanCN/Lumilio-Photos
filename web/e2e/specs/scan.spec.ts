import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type StorageView = components["schemas"]["dto.StorageViewResponseDTO"];
type ScanAccepted = components["schemas"]["dto.RepositoryScanQueuedDTO"];
type ScanRun = components["schemas"]["dto.RepositoryScanRunDTO"];
type AssetList = components["schemas"]["dto.QueryAssetsResponseDTO"];

const ACTIVE_VERIFICATION_STATUSES = new Set(["queued", "crawling", "catching_up", "finalizing"]);

test("@smoke administrator scans a real repository file and sees it", async ({
  page,
  workspace,
}) => {
  test.setTimeout(240_000);
  await expect(async () => {
    const view = await api<StorageView>("/api/v1/storage/view", { token: workspace.token });
    const repository = view.repositories?.find(({ id }) => id === workspace.repositoryId);
    expect(repository?.activity).toBe("idle");
  }).toPass({ timeout: 90_000 });

  const queued = await api<ScanAccepted>(
    `/api/v1/storage/repositories/${workspace.repositoryId}/verifications`,
    {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({ force: true }),
    },
  );
  expect(queued.operation_id).toBeTruthy();
  await expect(async () => {
    const run = await api<ScanRun>(
      `/api/v1/storage/repositories/${workspace.repositoryId}/verifications/${queued.operation_id}`,
      { token: workspace.token },
    );
    expect(ACTIVE_VERIFICATION_STATUSES.has(run.status ?? "")).toBe(false);
    expect(["completed", "partial"]).toContain(run.status);
    expect(run.files_observed ?? 0).toBeGreaterThan(0);
    expect(run.outbox_depth ?? 0).toBe(0);
  }).toPass({ timeout: 90_000 });
  await expect(async () => {
    const assets = await api<AssetList>("/api/v1/assets/list", {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({
        query: workspace.scanFilename,
        search_type: "filename",
        filter: { repository_id: workspace.repositoryId },
        pagination: { limit: 10, offset: 0 },
        stack_mode: "expanded",
      }),
    });
    expect(assets.items ?? []).toHaveLength(1);
  }).toPass({ timeout: 90_000 });

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await new GalleryPage(page).scopeTo(workspace.repositoryName);
  await expect(page.getByLabel(new RegExp(workspace.scanFilename, "i"))).toBeVisible({
    timeout: 60_000,
  });
});
