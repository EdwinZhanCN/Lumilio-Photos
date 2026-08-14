import type { Page } from "playwright/test";
import { expect, test } from "../fixtures/test";
import { t } from "../support/i18n";
import { saveBootstrapTOTP, totpCode } from "../support/totp";

type TOTPSetupResponse = {
  secret?: string;
  setup_token?: string;
};

type LoginResponse = {
  token?: string;
  csrfToken?: string;
  requires_mfa?: boolean;
  mfa_token?: string;
  mfa_methods?: string[];
};

const username = process.env.LUMILIO_E2E_USERNAME ?? "e2e-admin";
const password = process.env.LUMILIO_E2E_PASSWORD ?? "Lumilio-E2E-2026!";

async function fillOTP(page: Page, code: string) {
  const inputs = page.locator('input[inputmode="numeric"]');
  await expect(inputs).toHaveCount(6);
  for (let index = 0; index < code.length; index += 1) {
    await inputs.nth(index).fill(code[index] ?? "");
  }
}

async function freshCode(page: Page, secret: string, previous: string): Promise<string> {
  let code = totpCode(secret);
  if (code === previous) {
    const secondsIntoWindow = Math.floor(Date.now() / 1_000) % 30;
    await page.waitForTimeout((30 - secondsIntoWindow) * 1_000 + 150);
    code = totpCode(secret);
  }
  return code;
}

test.describe.configure({ retries: 0 });

test("@auth-totp first-admin bootstrap enables TOTP and gates the next password login", async ({
  page,
}) => {
  test.setTimeout(90_000);

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
  const setup = (await setupResponse.json()) as TOTPSetupResponse;
  expect(setup.secret).toBeTruthy();
  expect(setup.setup_token).toBeTruthy();
  const secret = setup.secret as string;
  saveBootstrapTOTP({ username, secret });

  const enrollmentCode = totpCode(secret);
  await fillOTP(page, enrollmentCode);
  await page.getByRole("button", { name: t("auth.register.verifyAndEnable") }).click();

  await expect(page.getByRole("button", { name: t("auth.bootstrap.passkey.skip") })).toBeVisible();
  await page.getByRole("button", { name: t("auth.bootstrap.passkey.skip") }).click();

  const savedLabel = t("auth.bootstrap.recovery.savedConfirm");
  await page.getByLabel(savedLabel, { exact: true }).check();
  await page.getByRole("button", { name: t("auth.bootstrap.recovery.cta") }).click();
  await page.getByRole("button", { name: t("auth.bootstrap.repository.submit") }).click();
  await expect(page).toHaveURL(/\/$/);

  await page.getByRole("button", { name: username }).click();
  await page.getByRole("button", { name: t("auth.logout"), exact: true }).click();
  await expect(page).toHaveURL(/\/login$/);

  await page.getByLabel(t("auth.login.username"), { exact: true }).fill(username);
  await page.getByRole("button", { name: t("auth.login.continue") }).click();
  await page.getByLabel(t("auth.login.password"), { exact: true }).fill(password);

  const loginResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname === "/api/v1/auth/login",
  );
  await page.getByRole("button", { name: t("auth.login.signIn") }).click();
  const loginResponse = await loginResponsePromise;
  expect(loginResponse.ok(), await loginResponse.text()).toBe(true);
  const login = (await loginResponse.json()) as LoginResponse;
  expect(login.requires_mfa).toBe(true);
  expect(login.mfa_token).toBeTruthy();
  expect(login.token).toBeUndefined();
  expect(login.csrfToken).toBeUndefined();
  await expect(page.getByRole("heading", { name: t("auth.login.verifyTitle") })).toBeVisible();
  expect(await page.evaluate(() => window.localStorage.getItem("auth_token"))).toBeNull();

  const loginCode = await freshCode(page, secret, enrollmentCode);
  await fillOTP(page, loginCode);
  // The login OtpInput submits automatically when the sixth digit is entered.
  // Do not click the button a second time while the authenticated route is
  // replacing the verification view.
  await expect(page).toHaveURL(/\/$/);
  expect(await page.evaluate(() => window.localStorage.getItem("auth_token"))).toBeTruthy();
});
