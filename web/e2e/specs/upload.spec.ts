import { readFileSync } from "node:fs";
import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { t } from "../support/i18n";

type Asset = {
  asset_id: string;
  original_filename: string;
};

type BrowseResponse = {
  items?: Array<{ asset?: Asset }>;
};

type Album = {
  album_id: number;
  album_name: string;
};

async function findAsset(
  token: string,
  repositoryID: string,
  filename: string,
): Promise<Asset | undefined> {
  const response = await api<BrowseResponse>("/api/v1/assets/list", {
    method: "POST",
    token,
    body: JSON.stringify({
      query: filename,
      search_type: "filename",
      filter: { repository_id: repositoryID },
      pagination: { limit: 20, offset: 0 },
      stack_mode: "expanded",
    }),
  });
  return response.items
    ?.map((item) => item.asset)
    .find((asset): asset is Asset => asset?.original_filename === filename);
}

test("@smoke user completes the compact upload, album, viewer, Trash, and restore journey", async ({
  page,
  workspace,
}) => {
  test.setTimeout(120_000);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/manage");
  await page
    .getByLabel(t("upload.UnifiedUploadSection.upload_target_label"))
    .selectOption({ label: workspace.repositoryName });
  // Uploaded under the worker's own name so the assertion below cannot be
  // satisfied by another worker's upload of the same source image.
  await page.locator('input[type="file"]').setInputFiles({
    name: workspace.uploadFilename,
    mimeType: "image/jpeg",
    buffer: readFileSync(workspace.uploadSource),
  });

  // Navigating away mid-upload aborts it, so wait on the real accept response
  // rather than on a toast.
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

  // Ingestion continues after the accept response, and the gallery query does not
  // refetch on its own, so reload until the asset lands.
  const gallery = new GalleryPage(page);
  await expect(async () => {
    await gallery.scopeTo(workspace.repositoryName);
    await expect(page.getByLabel(new RegExp(workspace.uploadFilename, "i"))).toBeVisible({
      timeout: 5_000,
    });
  }).toPass({ timeout: 60_000 });

  const asset = await findAsset(
    workspace.token,
    workspace.repositoryId,
    workspace.uploadFilename,
  );
  if (!asset) throw new Error("uploaded asset did not appear through the public browse API");

  // Reuse the same real upload for the rest of the compact product journey;
  // this adds coverage without another ingest/transcode workload.
  const albumName = `E2E Core ${workspace.username}`;
  const album = await api<Album>("/api/v1/albums", {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ album_name: albumName }),
  });
  await api(`/api/v1/albums/${album.album_id}/assets/${asset.asset_id}`, {
    method: "POST",
    token: workspace.token,
    body: "{}",
  });

  const albumPath = `/collections/${album.album_id}`;
  await page.goto(albumPath);
  const albumAsset = page.getByLabel(new RegExp(workspace.uploadFilename, "i"));
  await expect(albumAsset).toBeVisible();
  await albumAsset.click();
  await expect(page).toHaveURL(new RegExp(`${albumPath}/${asset.asset_id}$`));
  await expect(
    page
      .getByRole("button", {
        name: t("assets.assetsPageHeader.moreActions"),
        exact: true,
      })
      .filter({ visible: true }),
  ).toBeVisible();

  await page
    .getByRole("button", {
      name: t("assets.assetsPageHeader.moreActions"),
      exact: true,
    })
    .filter({ visible: true })
    .click();
  await page
    .getByRole("button", {
      name: t("common.delete"),
      exact: true,
    })
    .filter({ visible: true })
    .click();
  const deleteResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "DELETE" &&
      new URL(response.url()).pathname === `/api/v1/assets/${asset.asset_id}`,
  );
  await page
    .getByRole("button", {
      name: t("delete.confirm"),
      exact: true,
    })
    .click();
  expect((await deleteResponse).ok()).toBe(true);

  await page.goto("/collections/trash");
  const trashedAsset = page.getByLabel(new RegExp(workspace.uploadFilename, "i"));
  await expect(trashedAsset).toBeVisible();
  await page
    .getByRole("button", {
      name: t("assets.assetsPageHeader.selectionMode.label"),
      exact: true,
    })
    .filter({ visible: true })
    .click();
  await trashedAsset.click();
  await page
    .getByRole("button", {
      name: new RegExp(`^${t("assets.assetsPageHeader.actions.title")}`),
    })
    .filter({ visible: true })
    .click();
  await page
    .getByRole("button", {
      name: t("assets.trash.bulkActions.restore.label_one", { count: 1 }),
      exact: true,
    })
    .click();

  const restoreResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === `/api/v1/assets/${asset.asset_id}/restore`,
  );
  await page
    .getByRole("button", {
      name: t("common.confirm"),
      exact: true,
    })
    .click();
  expect((await restoreResponse).ok()).toBe(true);

  await page.goto(albumPath);
  await expect(page.getByLabel(new RegExp(workspace.uploadFilename, "i"))).toBeVisible();
});
