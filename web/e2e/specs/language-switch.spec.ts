import type { Page } from "playwright/test";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { t, tz } from "../support/i18n";
import { TERMS } from "../support/terminology";

// A translation key rendered as text means the bundle misses the key or the
// code bypassed i18n; neither is acceptable on a core page in either language.
const RAW_KEY =
  /\b(?:storagePanel|sidebar|settings|common|manage|assets|auth|upload|apiErrors)\.[A-Za-z][A-Za-z0-9_.]*/;

function documentLanguage(page: Page): Promise<string> {
  return page.evaluate(() => document.documentElement.lang);
}

async function expectNoRawKeys(page: Page) {
  const text = await page.locator("body").innerText();
  expect(text).not.toMatch(RAW_KEY);
}

/** The visible sidebar entry; the shell renders one per layout. */
function sidebarLink(page: Page, name: string) {
  return page.getByRole("link", { name, exact: true }).filter({ visible: true }).first();
}

async function chooseLanguage(page: Page, control: string, option: "English" | "中文") {
  // The language control is a dropdown button named after the Language row;
  // its options are endonyms defined in code, not translated copy.
  await page.getByRole("button", { name: control, exact: true }).click();
  await page.getByRole("button", { name: option, exact: true }).click();
}

test("@smoke the interface switches to 中文 with the registry terms and back to English", async ({
  page,
  workspace,
}) => {
  test.setTimeout(120_000);
  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/settings?tab=appearance");
  expect(await documentLanguage(page)).toBe("en");

  await chooseLanguage(page, t("settings.language"), "中文");
  await expect.poll(() => documentLanguage(page)).toBe("zh");
  // The preference is persisted, so a reload keeps the language.
  await page.reload();
  await expect.poll(() => documentLanguage(page)).toBe("zh");
  await expect(sidebarLink(page, tz("sidebar.settings"))).toBeVisible();

  // Storage: the heading, the Add Repository action, and the Storage Location
  // summary read with the registry's terms, and nothing shows a raw key.
  await page.goto("/storage");
  await expect(
    page.getByRole("heading", { name: tz("storagePanel.title"), exact: true }),
  ).toBeVisible();
  const addRepository = tz("storagePanel.action.addRepository");
  expect(addRepository).toContain(TERMS.repository.zh);
  await expect(page.getByRole("button", { name: addRepository, exact: true })).toBeVisible();
  await expect(page.getByText(TERMS.storageLocation.zh).first()).toBeVisible();
  await expect(sidebarLink(page, tz("sidebar.storage"))).toBeVisible();
  await expectNoRawKeys(page);

  // Photos.
  await page.goto("/assets");
  await expect(
    page.getByLabel(tz("assets.assetsPageHeader.scope.title")).filter({ visible: true }).first(),
  ).toBeVisible();
  await expect(sidebarLink(page, tz("sidebar.assets"))).toBeVisible();
  await expectNoRawKeys(page);

  // Back to English from the translated settings page.
  await page.goto("/settings?tab=appearance");
  await chooseLanguage(page, tz("settings.language"), "English");
  await expect.poll(() => documentLanguage(page)).toBe("en");
  await page.goto("/storage");
  await expect(
    page.getByRole("heading", { name: t("storagePanel.title"), exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: t("storagePanel.action.addRepository"), exact: true }),
  ).toBeVisible();
  await expectNoRawKeys(page);
});
