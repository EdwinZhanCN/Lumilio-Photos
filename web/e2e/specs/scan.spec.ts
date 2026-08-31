import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type RepositoryOptions = components["schemas"]["dto.IndexingRepositoryListResponseDTO"];
type ScanAccepted = components["schemas"]["dto.RepositoryScanQueuedDTO"];
type AssetList = components["schemas"]["dto.QueryAssetsResponseDTO"];

test("@smoke administrator scans a real repository file and sees it", async ({
  page,
  workspace,
}) => {
  test.setTimeout(120_000);
  await expect(async () => {
    const options = await api<RepositoryOptions>("/api/v1/assets/indexing/repositories", {
      token: workspace.token,
    });
    const repository = options.repositories?.find(({ id }) => id === workspace.repositoryId);
    expect(repository?.activity).toBe("idle");
  }).toPass({ timeout: 90_000 });

  const queued = await api<ScanAccepted>(`/api/v1/repositories/${workspace.repositoryId}/scan`, {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ force: false }),
  });
  expect(queued.operation_id).toBeTruthy();
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
    expect(assets.items?.length).toBe(1);
  }).toPass({ timeout: 60_000 });

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await new GalleryPage(page).scopeTo(workspace.repositoryName);
  await expect(page.getByLabel(new RegExp(workspace.scanFilename, "i")).first()).toBeVisible({
    timeout: 60_000,
  });
});
