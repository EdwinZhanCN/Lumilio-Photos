import type { Page } from "playwright/test";
import { expect, test } from "playwright/test";
import { GalleryPage } from "../pages/gallery.page";
import { api } from "../support/api";
import { t } from "../support/i18n";
import { sampleJPEG } from "../support/sample-image";
import { totpCode } from "../support/totp";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

/**
 * The published-artifact smoke (`release-smoke.yml`): one fresh install of a
 * Server image, driven only through the browser and the public API, the way a
 * first user meets it. It is not part of the `@smoke` CI slice: the instance
 * is a release bundle installation, not the E2E image, and it has no seed.
 *
 * Journey: setup wizard → upload a photo → browse and open it → an album with
 * a cover and a share link (user state for the compatibility fixture) →
 * backup → restore → the state is still there.
 */

type StorageTargets = components["schemas"]["dto.StorageTargetsResponseDTO"];
type BrowseResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];
type BackupList = components["schemas"]["dto.BackupListDTO"];
type RestoreOperation = components["schemas"]["dto.RestoreOperationDTO"];
type Album = { album_id: number; cover_asset_id?: string | null };

const username = process.env.LUMILIO_SMOKE_USERNAME ?? "smoke-admin";
const password = process.env.LUMILIO_SMOKE_PASSWORD ?? "Lumilio-Smoke-2026!";
const filename = "release-smoke.jpg";

async function fillOTP(page: Page, code: string) {
  const inputs = page.locator('input[inputmode="numeric"]');
  await expect(inputs).toHaveCount(6);
  for (let index = 0; index < code.length; index += 1) {
    await inputs.nth(index).fill(code[index] ?? "");
  }
}

/** The first-run wizard: admin account, TOTP, recovery codes, Primary Repository. */
async function completeSetup(page: Page): Promise<void> {
  await page.goto("/bootstrap");
  await page.getByRole("button", { name: t("auth.bootstrap.welcome.cta") }).click();
  await page.getByLabel(t("auth.register.username"), { exact: true }).fill(username);
  await page.getByLabel(t("auth.register.password"), { exact: true }).fill(password);
  await page.getByLabel(t("auth.register.confirmPassword"), { exact: true }).fill(password);
  const setupResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/auth/mfa/totp/setup",
  );
  await page.getByRole("button", { name: t("auth.bootstrap.admin.submit") }).click();
  const setupResponse = await setupResponsePromise;
  expect(setupResponse.ok(), await setupResponse.text()).toBe(true);
  const setup = (await setupResponse.json()) as { secret?: string };
  expect(setup.secret).toBeTruthy();
  await fillOTP(page, totpCode(setup.secret as string));
  await page.getByRole("button", { name: t("auth.register.verifyAndEnable") }).click();
  await page.getByRole("button", { name: t("auth.bootstrap.passkey.skip") }).click();
  await page.getByLabel(t("auth.bootstrap.recovery.savedConfirm"), { exact: true }).check();
  await page.getByRole("button", { name: t("auth.bootstrap.recovery.cta") }).click();
  await page.getByRole("button", { name: t("auth.bootstrap.repository.submit") }).click();
  await expect(page).toHaveURL(/\/$/);
}

async function sessionToken(page: Page): Promise<string> {
  const token = await page.evaluate(() => window.localStorage.getItem("auth_token"));
  if (!token) throw new Error("the setup wizard did not leave a signed-in session");
  return token;
}

async function findAsset(token: string, name: string) {
  const response = await api<BrowseResponse>("/api/v1/assets/list", {
    method: "POST",
    token,
    body: JSON.stringify({
      query: name,
      search_type: "filename",
      pagination: { limit: 20, offset: 0 },
      stack_mode: "expanded",
    }),
  });
  return response.items
    ?.map((item) => item.media_item?.primary_asset)
    .find((asset) => asset?.original_filename === name);
}

async function listBackups(token: string) {
  return (await api<BackupList>("/api/v1/settings/backups", { token })).backups ?? [];
}

async function restoreOperation(token: string, id: string): Promise<RestoreOperation | null> {
  try {
    return await api<RestoreOperation>(
      `/api/v1/settings/backup-restores/${encodeURIComponent(id)}`,
      { token },
    );
  } catch {
    // The runtime restarts in place to apply the restore; a transport failure
    // is an intermediate state, not a verdict.
    return null;
  }
}

test.describe.configure({ retries: 0 });

test("@release-smoke a fresh install completes setup, uploads, browses, backs up, and restores", async ({
  page,
}) => {
  test.setTimeout(600_000);

  await completeSetup(page);
  const token = await sessionToken(page);
  const { targets } = await api<StorageTargets>("/api/v1/storage/targets", { token });
  const primary = targets?.find((target) => target.role === "primary") ?? targets?.[0];
  if (!primary?.id || !primary.name) throw new Error("setup did not create a Primary Repository");

  // Upload through the page, as a user would.
  await page.goto("/upload");
  await page
    .getByLabel(t("upload.UnifiedUploadSection.upload_target_label"))
    .selectOption({ label: primary.name });
  await page.locator('input[type="file"]').setInputFiles({
    name: filename,
    mimeType: "image/jpeg",
    buffer: sampleJPEG(),
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

  // Browse until ingestion lands it, then open the detail view.
  const gallery = new GalleryPage(page);
  await expect(async () => {
    await gallery.scopeTo(primary.name as string);
    await expect(page.getByLabel(new RegExp(filename, "i"))).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: 120_000 });
  const asset = await findAsset(token, filename);
  if (!asset?.asset_id) throw new Error("the upload did not appear through the browse API");
  await page.getByLabel(new RegExp(filename, "i")).click();
  await expect(page).toHaveURL(new RegExp(`/${asset.asset_id}$`));
  await expect(
    page
      .getByRole("button", { name: t("assets.assetsPageHeader.moreActions"), exact: true })
      .filter({ visible: true }),
  ).toBeVisible();

  // User state the compatibility fixture carries.
  const album = await api<Album>("/api/v1/albums", {
    method: "POST",
    token,
    body: JSON.stringify({ album_name: "Release smoke" }),
  });
  await api(`/api/v1/albums/${album.album_id}/assets/${asset.asset_id}`, {
    method: "POST",
    token,
    body: "{}",
  });
  await api(`/api/v1/albums/${album.album_id}`, {
    method: "PUT",
    token,
    body: JSON.stringify({ cover_asset_id: asset.asset_id }),
  });
  await api("/api/v1/share-links", {
    method: "POST",
    token,
    body: JSON.stringify({
      title: "Release smoke share",
      source_kind: "asset_snapshot",
      asset_ids: [asset.asset_id],
      expires_in_days: 30,
    }),
  });

  // Backup from Settings, then restore it through the same page.
  await page.goto("/settings?tab=server");
  await expect(
    page.getByText(t("settings.serverSettings.backup.title"), { exact: true }),
  ).toBeVisible();
  const before = new Set((await listBackups(token)).map((entry) => entry.name));
  await page
    .getByRole("button", { name: t("settings.serverSettings.backup.createNow"), exact: true })
    .click();
  let backupName: string | undefined;
  await expect
    .poll(
      async () => {
        backupName = (await listBackups(token)).find(
          (entry) => !entry.restore_point && !before.has(entry.name),
        )?.name;
        return backupName;
      },
      { timeout: 120_000, intervals: [1_000, 2_000, 5_000] },
    )
    .toBeTruthy();
  await page
    .getByRole("button", { name: t("settings.serverSettings.backup.refresh"), exact: true })
    .click();
  const row = page.getByRole("listitem").filter({ hasText: backupName as string });
  await expect(row).toBeVisible();
  const restoreAccepted = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname.endsWith("/restore"),
    { timeout: 120_000 },
  );
  await row
    .getByRole("button", { name: t("settings.serverSettings.backup.restore"), exact: true })
    .click();
  await row
    .getByRole("button", { name: t("settings.serverSettings.backup.confirmYes"), exact: true })
    .click();
  const restoreResponse = await restoreAccepted;
  expect(restoreResponse.status()).toBe(202);
  const operation = (await restoreResponse.json()) as RestoreOperation;
  const operationID = operation.id;
  if (!operationID) throw new Error("the restore was accepted without an operation id");
  await expect
    .poll(async () => (await restoreOperation(token, operationID))?.status, {
      timeout: 180_000,
      intervals: [1_000, 2_000, 5_000],
    })
    .toBe("completed");

  // Everything the user made is still there after the round trip.
  await expect(async () => {
    await gallery.scopeTo(primary.name as string);
    await expect(page.getByLabel(new RegExp(filename, "i"))).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: 120_000 });
  const restoredAlbum = await api<Album>(`/api/v1/albums/${album.album_id}`, { token });
  expect(restoredAlbum.cover_asset_id).toBe(asset.asset_id);
  expect((await listBackups(token)).some((entry) => entry.restore_point)).toBe(true);
});
