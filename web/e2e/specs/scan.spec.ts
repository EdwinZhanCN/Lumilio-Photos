import { credentials, expect, seed, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { t } from "../support/i18n";

test("@smoke administrator scans a real repository file and sees it", async ({ page }) => {
  await new LoginPage(page).signIn(credentials.username, credentials.password);
  await page.goto("/manage");
  // The UI labels the primary repository with a translated name, not its stored one.
  await page
    .getByRole("button", {
      name: t("manage.repositories.actionsMenu", { name: t("navbar.repository.primary") }),
    })
    .click();
  const completedScan = page.waitForResponse(async (response) => {
    if (!response.url().includes("/scans/latest") || response.request().method() !== "GET") {
      return false;
    }
    return response.ok() && (await response.json()).status === "completed";
  });
  await page.getByRole("button", { name: t("manage.repositories.rescanRepository") }).click();
  await completedScan;
  await page.goto("/assets");
  await expect(page.getByLabel(new RegExp(seed.scanFilename, "i"))).toBeVisible({
    timeout: 60_000,
  });
});
