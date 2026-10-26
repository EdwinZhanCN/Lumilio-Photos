import { randomUUID } from "node:crypto";
import type { Locator, Page } from "playwright/test";
import type { components } from "../../src/lib/http-commons/schema.d.ts";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { confirmedAction, selectAsset } from "../support/assetActions";
import { SMOKE_SCAN_ASSET, smokeAsset } from "../support/assets";
import {
  container,
  containerChecksum,
  containerFiles,
  reinstallKeepingStorage,
  restartServer,
} from "../support/container";
import { t } from "../support/i18n";
import { saveBootstrapTOTP, totpCode } from "../support/totp";
import { bootstrapSession, placeScanFixture } from "../support/workspace";

type Schema = components["schemas"];
type StorageView = Schema["dto.StorageViewResponseDTO"];
type StorageDiagnostics = Schema["dto.StorageDiagnosticsResponseDTO"];
type StorageTargets = Schema["dto.StorageTargetsResponseDTO"];
type ScanAccepted = Schema["dto.RepositoryScanQueuedDTO"];
type ScanRun = Schema["dto.RepositoryScanRunDTO"];
type AssetList = Schema["dto.QueryAssetsResponseDTO"];
type RecoveryInventory = Schema["dto.RepositoryRecoveryDTO"];
type SetupStatus = Schema["dto.SetupStatusDTO"];
type TrashHolds = { count?: number };
type RepositoryResponse = { id?: string; name?: string; path?: string };

// Scans, Trash adoption and private recovery run on the Server's workers; on a
// low-power E2E host they take well over the default expect timeout.
const SERVER_PROCESSING_TIMEOUT = 120_000;

const bootstrap = {
  username: process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin",
  password: process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!",
};

/** The `id:` line of a `.lumiliorepo` marker read through the container. */
function markerIdentity(marker: string): string {
  const match = /^id:\s*"?([0-9a-fA-F-]{36})"?\s*$/m.exec(marker);
  if (!match) throw new Error(`marker without an id line:\n${marker}`);
  return match[1].toLowerCase();
}

async function repositoryPath(token: string, repositoryId: string): Promise<string> {
  const diagnostics = await api<StorageDiagnostics>("/api/v1/storage/diagnostics", { token });
  const path = diagnostics.items?.find(
    (item) => item.target_type === "repository" && item.target_id === repositoryId,
  )?.path;
  if (!path) throw new Error(`diagnostics omit the path of repository ${repositoryId}`);
  return path;
}

async function viewRow(token: string, repositoryId: string) {
  const view = await api<StorageView>("/api/v1/storage/view", { token });
  return view.repositories?.find((repository) => repository.id === repositoryId);
}

async function waitUntilIdle(token: string, repositoryId: string) {
  await expect(async () => {
    const row = await viewRow(token, repositoryId);
    expect(row).toBeTruthy();
    expect(row?.activity ?? "idle").toBe("idle");
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
}

async function scanUntilCompleted(token: string, repositoryId: string) {
  const receipt = await api<ScanAccepted>(
    `/api/v1/storage/repositories/${repositoryId}/verifications`,
    { method: "POST", token },
  );
  expect(receipt.operation_id).toBeTruthy();
  await expect(async () => {
    const run = await api<ScanRun>(
      `/api/v1/storage/repositories/${repositoryId}/verifications/${receipt.operation_id}`,
      { token },
    );
    expect(run.status).toBe("completed");
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
}

function listAssets(
  token: string,
  repositoryId: string,
  filename: string,
  state: "active" | "trashed",
) {
  return api<AssetList>("/api/v1/assets/list", {
    method: "POST",
    token,
    body: JSON.stringify({
      query: filename,
      search_type: "filename",
      filter: { repository_id: repositoryId, lifecycle_state: state },
      pagination: { limit: 10, offset: 0 },
      stack_mode: "expanded",
    }),
  });
}

/** The asset id a scanned file reached, once ingestion lists it. */
async function waitForAsset(
  token: string,
  repositoryId: string,
  filename: string,
  state: "active" | "trashed" = "active",
): Promise<string> {
  let assetId: string | undefined;
  await expect(async () => {
    const assets = await listAssets(token, repositoryId, filename, state);
    expect(assets.items).toHaveLength(1);
    assetId = assets.items?.[0]?.media_item?.primary_asset?.asset_id;
    expect(assetId).toBeTruthy();
  }).toPass({ timeout: SERVER_PROCESSING_TIMEOUT });
  return assetId as string;
}

async function heldCount(token: string, repositoryId: string): Promise<number> {
  const holds = await api<TrashHolds>(
    `/api/v1/assets/trash-holds?repository_id=${encodeURIComponent(repositoryId)}`,
    { token },
  );
  return holds.count ?? 0;
}

function recoveryInventory(token: string, repositoryId: string) {
  return api<RecoveryInventory>(`/api/v1/storage/repositories/${repositoryId}/recovery`, {
    token,
  });
}

/**
 * Opens an anchored menu and picks one entry. The menu closes on any scroll;
 * Playwright's own scroll-into-view can land after the menu opened, so scroll
 * first and reopen until the entry stays visible.
 */
async function pickMenuItem(
  page: Page,
  triggerName: string,
  itemName: string,
  scope: Locator | Page = page,
) {
  const trigger = scope.getByRole("button", { name: triggerName, exact: true });
  const item = page.getByRole("menuitem", { name: itemName, exact: true });
  await trigger.scrollIntoViewIfNeeded();
  await expect(async () => {
    if ((await trigger.getAttribute("aria-expanded")) !== "true") await trigger.click();
    await expect(item).toBeVisible({ timeout: 2_000 });
  }).toPass({ timeout: 15_000 });
  await item.click();
}

/**
 * Opens one Repository row's action menu. The row is found by its id, never by
 * its name alone: a separate copy of a Repository carries the same display name.
 */
function rowMenu(page: Page, repositoryId: string, repositoryName: string, actionKey: string) {
  const row = page
    .getByTestId("repository-row")
    .and(page.locator(`[data-repository-id=${JSON.stringify(repositoryId)}]`));
  return pickMenuItem(
    page,
    t("storagePanel.actionsFor", { name: repositoryName }),
    t(actionKey),
    row,
  );
}

async function openCandidates(page: Page) {
  await page.goto("/storage");
  await expect(
    page.getByRole("heading", { name: t("storagePanel.title"), exact: true }),
  ).toBeVisible();
  await pickMenuItem(page, t("storagePanel.moreActions"), t("manage.repositories.openExisting"));
  await expect(
    page.getByRole("heading", { name: t("manage.repositories.candidates.title"), exact: true }),
  ).toBeVisible();
}

function candidateCard(page: Page, directoryName: string): Locator {
  return page
    .getByTestId("repository-candidate")
    .and(page.locator(`[data-directory=${JSON.stringify(directoryName)}]`));
}

/**
 * Runs one candidate decision through the shared review: start the action,
 * acknowledge the risks the review lists (if any) and confirm. Returns the
 * command's response body.
 */
async function decideCandidate(
  page: Page,
  directoryName: string,
  action: "open" | "update_location" | "add_separate",
): Promise<RepositoryResponse> {
  const card = candidateCard(page, directoryName);
  await expect(card).toBeVisible();
  const startLabel =
    action === "open"
      ? t("common.open")
      : action === "update_location"
        ? t("manage.repositories.hostAction.updateLocation")
        : t("manage.repositories.hostAction.addSeparate");
  const confirmLabel =
    action === "add_separate" ? t("manage.repositories.candidates.addSeparateConfirm") : startLabel;
  await card.getByRole("button", { name: startLabel, exact: true }).click();
  const review = card.getByRole("region", {
    name: t("manage.repositories.candidates.reviewTitle"),
    exact: true,
  });
  await expect(review).toBeVisible();
  const confirm = review.getByRole("button", { name: confirmLabel, exact: true });
  const acknowledgement = review.getByText(t("manage.repositories.createRiskConfirmationTitle"), {
    exact: true,
  });
  // The review names this folder's risks; a folder without risks needs no
  // acknowledgement and the command is enabled as soon as the review arrives.
  await expect
    .poll(async () => (await acknowledgement.isVisible()) || (await confirm.isEnabled()), {
      timeout: 30_000,
    })
    .toBe(true);
  if (await acknowledgement.isVisible()) await acknowledgement.click();
  await expect(confirm).toBeEnabled();
  const endpoint =
    action === "open" ? "/api/v1/storage/candidates/open" : "/api/v1/storage/candidates/resolve";
  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" && new URL(response.url()).pathname === endpoint,
    { timeout: SERVER_PROCESSING_TIMEOUT },
  );
  await confirm.click();
  const response = await responsePromise;
  expect(response.ok(), await response.text()).toBe(true);
  return (await response.json()) as RepositoryResponse;
}

test("@storage-recovery a found folder is opened through the shared review, reconnected after a move, copied as a separate Repository, renamed, and its inherited Trash is held across a restart and restored", async ({
  page,
  workspace,
}) => {
  test.setTimeout(540_000);
  await page.setViewportSize({ width: 1440, height: 1000 });
  const { token, repositoryId, repositoryName, scanFilename } = workspace;
  const path = await repositoryPath(token, repositoryId);
  const directoryName = path.slice(path.lastIndexOf("/") + 1);
  const storageRoot = path.slice(0, path.lastIndexOf("/"));

  // A Repository with one scanned original and one trashed file: the private
  // state a later registration must bring along.
  await scanUntilCompleted(token, repositoryId);
  const assetId = await waitForAsset(token, repositoryId, scanFilename);
  const original = `${path}/${scanFilename}`;
  const checksum = containerChecksum(original);
  await api("/api/v1/assets/trash", {
    method: "POST",
    token,
    body: JSON.stringify({ asset_ids: [assetId] }),
  });
  container("test", "!", "-e", original);
  expect(containerFiles(`${path}/.lumilio/trash/files`)).toHaveLength(1);
  const identity = markerIdentity(container("cat", `${path}/.lumiliorepo`));
  expect(identity).toBe(repositoryId);
  await waitUntilIdle(token, repositoryId);

  // Remove from Lumilio keeps the folder, its marker and its Trash on disk.
  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/storage");
  await expect(
    page.getByRole("heading", { name: t("storagePanel.title"), exact: true }),
  ).toBeVisible();
  await rowMenu(page, repositoryId, repositoryName, "storagePanel.action.remove");
  await page
    .getByLabel(t("manage.repositories.removeConfirmationLabel", { name: repositoryName }), {
      exact: true,
    })
    .fill(repositoryName);
  const detached = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/v1/storage/repositories/${repositoryId}/detach`,
    { timeout: SERVER_PROCESSING_TIMEOUT },
  );
  await page
    .getByRole("button", { name: t("manage.repositories.removeAction"), exact: true })
    .click();
  const detachResponse = await detached;
  expect(detachResponse.ok(), await detachResponse.text()).toBe(true);
  expect((await detachResponse.json()) as { files_preserved?: boolean }).toMatchObject({
    files_preserved: true,
  });
  await expect.poll(() => viewRow(token, repositoryId), { timeout: 30_000 }).toBeUndefined();
  container("test", "-f", `${path}/.lumiliorepo`);
  expect(containerFiles(`${path}/.lumilio/trash/files`)).toHaveLength(1);

  // Open Existing Repository: the folder is a candidate again and is opened
  // through the Server's review of exactly that folder.
  await openCandidates(page);
  const opened = await decideCandidate(page, directoryName, "open");
  expect(opened.id).toBe(identity);

  // The inherited Trash is adopted and held; the receipt records the open.
  await expect
    .poll(async () => (await recoveryInventory(token, identity)).trash?.held_count, {
      timeout: SERVER_PROCESSING_TIMEOUT,
    })
    .toBe(1);
  const inventory = await recoveryInventory(token, identity);
  expect((inventory.receipts ?? []).map((receipt) => receipt.event)).toEqual(["open"]);
  expect(await heldCount(token, identity)).toBe(1);
  await waitForAsset(token, identity, scanFilename, "trashed");
  await waitUntilIdle(token, identity);

  // A Server restart changes nothing: the hold and the receipt are durable.
  await restartServer();
  expect(await heldCount(token, identity)).toBe(1);
  expect(
    ((await recoveryInventory(token, identity)).receipts ?? []).map((receipt) => receipt.event),
  ).toEqual(["open"]);
  await expect.poll(async () => (await viewRow(token, identity))?.reachability).toBe("active");

  // The folder moves while the Server runs. The registered path is positively
  // gone, so the moved tree can be used as the moved original: identity, Trash
  // and Assets follow, nothing is copied.
  const movedDirectory = `${directoryName}-moved`;
  const movedPath = `${storageRoot}/${movedDirectory}`;
  container("mv", path, movedPath);
  await openCandidates(page);
  const reconnected = await decideCandidate(page, movedDirectory, "update_location");
  expect(reconnected.id).toBe(identity);
  await expect.poll(() => repositoryPath(token, identity), { timeout: 30_000 }).toBe(movedPath);
  expect(markerIdentity(container("cat", `${movedPath}/.lumiliorepo`))).toBe(identity);
  expect(await heldCount(token, identity)).toBe(1);
  await waitUntilIdle(token, identity);

  // A byte-identical copy while the original is online can only become a
  // separate Repository with its own identity; the original is untouched.
  const copyDirectory = `${directoryName}-copy`;
  const copyPath = `${storageRoot}/${copyDirectory}`;
  container("cp", "-a", movedPath, copyPath);
  await openCandidates(page);
  const separateCard = candidateCard(page, copyDirectory);
  await expect(separateCard).toBeVisible();
  await expect(
    separateCard.getByRole("button", {
      name: t("manage.repositories.hostAction.updateLocation"),
      exact: true,
    }),
  ).toHaveCount(0);
  const separate = await decideCandidate(page, copyDirectory, "add_separate");
  expect(separate.id).toBeTruthy();
  expect(separate.id).not.toBe(identity);
  expect(markerIdentity(container("cat", `${copyPath}/.lumiliorepo`))).toBe(separate.id);
  expect(markerIdentity(container("cat", `${movedPath}/.lumiliorepo`))).toBe(identity);
  await expect.poll(() => repositoryPath(token, identity)).toBe(movedPath);
  await expect
    .poll(async () => (await viewRow(token, separate.id as string))?.reachability, {
      timeout: 30_000,
    })
    .toBe("active");

  // Rename changes only the display name, in the catalog and in the marker.
  const renamed = `${repositoryName} renamed`;
  await page.goto("/storage");
  await rowMenu(page, identity, repositoryName, "storagePanel.action.rename");
  await page.getByLabel(t("manage.repositories.renameLabel"), { exact: true }).fill(renamed);
  const renameResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/v1/storage/repositories/${identity}/rename`,
  );
  await page
    .getByRole("button", { name: t("manage.repositories.renameSubmit"), exact: true })
    .click();
  expect((await renameResponse).ok()).toBe(true);
  await expect.poll(async () => (await viewRow(token, identity))?.name).toBe(renamed);
  const markerAfterRename = container("cat", `${movedPath}/.lumiliorepo`);
  expect(markerIdentity(markerAfterRename)).toBe(identity);
  expect(markerAfterRename).toContain(renamed);

  // The Recovery inventory names what the folder brought along. A fresh load
  // reads the renamed row from the Server rather than from the mutation cache.
  await page.goto("/storage");
  await rowMenu(page, identity, renamed, "storagePanel.action.recovery");
  const recoveryDialog = page.getByRole("dialog", {
    name: t("storagePanel.recovery.title"),
    exact: true,
  });
  await expect(recoveryDialog).toBeVisible();
  await expect(
    recoveryDialog.getByText(t("storagePanel.recovery.heldTrashValue_one", { count: 1 }), {
      exact: true,
    }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(recoveryDialog).toBeHidden();

  // The Trash view shows the hold and restores the file to its original place.
  await page.goto(`/storage/${identity}/trash`);
  await expect(
    page.getByText(t("assets.trash.holds.title_one", { count: 1 }), { exact: true }),
  ).toBeVisible();
  await selectAsset(page, scanFilename);
  await confirmedAction(
    page,
    "assets.trash.bulkActions.restore.label_one",
    "common.confirm",
    "/api/v1/assets/restore",
  );
  const restored = `${movedPath}/${scanFilename}`;
  expect(containerChecksum(restored)).toBe(checksum);
  expect(containerFiles(`${movedPath}/.lumilio/trash/files`)).toHaveLength(0);
  await expect.poll(() => heldCount(token, identity)).toBe(0);
  expect(await waitForAsset(token, identity, scanFilename, "active")).toBe(assetId);
});

/**
 * A reinstall over kept media. The app-state volume (catalog, queue, secrets)
 * is emptied and the Server starts again against the untouched storage volume,
 * so the browser owns the first-admin ceremony and the setup review finds the
 * Primary Repository that is already there. The ceremony cannot be replayed on
 * the same instance, so this scenario does not retry.
 */
test.describe("reinstall", () => {
  test.describe.configure({ retries: 0 });

  async function fillOTP(page: Page, code: string) {
    const inputs = page.locator('input[inputmode="numeric"]');
    await expect(inputs).toHaveCount(6);
    for (let index = 0; index < code.length; index += 1) {
      await inputs.nth(index).fill(code[index] ?? "");
    }
  }

  test("@storage-recovery a reinstall adopts the retained Primary Repository; its Trash is held and its originals come back", async ({
    page,
  }) => {
    test.setTimeout(600_000);
    await page.setViewportSize({ width: 1440, height: 1000 });
    const { token } = await bootstrapSession();
    const { targets } = await api<StorageTargets>("/api/v1/storage/targets", { token });
    const primaryId = targets?.find((target) => target.role === "primary")?.id;
    if (!primaryId) throw new Error("the seeded instance has no Primary Repository");
    const primaryPath = await repositoryPath(token, primaryId);
    const identity = markerIdentity(container("cat", `${primaryPath}/.lumiliorepo`));
    expect(identity).toBe(primaryId);

    // One original stays, one goes to the Trash before the catalog is lost.
    const suffix = randomUUID().slice(0, 8);
    const keptName = `e2e-reinstall-kept-${suffix}.jpg`;
    const trashedName = `e2e-reinstall-trashed-${suffix}.jpg`;
    const primary = { id: primaryId, name: "primary", path: primaryPath };
    placeScanFixture(primary, smokeAsset(SMOKE_SCAN_ASSET), keptName);
    placeScanFixture(primary, smokeAsset(SMOKE_SCAN_ASSET), trashedName);
    await waitUntilIdle(token, primaryId);
    await scanUntilCompleted(token, primaryId);
    await waitForAsset(token, primaryId, keptName);
    const trashedAssetId = await waitForAsset(token, primaryId, trashedName);
    const keptChecksum = containerChecksum(`${primaryPath}/${keptName}`);
    const trashedChecksum = containerChecksum(`${primaryPath}/${trashedName}`);
    await api("/api/v1/assets/trash", {
      method: "POST",
      token,
      body: JSON.stringify({ asset_ids: [trashedAssetId] }),
    });
    container("test", "!", "-e", `${primaryPath}/${trashedName}`);
    const markerBefore = container("cat", `${primaryPath}/.lumiliorepo`);
    await waitUntilIdle(token, primaryId);

    await reinstallKeepingStorage();
    const status = await api<SetupStatus>("/api/v1/setup/status");
    expect(status.admin_initialized).toBe(false);
    expect(status.primary_repository_initialized).toBe(false);
    container("test", "-f", `${primaryPath}/${keptName}`);

    // First-admin ceremony on the fresh instance.
    await page.goto("/bootstrap");
    await page.getByRole("button", { name: t("auth.bootstrap.welcome.cta") }).click();
    await page.getByLabel(t("auth.register.username"), { exact: true }).fill(bootstrap.username);
    await page.getByLabel(t("auth.register.password"), { exact: true }).fill(bootstrap.password);
    await page
      .getByLabel(t("auth.register.confirmPassword"), { exact: true })
      .fill(bootstrap.password);
    const setupResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/api/v1/auth/mfa/totp/setup",
    );
    await page.getByRole("button", { name: t("auth.bootstrap.admin.submit") }).click();
    const setupResponse = await setupResponsePromise;
    expect(setupResponse.ok(), await setupResponse.text()).toBe(true);
    const { secret } = (await setupResponse.json()) as { secret?: string };
    expect(secret).toBeTruthy();
    // Later seeds and workspaces sign the bootstrap admin in from this hand-off.
    saveBootstrapTOTP({ username: bootstrap.username, secret: secret as string });
    await fillOTP(page, totpCode(secret as string));
    await page.getByRole("button", { name: t("auth.register.verifyAndEnable") }).click();
    await page.getByRole("button", { name: t("auth.bootstrap.passkey.skip") }).click();
    await page.getByLabel(t("auth.bootstrap.recovery.savedConfirm"), { exact: true }).check();
    await page.getByRole("button", { name: t("auth.bootstrap.recovery.cta") }).click();

    // The storage step reviews primary/ and offers Adopt, not Create.
    await expect(page.getByText(t("auth.primarySetup.adopt.title"), { exact: true })).toBeVisible();
    await expect(
      page.getByText(t("auth.primarySetup.adopt.heldTrash"), { exact: true }),
    ).toBeVisible();
    const adoptResponsePromise = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname === "/api/v1/setup/primary-repository/adopt",
      { timeout: SERVER_PROCESSING_TIMEOUT },
    );
    await page
      .getByRole("button", { name: t("auth.primarySetup.adopt.submit"), exact: true })
      .click();
    const adoptResponse = await adoptResponsePromise;
    expect(adoptResponse.ok(), await adoptResponse.text()).toBe(true);
    const adopted = (await adoptResponse.json()) as { repository?: { id?: string; role?: string } };
    expect(adopted.repository).toMatchObject({ id: identity, role: "primary" });
    await expect(page).toHaveURL(/\/$/);

    // The new catalog: same identity, untouched marker and originals, the
    // trashed file held, the kept original ingested again by the adopt scan.
    const { token: adoptedToken } = await bootstrapSession();
    const { targets: adoptedTargets } = await api<StorageTargets>("/api/v1/storage/targets", {
      token: adoptedToken,
    });
    expect(adoptedTargets?.find((target) => target.role === "primary")?.id).toBe(identity);
    expect(container("cat", `${primaryPath}/.lumiliorepo`)).toBe(markerBefore);
    expect(containerChecksum(`${primaryPath}/${keptName}`)).toBe(keptChecksum);
    await expect
      .poll(() => heldCount(adoptedToken, identity), { timeout: SERVER_PROCESSING_TIMEOUT })
      .toBeGreaterThanOrEqual(1);
    await waitForAsset(adoptedToken, identity, trashedName, "trashed");
    await waitForAsset(adoptedToken, identity, keptName);

    // The Trash view shows the hold; restoring puts the file back as it was.
    const held = await heldCount(adoptedToken, identity);
    await page.goto(`/storage/${identity}/trash`);
    await expect(
      page.getByText(
        t(held === 1 ? "assets.trash.holds.title_one" : "assets.trash.holds.title_other", {
          count: held,
        }),
        { exact: true },
      ),
    ).toBeVisible();
    await selectAsset(page, trashedName);
    await confirmedAction(
      page,
      "assets.trash.bulkActions.restore.label_one",
      "common.confirm",
      "/api/v1/assets/restore",
    );
    expect(containerChecksum(`${primaryPath}/${trashedName}`)).toBe(trashedChecksum);
    await expect.poll(() => heldCount(adoptedToken, identity)).toBe(held - 1);
    await waitForAsset(adoptedToken, identity, trashedName);
  });
});
