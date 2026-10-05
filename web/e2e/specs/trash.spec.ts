import { execFileSync } from "node:child_process";
import type { Page } from "playwright/test";
import type { components } from "../../src/lib/http-commons/schema.d.ts";
import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import { compose, docker, repositoryRoot } from "../support/docker";
import { t } from "../support/i18n";

type Schema = components["schemas"];
type AssetList = Schema["dto.QueryAssetsResponseDTO"];
type ScanRun = Schema["dto.RepositoryScanRunDTO"];

function container(...command: string[]): string {
  // execFileSync throws on Docker/container failures instead of treating a
  // broken daemon as evidence that a file was deleted.
  return execFileSync("docker", [...docker, ...compose, "exec", "-T", "lumilio", ...command], {
    cwd: repositoryRoot,
    encoding: "utf8",
  }).trim();
}

async function selectAsset(page: Page, filename: string) {
  await page
    .getByRole("button", { name: t("assets.assetsPageHeader.selectionMode.label"), exact: true })
    .filter({ visible: true })
    .click();
  await page.getByRole("button", { name: new RegExp(filename, "i") }).click();
  await page
    .getByRole("button", { name: new RegExp(t("assets.assetsPageHeader.actions.title")) })
    .filter({ visible: true })
    .click();
}

async function confirmedAction(
  page: Page,
  actionKey: string,
  confirmKey: string,
  endpoint: string,
) {
  await page
    .getByRole("button", { name: t(actionKey, { count: 1 }), exact: true })
    .filter({ visible: true })
    .click();
  const response = page.waitForResponse(
    (candidate) =>
      candidate.request().method() === "POST" && new URL(candidate.url()).pathname === endpoint,
  );
  // The header menu and legacy confirmation modal can expose the same
  // translated action name. Scope confirmation to the open modal.
  await page
    .locator(".modal-open")
    .getByRole("button", { name: t(confirmKey), exact: true })
    .filter({ visible: true })
    .click();
  const completed = await response;
  expect(completed.ok()).toBe(true);
  return completed;
}

test("@smoke Trash restores album membership and permanently deletes the file", async ({
  page,
  workspace,
}) => {
  test.setTimeout(300_000);
  await page.setViewportSize({ width: 1440, height: 1000 });
  const token = workspace.token;
  const diagnostics = await api<Schema["dto.StorageDiagnosticsResponseDTO"]>(
    "/api/v1/storage/diagnostics",
    { token },
  );
  const repositoryPath = diagnostics.items?.find(
    (item) => item.target_type === "repository" && item.target_id === workspace.repositoryId,
  )?.path;
  expect(repositoryPath).toBeTruthy();
  const original = `${repositoryPath}/${workspace.scanFilename}`;
  const trashDirectory = `${repositoryPath}/.lumilio/trash/files`;
  const checksum = container("sha256sum", original).split(" ")[0];
  const trashFiles = () =>
    container("find", trashDirectory, "-type", "f").split("\n").filter(Boolean);

  const receipt = await api<Schema["dto.RepositoryScanQueuedDTO"]>(
    `/api/v1/storage/repositories/${workspace.repositoryId}/verifications`,
    { method: "POST", token },
  );
  expect(receipt.operation_id).toBeTruthy();
  await expect(async () => {
    const run = await api<ScanRun>(
      `/api/v1/storage/repositories/${workspace.repositoryId}/verifications/${receipt.operation_id}`,
      { token },
    );
    expect(run.status).toBe("completed");
  }).toPass({ timeout: 90_000 });
  const list = (state: "active" | "trashed") =>
    api<AssetList>("/api/v1/assets/list", {
      method: "POST",
      token,
      body: JSON.stringify({
        query: workspace.scanFilename,
        search_type: "filename",
        filter: { repository_id: workspace.repositoryId, lifecycle_state: state },
        pagination: { limit: 10, offset: 0 },
        stack_mode: "expanded",
      }),
    });
  let assetId: string | undefined;
  await expect(async () => {
    const assets = await list("active");
    expect(assets.items).toHaveLength(1);
    assetId = assets.items?.[0].media_item?.primary_asset?.asset_id;
    expect(assetId).toBeTruthy();
  }).toPass({ timeout: 90_000 });
  const album = await api<Schema["dto.GetAlbumResponseDTO"]>("/api/v1/albums", {
    method: "POST",
    token,
    body: JSON.stringify({ album_name: `Trash ${workspace.username}` }),
  });
  expect(album.album_id).toBeTruthy();
  await api(`/api/v1/albums/${album.album_id}/assets/${assetId}`, {
    method: "POST",
    token,
    body: "{}",
  });
  const albumMembers = () =>
    api<Schema["dto.AlbumAssetsResponseDTO"]>(`/api/v1/albums/${album.album_id}/assets`, { token });
  expect((await albumMembers()).assets?.map((asset) => asset.asset_id)).toEqual([assetId]);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  const deleteFromGallery = async () => {
    await new GalleryPage(page).scopeTo(workspace.repositoryName);
    await selectAsset(page, workspace.scanFilename);
    await confirmedAction(
      page,
      "assets.assetsPageHeader.actions.deleteSelected",
      "assets.assetsPageHeader.deleteConfirmModal.deleteButton",
      "/api/v1/assets/trash",
    );
    container("test", "!", "-e", original);
    expect((await list("active")).items ?? []).toHaveLength(0);
    expect((await list("trashed")).items?.[0].media_item?.primary_asset?.asset_id).toBe(assetId);
    const files = trashFiles();
    expect(files).toHaveLength(1);
    expect(container("sha256sum", files[0]).split(" ")[0]).toBe(checksum);
    return files[0];
  };
  const firstTrashPath = await deleteFromGallery();
  await page.goto(`/storage/${workspace.repositoryId}/trash`);
  await selectAsset(page, workspace.scanFilename);
  await confirmedAction(
    page,
    "assets.trash.bulkActions.restore.label_one",
    "common.confirm",
    "/api/v1/assets/restore",
  );
  expect(container("sha256sum", original).split(" ")[0]).toBe(checksum);
  container("test", "!", "-e", firstTrashPath);
  expect((await list("active")).items?.[0].media_item?.primary_asset?.asset_id).toBe(assetId);
  expect((await albumMembers()).assets?.map((asset) => asset.asset_id)).toEqual([assetId]);
  await page.goto(`/collections/${album.album_id}`);
  await expect(
    page.getByRole("button", { name: new RegExp(workspace.scanFilename, "i") }),
  ).toBeVisible();

  const finalTrashPath = await deleteFromGallery();
  await page.goto(`/storage/${workspace.repositoryId}/trash`);
  await selectAsset(page, workspace.scanFilename);
  await confirmedAction(
    page,
    "assets.lifecycle.deletePermanently",
    "common.confirm",
    "/api/v1/assets/delete-permanently",
  );
  container("test", "!", "-e", original);
  container("test", "!", "-e", finalTrashPath);
  expect(trashFiles()).toHaveLength(0);
  expect((await list("trashed")).items ?? []).toHaveLength(0);
  expect((await albumMembers()).assets ?? []).toHaveLength(0);
  const missing = await fetch(`${baseURL}/api/v1/assets/${assetId}`, {
    headers: { authorization: `Bearer ${token}` },
  });
  expect(missing.status).toBe(404);
  await expect(
    page.getByRole("button", { name: new RegExp(workspace.scanFilename, "i") }),
  ).toHaveCount(0);
});
