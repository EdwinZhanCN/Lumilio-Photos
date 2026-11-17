import type { Page } from "playwright/test";
import { expect } from "playwright/test";
import { t } from "./i18n.ts";

/** Enters selection mode on an assets page and selects one asset by filename. */
export async function selectAsset(page: Page, filename: string) {
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

/**
 * Runs one confirmed bulk action from the open actions menu and returns the
 * successful response of the endpoint it must call.
 */
export async function confirmedAction(
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
