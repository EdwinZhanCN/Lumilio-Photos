import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import { uniqueJpeg } from "../support/assets";
import { t } from "../support/i18n";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type BrowseResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];
type CreatedShare = components["schemas"]["dto.CreateShareLinkResponseDTO"];
type RevokedShare = components["schemas"]["dto.ShareLinkDTO"];
type PublicShare = components["schemas"]["dto.PublicShareMetadataDTO"];
type PublicAssets = components["schemas"]["dto.PublicShareAssetListResponseDTO"];

// Share creation needs a finished ingest and a thumbnail before the recipient
// sees media. The shared E2E host can be a low-power box, so this waits on
// server processing with a deliberately wider bound than UI transitions.
const INGEST_TIMEOUT = 90_000;

async function findAssetID(token: string, repositoryID: string, filename: string) {
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
    ?.map((item) => item.media_item?.primary_asset)
    .find((asset) => asset?.original_filename === filename)?.asset_id;
}

/** Status of an unauthenticated request, exactly as a link recipient would send it. */
async function publicStatus(pathname: string) {
  const response = await fetch(`${baseURL}${pathname}`);
  await response.body?.cancel();
  return response.status;
}

test("@smoke owner shares a photo, a logged-out recipient opens it, and revoke cuts access", async ({
  page,
  browser,
  workspace,
}) => {
  test.setTimeout(240_000);
  const filename = `e2e-share-${workspace.username}.jpg`;
  const title = `E2E Share ${workspace.username}`;

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/upload");
  await page
    .getByLabel(t("upload.UnifiedUploadSection.upload_target_label"))
    .selectOption({ label: workspace.repositoryName });
  await page.locator('input[type="file"]').setInputFiles({
    name: filename,
    mimeType: "image/jpeg",
    buffer: uniqueJpeg(workspace.uploadSource, `lumilio-share:${filename}`),
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

  let assetID: string | undefined;
  await expect(async () => {
    assetID = await findAssetID(workspace.token, workspace.repositoryId, filename);
    expect(assetID).toBeTruthy();
  }).toPass({ timeout: INGEST_TIMEOUT });
  if (!assetID) throw new Error("uploaded asset did not appear through the browse API");

  // Create through the real gallery: selection mode, bulk Share, share form.
  const gallery = new GalleryPage(page);
  const tile = page.getByLabel(new RegExp(filename, "i"));
  await expect(async () => {
    await gallery.scopeTo(workspace.repositoryName);
    await expect(tile).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: INGEST_TIMEOUT });
  await page
    .getByRole("button", { name: t("assets.assetsPageHeader.selectionMode.label"), exact: true })
    .filter({ visible: true })
    .click();
  await tile.click();
  await page
    .getByRole("button", { name: new RegExp(`^${t("assets.assetsPageHeader.actions.title")}`) })
    .filter({ visible: true })
    .click();
  await page
    .getByRole("button", {
      name: t("assets.assetsPageHeader.bulkActions.share.label"),
      exact: true,
    })
    .filter({ visible: true })
    .click();

  const shareDialog = page.getByRole("dialog", { name: t("share.create.title") });
  await expect(shareDialog).toBeVisible();
  await shareDialog.getByPlaceholder(t("share.create.fields.title.placeholder")).fill(title);
  const createdResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/v1/share-links" &&
      response.request().method() === "POST",
  );
  await shareDialog.getByRole("button", { name: t("share.create.submit"), exact: true }).click();
  const createResponse = await createdResponse;
  expect(createResponse.status()).toBe(200);
  const created = (await createResponse.json()) as CreatedShare;
  if (!created.token || !created.share_id) throw new Error("share creation returned no token");
  expect(created.asset_count).toBe(1);

  // The one-time link the owner copies is the recipient's only capability.
  const createdDialog = page.getByRole("dialog", { name: t("share.create.createdTitle") });
  const shareURL = await createdDialog.getByRole("textbox").inputValue();
  expect(new URL(shareURL).pathname).toBe(`/s/${created.token}`);
  await createdDialog.getByRole("button", { name: t("common.done"), exact: true }).click();

  // The recipient has a fresh browser: no cookies, no stored session.
  const recipient = await browser.newContext({ locale: "en-US" });
  try {
    expect(await recipient.cookies()).toEqual([]);
    const publicPage = await recipient.newPage();
    const authorizedRequests: string[] = [];
    publicPage.on("request", (request) => {
      if (request.headers()["authorization"]) authorizedRequests.push(request.url());
    });

    const metadataPath = `/api/v1/public/shares/${created.token}`;
    const thumbnailPath = `${metadataPath}/assets/${assetID}/thumbnail?size=medium`;
    const metadata = publicPage.waitForResponse(
      (response) => new URL(response.url()).pathname === metadataPath,
    );
    const assets = publicPage.waitForResponse(
      (response) => new URL(response.url()).pathname === `${metadataPath}/assets/list`,
    );
    await publicPage.goto(shareURL);
    const metadataResponse = await metadata;
    expect(metadataResponse.status()).toBe(200);
    expect(((await metadataResponse.json()) as PublicShare).title).toBe(title);
    const assetsResponse = await assets;
    expect(assetsResponse.status()).toBe(200);
    const listed = (await assetsResponse.json()) as PublicAssets;
    expect(listed.items?.map((item) => item.asset_id)).toEqual([assetID]);
    await expect(publicPage).toHaveURL(new RegExp(`/s/${created.token}$`));
    await expect(publicPage.getByRole("heading", { level: 1, name: title })).toBeVisible();
    // Thumbnail generation continues after ingest, so poll the recipient-side
    // endpoint rather than assume the media is ready.
    await expect.poll(() => publicStatus(thumbnailPath), { timeout: INGEST_TIMEOUT }).toBe(200);
    expect(authorizedRequests).toEqual([]);

    // Revoke through the owner's Shared Links page.
    await page.goto("/collections/shared-links");
    const row = page.getByRole("row").filter({ hasText: title });
    const revoked = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `/api/v1/share-links/${created.share_id}/revoke` &&
        response.request().method() === "POST",
    );
    await row.getByRole("button", { name: t("share.manage.actions.revoke"), exact: true }).click();
    const revokeResponse = await revoked;
    expect(revokeResponse.status()).toBe(200);
    expect(((await revokeResponse.json()) as RevokedShare).status).toBe("revoked");

    // Revoked, expired, and unknown tokens are deliberately indistinguishable
    // (share_link_handler.go resolvePublicShare): every public endpoint is 404.
    const refused = publicPage.waitForResponse(
      (response) => new URL(response.url()).pathname === metadataPath,
    );
    await publicPage.reload();
    expect((await refused).status()).toBe(404);
    await expect(publicPage.getByText(t("share.public.unavailable.title"))).toBeVisible();
    await expect(publicPage.getByRole("heading", { level: 1, name: title })).toHaveCount(0);
    expect(await publicStatus(metadataPath)).toBe(404);
    expect(await publicStatus(thumbnailPath)).toBe(404);
    expect(authorizedRequests).toEqual([]);
  } finally {
    await recipient.close();
  }
});
