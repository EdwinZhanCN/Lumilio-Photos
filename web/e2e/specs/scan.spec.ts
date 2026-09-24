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

  const requestScan = () =>
    api<ScanAccepted>(`/api/v1/storage/repositories/${workspace.repositoryId}/verifications`, {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({ force: true }),
    });
  const waitForTerminalScan = async (operationID: string) => {
    let run: ScanRun = {};
    await expect(async () => {
      run = await api<ScanRun>(
        `/api/v1/storage/repositories/${workspace.repositoryId}/verifications/${operationID}`,
        { token: workspace.token },
      );
      expect(ACTIVE_VERIFICATION_STATUSES.has(run.status ?? "")).toBe(false);
    }).toPass({ timeout: 90_000 });
    return run;
  };

  let queued = await requestScan();
  expect(queued.operation_id).toBeTruthy();
  let run = await waitForTerminalScan(queued.operation_id!);
  // A fixture copy can overlap the repository's lifecycle scan. Its receipt
  // is valid but its snapshot predates the copied file, so request one fresh
  // verifier only after that coalesced operation reaches a terminal state.
  if (queued.coalesced) {
    queued = await requestScan();
    expect(queued.operation_id).toBeTruthy();
    run = await waitForTerminalScan(queued.operation_id!);
  }
  // The fixture file is settled and readable, so the verifier that runs after
  // the copy must cover it fully. A partial run (for example a file skipped as
  // still settling) would never surface the asset; fail on it here.
  expect(run.status).toBe("completed");
  expect(run.files_observed ?? 0).toBeGreaterThan(0);
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
