import { spawnSync } from "node:child_process";
import type { Locator, Page } from "playwright/test";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { SMOKE_SCAN_ASSET, smokeAsset } from "../support/assets";
import { compose, docker, repositoryRoot } from "../support/docker";
import { t } from "../support/i18n";
import { placeScanFixture } from "../support/workspace";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type StorageView = components["schemas"]["dto.StorageViewResponseDTO"];
type StorageRepositoryView = components["schemas"]["dto.StorageRepositoryViewDTO"];
type CreateRepositoryResponse = components["schemas"]["dto.CreateRepositoryResponseDTO"];
type ScanAccepted = components["schemas"]["dto.RepositoryScanQueuedDTO"];
type ScanRun = components["schemas"]["dto.RepositoryScanRunDTO"];
type AssetList = components["schemas"]["dto.QueryAssetsResponseDTO"];

const ACTIVE_VERIFICATION_STATUSES = new Set(["queued", "crawling", "catching_up", "finalizing"]);

// Scan and ingestion run on the server's River workers. On a low-power E2E
// host (Intel N100) a lifecycle scan plus a manual scan of one file can take
// well over the default expect timeout, so server-processing outcomes poll
// the API with this explicit bound instead.
const SERVER_PROCESSING_TIMEOUT = 90_000;

async function storageRepository(token: string, id: string): Promise<StorageRepositoryView> {
  const view = await api<StorageView>("/api/v1/storage/view", { token });
  const repository = view.repositories?.find((candidate) => candidate.id === id);
  if (!repository) throw new Error(`storage view does not list repository ${id}`);
  return repository;
}

function containerDirectoryExists(path: string): boolean {
  // Storage is a named volume inside the container, so ask the container.
  const result = spawnSync(
    "docker",
    [...docker, ...compose, "exec", "-T", "lumilio", "test", "-d", path],
    {
      cwd: repositoryRoot,
      stdio: "ignore",
    },
  );
  if (result.error) throw result.error;
  return result.status === 0;
}

function repositoryRow(page: Page, name: string): Locator {
  return page.getByRole("row").filter({ has: page.getByRole("button", { name, exact: true }) });
}

/** The row cell under the column whose header is `headerKey`. */
async function cellUnder(page: Page, row: Locator, headerKey: string): Promise<Locator> {
  const table = page.getByRole("table").filter({ has: row });
  const headers = await table.getByRole("columnheader").allTextContents();
  const index = headers.findIndex((header) => header.trim() === t(headerKey));
  expect(index, `column ${headerKey}`).toBeGreaterThanOrEqual(0);
  return row.locator(":scope > th, :scope > td").nth(index);
}

test("@smoke administrator adds a Repository and scans it from Storage", async ({
  page,
  workspace,
}) => {
  test.setTimeout(300_000);
  // The attempt username is unique per test/repeat/retry, so the Repository
  // name and its folder cannot collide across reruns or parallel agents.
  const repositoryName = `Storage Admin ${workspace.username}`;
  const scanFilename = `e2e-storage-admin-${workspace.username}.jpg`;

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/storage");
  await expect(
    page.getByRole("heading", { name: t("storagePanel.title"), exact: true }),
  ).toBeVisible();

  // Add Repository wizard: Details → Storage Location → layout → Review.
  await page
    .getByRole("button", { name: t("storagePanel.action.addRepository"), exact: true })
    .click();
  await page
    .getByLabel(t("manage.repositories.createNameLabel"), { exact: true })
    .fill(repositoryName);
  const next = page.getByRole("button", { name: t("common.next"), exact: true });
  await next.click();
  // The default Storage Location is preselected once the locations load.
  await expect(
    page.getByLabel(t("manage.repositories.storageLocationLabel"), { exact: true }),
  ).not.toHaveValue("");
  await next.click();
  await next.click();

  const created = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/storage/repositories",
  );
  await page
    .getByRole("button", { name: t("manage.repositories.createSubmit"), exact: true })
    .click();
  const createResponse = await created;
  expect(createResponse.ok()).toBe(true);
  const { repository } = (await createResponse.json()) as CreateRepositoryResponse;
  expect(repository?.id).toBeTruthy();
  expect(repository?.path).toBeTruthy();
  const repositoryId = repository!.id!;
  const repositoryPath = repository!.path!;
  expect(repository).toMatchObject({ name: repositoryName, role: "regular", is_primary: false });

  // The Server created the Repository folder inside its Storage Location.
  expect(containerDirectoryExists(repositoryPath)).toBe(true);

  // Listed as a regular, reachable Repository with no assets yet.
  const listed = await storageRepository(workspace.token, repositoryId);
  expect(listed).toMatchObject({
    name: repositoryName,
    role: "regular",
    reachability: "active",
  });
  // A counted empty Repository reports 0; an absent count means the Server
  // could not count its assets.
  expect(listed.asset_count).toBe(0);
  expect(listed.storage_location_id).toBe(repository!.storage_location_id);

  const row = repositoryRow(page, repositoryName);
  await expect(row).toBeVisible();
  await expect(await cellUnder(page, row, "storagePanel.column.assets")).toHaveText("0");

  // Creation may start a lifecycle scan. Let it finish before the fixture
  // lands so the manual scan below is the one that must observe the file.
  await expect(async () => {
    const current = await storageRepository(workspace.token, repositoryId);
    expect(current.activity ?? "idle").toBe("idle");
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
  await expect(await cellUnder(page, row, "storagePanel.column.state")).toHaveText(
    t("storagePanel.state.available"),
  );

  placeScanFixture(
    { id: repositoryId, name: repositoryName, path: repositoryPath },
    smokeAsset(SMOKE_SCAN_ASSET),
    scanFilename,
  );

  const scanFromRowMenu = async () => {
    const queued = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname ===
          `/api/v1/storage/repositories/${repositoryId}/verifications`,
    );
    const trigger = page.getByRole("button", {
      name: t("storagePanel.actionsFor", { name: repositoryName }),
      exact: true,
    });
    const scanNow = page.getByRole("menuitem", {
      name: t("storagePanel.action.scan"),
      exact: true,
    });
    // The anchored menu closes on any scroll. Playwright scrolls the trigger
    // into view as part of the click, and that scroll event can land after
    // the menu opened, so scroll first and reopen until the menu stays open.
    await trigger.scrollIntoViewIfNeeded();
    await expect(async () => {
      if ((await trigger.getAttribute("aria-expanded")) !== "true") await trigger.click();
      await expect(scanNow).toBeVisible({ timeout: 2_000 });
    }).toPass({ timeout: 15_000 });
    await scanNow.click();
    const response = await queued;
    expect(response.ok()).toBe(true);
    const receipt = (await response.json()) as ScanAccepted;
    expect(receipt.repository_id).toBe(repositoryId);
    expect(receipt.operation_id).toBeTruthy();
    return receipt;
  };
  const waitForTerminalScan = async (operationId: string) => {
    let run: ScanRun | undefined;
    await expect(async () => {
      run = await api<ScanRun>(
        `/api/v1/storage/repositories/${repositoryId}/verifications/${operationId}`,
        { token: workspace.token },
      );
      expect(ACTIVE_VERIFICATION_STATUSES.has(run.status ?? "")).toBe(false);
    }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
    return run!;
  };

  let receipt = await scanFromRowMenu();
  let run = await waitForTerminalScan(receipt.operation_id!);
  // A coalesced receipt joined a run whose snapshot may predate the copied
  // file; request one fresh scan once that run is terminal.
  if (receipt.coalesced) {
    receipt = await scanFromRowMenu();
    run = await waitForTerminalScan(receipt.operation_id!);
  }

  expect(run).toMatchObject({
    operation_id: receipt.operation_id,
    repository_id: repositoryId,
    mode: "manual",
    requested_by: workspace.username,
    status: "completed",
  });
  expect(run.files_observed ?? 0).toBeGreaterThanOrEqual(1);
  expect(run.error_directories ?? 0).toBe(0);

  // The scan ingested exactly the copied file into this Repository.
  await expect(async () => {
    const assets = await api<AssetList>("/api/v1/assets/list", {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({
        query: scanFilename,
        search_type: "filename",
        filter: { repository_id: repositoryId },
        pagination: { limit: 10, offset: 0 },
        stack_mode: "expanded",
      }),
    });
    expect(assets.items ?? []).toHaveLength(1);
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });

  // The Storage read model reports this scan as the latest verification.
  await expect(async () => {
    const current = await storageRepository(workspace.token, repositoryId);
    expect(current.verification).toMatchObject({
      operation_id: receipt.operation_id,
      status: "completed",
    });
    expect(current.activity ?? "idle").toBe("idle");
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
  // ...and counts the ingested asset. Ingestion is already proven above, so
  // this is a read-model fact, not a processing wait.
  await expect
    .poll(async () => (await storageRepository(workspace.token, repositoryId)).asset_count, {
      timeout: 30_000,
    })
    .toBe(1);

  // The page reads the same facts after a fresh load.
  await page.reload();
  await expect(row).toBeVisible();
  await expect(await cellUnder(page, row, "storagePanel.column.verification")).toHaveText(
    t("storagePanel.verificationBadge.verified"),
  );
  await expect(await cellUnder(page, row, "storagePanel.column.state")).toHaveText(
    t("storagePanel.state.available"),
  );
  await expect(await cellUnder(page, row, "storagePanel.column.assets")).toHaveText("1");
});
