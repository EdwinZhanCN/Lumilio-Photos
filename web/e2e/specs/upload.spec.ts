import { readFileSync } from "node:fs";
import { expect, test } from "../fixtures/test";
import { GalleryPage } from "../pages/gallery.page";
import { LoginPage } from "../pages/login.page";
import { t } from "../support/i18n";

test("@smoke user uploads a real image and sees it in the library", async ({ page, workspace }) => {
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
});
