import { credentials, expect, seed, smokeAsset, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { t } from "../support/i18n";

test("@smoke user uploads a real image and sees it in the library", async ({ page }) => {
  await new LoginPage(page).signIn(credentials.username, credentials.password);
  await page.goto("/manage");
  await page.locator('input[type="file"]').setInputFiles(smokeAsset(seed.uploadAsset));
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
  await expect(async () => {
    await page.goto("/assets");
    await expect(page.getByLabel(/upload-001\.jpg/i)).toBeVisible({ timeout: 5_000 });
  }).toPass({ timeout: 60_000 });
});
